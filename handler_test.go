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

func TestMiddlewareUseAccessKey(t *testing.T) {
	auth := jwtauth.New("HS256", []byte("secret"), nil)

	cfg := newConfig()
	server, cleanup := test.NewServer(&cfg)
	t.Cleanup(cleanup)

	now := time.Now()
	project := uint64(7)
	key := proto.GenerateAccessKey(project)
	service := proto.Service_Indexer

	const _credits = middleware.DefaultPublicRate / 10

	limit := proto.Limit{
		RateLimit: _credits * 100,
		FreeWarn:  _credits * 5,
		FreeMax:   _credits * 5,
		OverWarn:  _credits * 7,
		OverMax:   _credits * 10,
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
		middleware.Session(client, auth, nil, nil),
		addCredits(_credits*2).Middleware,
		addCredits(_credits*-1).Middleware,
		middleware.RateLimit(cfg.RateLimiter, cfg.Redis, nil),
		middleware.SpendUsage(client, nil),
	)

	r.Handle("/*", &counter)

	expectedUsage := proto.AccessUsage{}

	t.Run("WithAccessKey", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)
		server.FlushNotifications()

		// Spend Free CU
		for i := int64(_credits); i < limit.FreeWarn; i += _credits {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, strconv.FormatInt(limit.FreeMax-i, 10), headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, "", headers.Get(middleware.HeaderQuotaOverage))
			assert.Empty(t, server.GetEvents(project), i)
			expectedUsage.Add(proto.AccessUsage{ValidCompute: _credits})
		}

		// Go over free CU
		ok, headers, err := executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaOverage))
		assert.Contains(t, server.GetEvents(project), proto.EventType_FreeMax)
		expectedUsage.Add(proto.AccessUsage{ValidCompute: _credits})

		// Get close to soft quota
		for i := limit.FreeWarn + _credits; i < limit.OverWarn; i += _credits {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, strconv.FormatInt(i-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
			assert.Len(t, server.GetEvents(project), 1)
			expectedUsage.Add(proto.AccessUsage{OverCompute: _credits})
		}

		// Go over soft quota
		ok, headers, err = executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
		assert.Equal(t, strconv.FormatInt(limit.OverWarn-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
		assert.Contains(t, server.GetEvents(project), proto.EventType_OverWarn)
		expectedUsage.Add(proto.AccessUsage{OverCompute: _credits})

		// Get close to hard quota
		for i := limit.OverWarn + _credits; i < limit.OverMax; i += _credits {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, strconv.FormatInt(i-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
			assert.Len(t, server.GetEvents(project), 2)
			expectedUsage.Add(proto.AccessUsage{OverCompute: _credits})
		}

		// Go over hard quota
		ok, headers, err = executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
		assert.Equal(t, strconv.FormatInt(limit.OverMax-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
		assert.Contains(t, server.GetEvents(project), proto.EventType_OverMax)
		expectedUsage.Add(proto.AccessUsage{OverCompute: _credits})

		// Denied
		for i := 0; i < 10; i++ {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.ErrorIs(t, err, proto.ErrLimitExceeded)
			assert.False(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, strconv.FormatInt(limit.OverMax-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
			expectedUsage.Add(proto.AccessUsage{LimitedCompute: _credits})
		}

		// check the usage
		client.Stop(context.Background())
		usage, err := server.Store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), _credits*counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ChangeLimits", func(t *testing.T) {
		// Increase CreditsOverageLimit which should still allow requests to go through, etc.
		err = server.Store.SetAccessLimit(ctx, project, &proto.Limit{
			RateLimit: _credits * 100,
			OverWarn:  _credits * 5,
			OverMax:   _credits * 110,
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
		expectedUsage.Add(proto.AccessUsage{ValidCompute: 0, OverCompute: _credits, LimitedCompute: 0})
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), _credits*counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("PublicRateLimit", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)

		for i, max := 0, cfg.RateLimiter.PublicRate*2; i < max; i += _credits {
			ok, headers, err := executeRequest(ctx, r, "", "", "")
			if i < cfg.RateLimiter.PublicRate {
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
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), _credits*counter.GetValue())
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
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), _credits*counter.GetValue())
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
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), _credits*counter.GetValue())
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

	service := proto.Service_Metadata
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

	client := newQuotaClient(cfg, service)

	aq, err := client.FetchKeyQuota(ctx, keys[0], "", now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	aq, err = client.FetchKeyQuota(ctx, keys[0], "", now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	access, err = server.UpdateAccessKey(ctx, keys[0], proto.Ptr("new name"), nil, []proto.Service{service})
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
		middleware.Session(client, auth, nil, nil),
		middleware.EnsureUsage(client, nil),
		middleware.SpendUsage(client, nil),
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
		middleware.Session(client, auth, nil, nil),
		middleware.RateLimit(cfg.RateLimiter, cfg.Redis, nil),
		middleware.EnsurePermission(client, UserPermission_READ_WRITE, nil),
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
		assert.Equal(t, strconv.FormatInt(limit.RateLimit, 10), headers.Get(middleware.HeaderCreditsLimit))
	})

	server.Store.SetUserPermission(ctx, project, account, proto.UserPermission_READ_WRITE, proto.ResourceAccess{ProjectID: project})
	server.FlushCache(ctx)
	t.Run("EnoughPermission", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, strconv.FormatInt(limit.RateLimit, 10), headers.Get(middleware.HeaderCreditsLimit))
		expectedHits++
	})

	server.Store.SetUserPermission(ctx, project, account, proto.UserPermission_ADMIN, proto.ResourceAccess{ProjectID: project})
	server.FlushCache(ctx)
	t.Run("MorePermission", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, strconv.FormatInt(limit.RateLimit, 10), headers.Get(middleware.HeaderCreditsLimit))
		expectedHits++
	})

	assert.Equal(t, expectedHits, counter.GetValue())
}

