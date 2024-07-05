package quotacontrol_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	. "github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/0xsequence/quotacontrol/test"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const RateLimitHeader = "x-ratelimit-limit"

func TestMiddlewareUseAccessKey(t *testing.T) {
	auth := jwtauth.New("HS256", []byte("secret"), nil)

	cfg := newConfig()
	server, cleanup := test.NewServer(&cfg)
	t.Cleanup(cleanup)

	now := time.Now()
	project := uint64(7)
	key := proto.GenerateAccessKey(project)
	service := proto.Service_Indexer

	limit := proto.Limit{
		RateLimit: 100,
		FreeWarn:  5,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}

	ctx := context.Background()
	err := server.Store.SetAccessLimit(ctx, project, &limit)
	require.NoError(t, err)
	err = server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: key, ProjectID: project})
	require.NoError(t, err)

	client := newQuotaClient(cfg, service)

	counter := spendingCounter(0)

	r := chi.NewRouter()
	r.Use(
		middleware.Session(client, auth),
		addCredits(2).Middleware,
		addCredits(-1).Middleware,
		middleware.RateLimit(cfg.RateLimiter, cfg.Redis),
		middleware.SpendUsage(client),
	)

	r.Handle("/*", &counter)

	expectedUsage := proto.AccessUsage{}

	t.Run("WithAccessKey", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)
		server.FlushNotifications()

		// Spend Free CU
		for i := int64(1); i < limit.FreeWarn; i++ {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, strconv.FormatInt(limit.FreeMax-i, 10), headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, "", headers.Get(middleware.HeaderQuotaOverage))
			assert.Empty(t, server.GetEvents(project), i)
			expectedUsage.Add(proto.AccessUsage{ValidCompute: 1})
		}

		// Go over free CU
		ok, headers, err := executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaOverage))
		assert.Contains(t, server.GetEvents(project), proto.EventType_FreeMax)
		expectedUsage.Add(proto.AccessUsage{ValidCompute: 1})

		// Get close to soft quota
		for i := limit.FreeWarn + 1; i < limit.OverWarn; i++ {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, strconv.FormatInt(i-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
			assert.Len(t, server.GetEvents(project), 1)
			expectedUsage.Add(proto.AccessUsage{OverCompute: 1})
		}

		// Go over soft quota
		ok, headers, err = executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
		assert.Equal(t, strconv.FormatInt(limit.OverWarn-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
		assert.Contains(t, server.GetEvents(project), proto.EventType_OverWarn)
		expectedUsage.Add(proto.AccessUsage{OverCompute: 1})

		// Get close to hard quota
		for i := limit.OverWarn + 1; i < limit.OverMax; i++ {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, strconv.FormatInt(i-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
			assert.Len(t, server.GetEvents(project), 2)
			expectedUsage.Add(proto.AccessUsage{OverCompute: 1})
		}

		// Go over hard quota
		ok, headers, err = executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
		assert.Equal(t, strconv.FormatInt(limit.OverMax-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
		assert.Contains(t, server.GetEvents(project), proto.EventType_OverMax)
		expectedUsage.Add(proto.AccessUsage{OverCompute: 1})

		// Denied
		for i := 0; i < 10; i++ {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.ErrorIs(t, err, proto.ErrLimitExceeded)
			assert.False(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, strconv.FormatInt(limit.OverMax-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
			expectedUsage.Add(proto.AccessUsage{LimitedCompute: 1})
		}

		// check the usage
		client.Stop(context.Background())
		usage, err := server.Store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ChangeLimits", func(t *testing.T) {
		// Increase CreditsOverageLimit which should still allow requests to go through, etc.
		err = server.Store.SetAccessLimit(ctx, project, &proto.Limit{
			RateLimit: 100,
			OverWarn:  5,
			OverMax:   110,
		})
		assert.NoError(t, err)
		err = client.ClearQuotaCacheByAccessKey(ctx, key)
		assert.NoError(t, err)

		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)
		server.FlushNotifications()

		ok, headers, err := executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaLimit))

		client.Stop(context.Background())
		usage, err := server.Store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		expectedUsage.Add(proto.AccessUsage{ValidCompute: 0, OverCompute: 1, LimitedCompute: 0})
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("PublicRateLimit", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)

		for i, max := 0, cfg.RateLimiter.PublicRPM*2; i < max; i++ {
			ok, headers, err := executeRequest(ctx, r, "", "", "")
			if i < cfg.RateLimiter.PublicRPM {
				assert.NoError(t, err, i)
				assert.True(t, ok, i)
				assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
			} else {
				assert.ErrorIs(t, err, proto.ErrLimitExceeded, i)
				assert.False(t, ok, i)
				assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
			}
		}

		client.Stop(context.Background())
		usage, err := server.Store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ServerErrors", func(t *testing.T) {
		server.FlushCache(ctx)

		go client.Run(context.Background())

		errList := []error{
			errors.New("unexpected error"),
			proto.ErrWebrpcBadRoute,
			proto.ErrTimeout,
		}

		ctx := middleware.WithTime(context.Background(), now)

		for _, err := range errList {
			server.ErrGetAccessQuota = err
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.True(t, ok)
			assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
			assert.NoError(t, err)
		}
		server.ErrGetAccessQuota = nil

		server.FlushCache(ctx)

		for _, err := range errList {
			server.ErrPrepareUsage = err
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.True(t, ok)
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaLimit))
			assert.NoError(t, err)
		}
		server.ErrPrepareUsage = nil

		client.Stop(context.Background())
		usage, err := server.Store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ServerTimeout", func(t *testing.T) {
		server.FlushCache(ctx)

		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)

		server.PrepareUsageDelay = time.Second * 3
		ok, headers, err := executeRequest(ctx, r, "", key, "")
		assert.True(t, ok)
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaLimit))
		assert.NoError(t, err)

		client.Stop(context.Background())
		usage, err := server.Store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})
}