func TestSession(t *testing.T) {
	const (
		project     = uint64(7)
		accessKey   = "AQAAAAAAAAAHkL0mNSrn6Sm3oHs0xfa_DnY"
		service     = proto.Service_Indexer
		address     = "walletAddress"
		userAddress = "userAddress"
		serviceName = "serviceName"
	)

	auth := jwtauth.New("HS256", []byte("secret"), nil)
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
		MethodUser      = "MethodUser"
		MethodAdmin     = "MethodAdmin"
		MethodService   = "MethodService"
	)

	ACL := middleware.ServiceConfig[middleware.ACL]{
		"Service": {
			MethodPublic:    middleware.NewACL(proto.SessionType_Public.OrHigher()...),
			MethodAccount:   middleware.NewACL(proto.SessionType_Wallet.OrHigher()...),
			MethodAccessKey: middleware.NewACL(proto.SessionType_AccessKey.OrHigher()...),
			MethodProject:   middleware.NewACL(proto.SessionType_Project.OrHigher()...),
			MethodUser:      middleware.NewACL(proto.SessionType_User.OrHigher()...),
			MethodAdmin:     middleware.NewACL(proto.SessionType_Admin.OrHigher()...),
			MethodService:   middleware.NewACL(proto.SessionType_Service.OrHigher()...),
		},
	}

	r := chi.NewRouter()
	r.Use(
		middleware.Session(client, auth, server.Store, nil),
		middleware.RateLimit(cfg.RateLimiter, cfg.Redis, nil),
		middleware.AccessControl(ACL, nil, 1, nil),
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
	server.Store.AddUser(ctx, userAddress, false)
	server.Store.SetAccessLimit(ctx, project, &limit)
	server.Store.SetUserPermission(ctx, project, address, proto.UserPermission_READ, proto.ResourceAccess{ProjectID: project})
	server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: accessKey, ProjectID: project})

	type testCase struct {
		AccessKey string
		Session   proto.SessionType
	}

	testCases := []testCase{
		{
			Session: proto.SessionType_Public,
		},
		{
			Session: proto.SessionType_Wallet,
		},
		{
			Session:   proto.SessionType_AccessKey,
			AccessKey: accessKey,
		},
		{
			Session: proto.SessionType_Project,
		},
		{
			Session:   proto.SessionType_Project,
			AccessKey: accessKey,
		},
		{
			Session: proto.SessionType_User,
		},
		{
			Session: proto.SessionType_Admin,
		},
		{
			Session:   proto.SessionType_Admin,
			AccessKey: accessKey,
		},
		{
			Session: proto.SessionType_Service,
		},
		{
			Session:   proto.SessionType_Service,
			AccessKey: accessKey,
		},
	}

	var (
		publicRPM  = fmt.Sprint(middleware.DefaultPublicRate)
		accountRPM = fmt.Sprint(middleware.DefaultAccountRate)
		serviceRPM = ""
		quotaRPM   = fmt.Sprint(limit.RateLimit)
	)

	methods := []string{
		MethodPublic,
		MethodAccount,
		MethodAccessKey,
		MethodProject,
		MethodUser,
		MethodAdmin,
		MethodService,
	}

	for service := range ACL {
		for _, method := range methods {
			types := ACL[service][method]
			for _, tc := range testCases {
				t.Run(fmt.Sprintf("%s/%s/%s", method, tc.Session, tc.AccessKey), func(t *testing.T) {
					var claims middleware.Claims
					switch tc.Session {
					case proto.SessionType_Wallet:
						claims = middleware.Claims{"account": address}
					case proto.SessionType_Project:
						claims = middleware.Claims{"account": address, "project": project}
					case proto.SessionType_User:
						claims = middleware.Claims{"account": userAddress}
					case proto.SessionType_Admin:
						claims = middleware.Claims{"account": address, "admin": true}
					case proto.SessionType_Service:
						claims = middleware.Claims{"service": serviceName}
					}

					ok, h, err := executeRequest(ctx, r, "/rpc/"+service+"/"+method, tc.AccessKey, mustJWT(t, auth, claims))
					if !types.Includes(tc.Session) {
						assert.Error(t, err)
						assert.False(t, ok)
						return
					}

					assert.NoError(t, err, "%s/%s %+v", service, method, tc)
					assert.True(t, ok)
					rateLimit := h.Get(middleware.HeaderCreditsLimit)
					switch tc.Session {
					case proto.SessionType_Public:
						assert.Equal(t, publicRPM, rateLimit)
					case proto.SessionType_AccessKey, proto.SessionType_Project:
						assert.Equal(t, quotaRPM, rateLimit)
						assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), h.Get(middleware.HeaderQuotaLimit))
					case proto.SessionType_Wallet, proto.SessionType_Admin, proto.SessionType_User:
						assert.Equal(t, accountRPM, rateLimit)
					case proto.SessionType_Service:
						assert.Equal(t, serviceRPM, rateLimit)
					}
				})
			}
		}

	}
}