func TestDefaultKey(t *testing.T) {
	cfg := newConfig()
	server, cleanup := test.NewServer(&cfg)
	t.Cleanup(cleanup)

	now := time.Now()
	project := uint64(7)
	keys := []string{
		proto.GenerateAccessKey(project),
		proto.GenerateAccessKey(project),
	}

	service := proto.Ptr(proto.Service_Metadata)
	limit := proto.Limit{
		RateLimit: 100,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}

	access := &proto.AccessKey{
		Active:    true,
		AccessKey: keys[0],
		ProjectID: project,
	}

	// populate store
	ctx := context.Background()
	err := server.Store.SetAccessLimit(ctx, project, &limit)
	require.NoError(t, err)
	err = server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: keys[0], ProjectID: project})
	require.NoError(t, err)

	client := newQuotaClient(cfg, *service)

	aq, err := client.FetchKeyQuota(ctx, keys[0], "", now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	aq, err = client.FetchKeyQuota(ctx, keys[0], "", now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	access, err = server.UpdateAccessKey(ctx, keys[0], proto.Ptr("new name"), nil, []*proto.Service{service})
	require.NoError(t, err)

	aq, err = client.FetchKeyQuota(ctx, keys[0], "", now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	ok, err := server.DisableAccessKey(ctx, keys[0])
	require.ErrorIs(t, err, proto.ErrAtLeastOneKey)
	assert.False(t, ok)
	newAccess := proto.AccessKey{Active: true, AccessKey: keys[1], ProjectID: project}
	err = server.Store.InsertAccessKey(ctx, &newAccess)
	require.NoError(t, err)

	ok, err = server.DisableAccessKey(ctx, keys[0])
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = client.FetchKeyQuota(ctx, keys[0], "", now)
	require.ErrorIs(t, err, proto.ErrAccessKeyNotFound)

	newAccess.Default = true
	aq, err = client.FetchKeyQuota(ctx, newAccess.AccessKey, "", now)
	require.NoError(t, err)
	assert.Equal(t, &newAccess, aq.AccessKey)
}

func TestJWT(t *testing.T) {
	auth := jwtauth.New("HS256", []byte("secret"), nil)

	project := uint64(7)
	account := "account"
	key := proto.GenerateAccessKey(project)
	service := proto.Service_Indexer

	counter := spendingCounter(0)

	cfg := newConfig()
	server, cleanup := test.NewServer(&cfg)
	t.Cleanup(cleanup)
	client := newQuotaClient(cfg, service)

	r := chi.NewRouter()

	r.Use(
		middleware.Session(client, auth),
		middleware.EnsureUsage(client),
		middleware.SpendUsage(client),
	)
	r.Handle("/*", &counter)

	ctx := context.Background()

	limit := proto.Limit{
		RateLimit: 100,
		FreeWarn:  5,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}
	server.Store.SetAccessLimit(ctx, project, &limit)

	token := mustJWT(t, auth, middleware.Claims{"project": project, "account": account})

	var expectedHits int64

	t.Run("UnauthorizedUser", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.ErrorIs(t, err, proto.ErrUnauthorizedUser)
		assert.False(t, ok)
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
	})
	server.Store.SetUserPermission(ctx, project, account, proto.UserPermission_READ_WRITE, proto.ResourceAccess{ProjectID: project})
	t.Run("AuthorizedUser", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		expectedHits++
	})
	t.Run("AccessKeyNotFound", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", key, token)
		require.ErrorIs(t, err, proto.ErrAccessKeyNotFound)
		assert.False(t, ok)
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
	})
	server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: key, ProjectID: project})
	t.Run("AccessKeyFound", func(t *testing.T) {
		ok, _, err := executeRequest(ctx, r, "", key, token)
		require.NoError(t, err)
		assert.True(t, ok)
		expectedHits++
	})
	t.Run("AccessKeyMismatch", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", proto.GenerateAccessKey(project+1), token)
		require.ErrorIs(t, err, proto.ErrAccessKeyMismatch)
		assert.False(t, ok)
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
	})

	assert.Equal(t, expectedHits, counter.GetValue())
}

func TestJWTAccess(t *testing.T) {
	auth := jwtauth.New("HS256", []byte("secret"), nil)

	project := uint64(7)
	service := proto.Service_Indexer
	account := "account"

	counter := hitCounter(0)

	cfg := newConfig()
	server, cleanup := test.NewServer(&cfg)
	t.Cleanup(cleanup)
	client := newQuotaClient(cfg, service)

	r := chi.NewRouter()
	r.Use(
		middleware.Session(client, auth),
		middleware.RateLimit(cfg.RateLimiter, cfg.Redis),
		middleware.EnsurePermission(client, UserPermission_READ_WRITE),
	)
	r.Handle("/*", &counter)

	ctx := context.Background()
	limit := proto.Limit{
		RateLimit: 100,
		FreeWarn:  5,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}
	server.Store.SetAccessLimit(ctx, project, &limit)

	token := mustJWT(t, auth, middleware.Claims{"account": account, "project": project})

	var expectedHits int64

	t.Run("NoPermission", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.ErrorIs(t, err, proto.ErrUnauthorizedUser)
		assert.False(t, ok)
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
	})

	server.Store.SetUserPermission(ctx, project, account, proto.UserPermission_READ, proto.ResourceAccess{ProjectID: project})
	t.Run("LowPermission", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.ErrorIs(t, err, proto.ErrUnauthorizedUser)
		assert.False(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, strconv.FormatInt(limit.RateLimit, 10), headers.Get(RateLimitHeader))
	})

	server.Store.SetUserPermission(ctx, project, account, proto.UserPermission_READ_WRITE, proto.ResourceAccess{ProjectID: project})
	server.FlushCache(ctx)
	t.Run("EnoughPermission", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, strconv.FormatInt(limit.RateLimit, 10), headers.Get(RateLimitHeader))
		expectedHits++
	})

	server.Store.SetUserPermission(ctx, project, account, proto.UserPermission_ADMIN, proto.ResourceAccess{ProjectID: project})
	server.FlushCache(ctx)
	t.Run("MorePermission", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, strconv.FormatInt(limit.RateLimit, 10), headers.Get(RateLimitHeader))
		expectedHits++
	})

	assert.Equal(t, expectedHits, counter.GetValue())
}

func TestSession(t *testing.T) {
	auth := jwtauth.New("HS256", []byte("secret"), nil)

	project := uint64(7)
	key := proto.GenerateAccessKey(project)
	service := proto.Service_Indexer
	address := "accountId"
	//serviceName := "//serviceName"

	counter := hitCounter(0)

	cfg := newConfig()
	server, cleanup := test.NewServer(&cfg)
	t.Cleanup(cleanup)
	client := newQuotaClient(cfg, service)

	const (
		MethodPublic    = "MethodPublic"
		MethodAccount   = "MethodAccount"
		MethodAccessKey = "MethodAccessKey"
		MethodProject   = "MethodProject"
		MethodAdmin     = "MethodAdmin"
		MethodService   = "MethodService"
	)

	ACL := middleware.ACL{
		"Service": {
			MethodPublic:    proto.SessionType_Public,
			MethodAccount:   proto.SessionType_Account,
			MethodAccessKey: proto.SessionType_AccessKey,
			MethodProject:   proto.SessionType_Project,
			MethodAdmin:     proto.SessionType_Admin,
			MethodService:   proto.SessionType_Service,
		},
	}

	r := chi.NewRouter()
	r.Use(
		middleware.Session(client, auth),
		middleware.RateLimit(cfg.RateLimiter, cfg.Redis),
		middleware.AccessControl(ACL, middleware.Cost{}, 1),
	)
	r.Handle("/*", &counter)

	ctx := context.Background()
	limit := proto.Limit{
		RateLimit: 100,
		FreeWarn:  5,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}
	server.Store.SetAccessLimit(ctx, project, &limit)
	server.Store.SetUserPermission(ctx, project, address, proto.UserPermission_READ, proto.ResourceAccess{ProjectID: project})
	server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: key, ProjectID: project})

	testCases := []struct {
		AccessKey string
		Claims    middleware.Claims
		Session   proto.SessionType
	}{
		// {
		// 	Session: proto.SessionType_Public,
		// },
		// {
		// 	Claims:  middleware.Claims{"account": address},
		// 	Session: proto.SessionType_Account,
		// },
		{
			AccessKey: key,
			Claims:    middleware.Claims{"account": address},
			Session:   proto.SessionType_AccessKey,
		},
		{
			Claims:  middleware.Claims{"account": address, "project": project},
			Session: proto.SessionType_Project,
		},
		{
			AccessKey: key,
			Claims:    middleware.Claims{"account": address, "project": project},
			Session:   proto.SessionType_Project,
		},
		// {
		// 	Claims:  middleware.Claims{"account": address, "admin": true},
		// 	Session: proto.SessionType_Admin,
		// },
		// {
		// 	AccessKey: key,
		// 	Claims:    middleware.Claims{"account": address, "admin": true},
		// 	Session:   proto.SessionType_Admin,
		// },
		// {
		// 	Claims:  middleware.Claims{"service": //serviceName},
		// 	Session: proto.SessionType_Service,
		// },
		// {
		// 	AccessKey: key,
		// 	Claims:    middleware.Claims{"service": serviceName},
		// 	Session:   proto.SessionType_Service,
		// },
	}

	var (
		publicRPM  = fmt.Sprint(cfg.RateLimiter.PublicRPM)
		accountRPM = fmt.Sprint(cfg.RateLimiter.AccountRPM)
		serviceRPM = fmt.Sprint(cfg.RateLimiter.ServiceRPM)
		quotaRPM   = fmt.Sprint(limit.RateLimit)
	)

	methods := []string{
		// MethodPublic,
		// MethodAccount,
		MethodAccessKey,
		// MethodProject,
		// MethodAdmin,
		// MethodService,
	}

	for service := range ACL {
		for _, method := range methods {
			minSession := ACL[service][method]
			fmt.Printf("%s/%s - %s+\n", service, method, minSession)
			for _, tc := range testCases {
				args := []any{"%s/%s %+v", service, method, tc}
				path := "/rpc/" + service + "/" + method
				jwt := ""
				if tc.Claims != nil {
					jwt = mustJWT(t, auth, tc.Claims)
				}
				ok, h, err := executeRequest(ctx, r, path, tc.AccessKey, jwt)
				success := tc.Session >= minSession
				if ok != success {
					fmt.Printf("  - %s %v Key:%v\n", tc.Session, tc.Claims, tc.AccessKey != "")
				}
				if success {
					assert.NoError(t, err, "%s/%s %+v", service, method, tc)
					assert.True(t, ok)
					switch v := h.Get(RateLimitHeader); tc.Session {
					case proto.SessionType_Public:
						assert.Equal(t, publicRPM, v)
					case proto.SessionType_AccessKey, proto.SessionType_Project:
						assert.Equal(t, quotaRPM, v)
						assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), h.Get(middleware.HeaderQuotaLimit))
					case proto.SessionType_Account:
						assert.Equal(t, accountRPM, v)
					case proto.SessionType_Service:
						assert.Equal(t, serviceRPM, v, args...)
					}
				} else {
					assert.Error(t, err, args...)
					assert.False(t, ok)
				}
			}
		}

	}
}
