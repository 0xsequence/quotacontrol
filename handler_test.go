package quotacontrol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/encoding"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/0xsequence/quotacontrol/tests/mock"
	"github.com/go-chi/chi/v5"
	"github.com/goware/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xsequence/authcontrol"
)

var (
	Secret        = "secret"
	ProjectID     = uint64(7)
	AccessKey     = "AQAAAAAAAAAHkL0mNSrn6Sm3oHs0xfa_DnY"
	Service       = proto.Service_Indexer
	WalletAddress = "walletAddress"
	UserAddress   = "userAddress"
	ServiceName   = "serviceName"
)

func TestMiddlewareUseAccessKey(t *testing.T) {
	cfg := newConfig()
	server, cleanup := mock.NewServer(&cfg)
	t.Cleanup(cleanup)

	now := time.Now()
	key := proto.GenerateAccessKey(encoding.WithVersion(context.Background(), 1), ProjectID)

	const _credits = middleware.DefaultPublicRate / 10

	limit := proto.Limit{
		RateLimit: _credits * 100,
		FreeWarn:  _credits * 5,
		FreeMax:   _credits * 5,
		OverWarn:  _credits * 7,
		OverMax:   _credits * 10,
	}

	ctx := context.Background()
	err := server.Store.SetAccessLimit(ctx, ProjectID, &limit)
	require.NoError(t, err)
	err = server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: key, ProjectID: ProjectID})
	require.NoError(t, err)

	logger := logger.NewLogger(logger.LogLevel_INFO)
	client := quotacontrol.NewClient(logger, Service, cfg, nil)

	counter := spendingCounter(0)

	addCost := func(i int64) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(middleware.AddCost(r.Context(), int64(i))))
			})
		}
	}

	authOptions := authcontrol.Options{
		JWTSecret: Secret,
	}
	quotaOptions := middleware.Options{}

	limitCounter := quotacontrol.NewLimitCounter(cfg.Redis, logger)

	r := chi.NewRouter()
	r.Use(authcontrol.VerifyToken(authOptions))
	r.Use(authcontrol.Session(authOptions))
	r.Use(middleware.VerifyQuota(client, quotaOptions))
	r.Use(addCost(_credits * 2))
	r.Use(addCost(_credits * -1))
	r.Use(middleware.RateLimit(cfg.RateLimiter, limitCounter, quotaOptions))
	r.Use(middleware.SpendUsage(client, quotaOptions))

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
			assert.Empty(t, server.GetEvents(ProjectID), i)
			expectedUsage.Add(proto.AccessUsage{ValidCompute: _credits})
		}

		// Go over free CU
		ok, headers, err := executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaOverage))
		assert.Contains(t, server.GetEvents(ProjectID), proto.EventType_FreeMax)
		expectedUsage.Add(proto.AccessUsage{ValidCompute: _credits})

		// Get close to soft quota
		for i := limit.FreeWarn + _credits; i < limit.OverWarn; i += _credits {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, strconv.FormatInt(i-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
			assert.Len(t, server.GetEvents(ProjectID), 1)
			expectedUsage.Add(proto.AccessUsage{OverCompute: _credits})
		}

		// Go over soft quota
		ok, headers, err = executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
		assert.Equal(t, strconv.FormatInt(limit.OverWarn-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
		assert.Contains(t, server.GetEvents(ProjectID), proto.EventType_OverWarn)
		expectedUsage.Add(proto.AccessUsage{OverCompute: _credits})

		// Get close to hard quota
		for i := limit.OverWarn + _credits; i < limit.OverMax; i += _credits {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, strconv.FormatInt(i-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
			assert.Len(t, server.GetEvents(ProjectID), 2)
			expectedUsage.Add(proto.AccessUsage{OverCompute: _credits})
		}

		// Go over hard quota
		ok, headers, err = executeRequest(ctx, r, "", key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
		assert.Equal(t, strconv.FormatInt(limit.OverMax-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
		assert.Contains(t, server.GetEvents(ProjectID), proto.EventType_OverMax)
		expectedUsage.Add(proto.AccessUsage{OverCompute: _credits})

		// Denied
		for i := 0; i < 10; i++ {
			ok, headers, err := executeRequest(ctx, r, "", key, "")
			assert.ErrorIs(t, err, proto.ErrQuotaExceeded)
			assert.False(t, ok)
			assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
			assert.Equal(t, "0", headers.Get(middleware.HeaderQuotaRemaining))
			assert.Equal(t, strconv.FormatInt(limit.OverMax-limit.FreeWarn, 10), headers.Get(middleware.HeaderQuotaOverage))
			expectedUsage.Add(proto.AccessUsage{LimitedCompute: _credits})
		}

		// check the usage
		client.Stop(context.Background())
		usage, err := server.Store.GetAccountUsage(ctx, ProjectID, &Service, now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), _credits*counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ChangeLimits", func(t *testing.T) {
		// Increase CreditsOverageLimit which should still allow requests to go through, etc.
		err = server.Store.SetAccessLimit(ctx, ProjectID, &proto.Limit{
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
		usage, err := server.Store.GetAccountUsage(ctx, ProjectID, &Service, now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		expectedUsage.Add(proto.AccessUsage{ValidCompute: 0, OverCompute: _credits, LimitedCompute: 0})
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), _credits*counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("PublicRateLimit", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)

		for i := 0; i < cfg.RateLimiter.PublicRPM*2; i += _credits {
			ok, headers, err := executeRequest(ctx, r, "", "", "")
			if i < cfg.RateLimiter.PublicRPM {
				assert.NoError(t, err, i)
				assert.True(t, ok, i)
				assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
			} else {
				assert.ErrorIs(t, err, proto.ErrQuotaExceeded, i)
				assert.False(t, ok, i)
				assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
			}
		}

		client.Stop(context.Background())
		usage, err := server.Store.GetAccountUsage(ctx, ProjectID, &Service, now.Add(-time.Hour), now.Add(time.Hour))
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
		usage, err := server.Store.GetAccountUsage(ctx, ProjectID, &Service, now.Add(-time.Hour), now.Add(time.Hour))
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
		usage, err := server.Store.GetAccountUsage(ctx, ProjectID, &Service, now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), _credits*counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})
}

func TestDefaultKey(t *testing.T) {
	cfg := newConfig()
	server, cleanup := mock.NewServer(&cfg)
	t.Cleanup(cleanup)

	now := time.Now()
	keys := []string{
		proto.GenerateAccessKey(encoding.WithVersion(context.Background(), 1), ProjectID),
		proto.GenerateAccessKey(encoding.WithVersion(context.Background(), 1), ProjectID),
	}

	limit := proto.Limit{
		RateLimit: 100,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}

	access := &proto.AccessKey{
		Active:    true,
		AccessKey: keys[0],
		ProjectID: ProjectID,
	}

	// populate store
	ctx := context.Background()
	err := server.Store.SetAccessLimit(ctx, ProjectID, &limit)
	require.NoError(t, err)
	err = server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: keys[0], ProjectID: ProjectID})
	require.NoError(t, err)

	logger := logger.NewLogger(logger.LogLevel_INFO)
	client := quotacontrol.NewClient(logger, Service, cfg, nil)

	aq, err := client.FetchKeyQuota(ctx, keys[0], "", now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	aq, err = client.FetchKeyQuota(ctx, keys[0], "", now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	access, err = server.UpdateAccessKey(ctx, keys[0], proto.Ptr("new name"), nil, nil, []proto.Service{Service})
	require.NoError(t, err)

	aq, err = client.FetchKeyQuota(ctx, keys[0], "", now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	ok, err := server.DisableAccessKey(ctx, keys[0])
	require.ErrorIs(t, err, proto.ErrAtLeastOneKey)
	assert.False(t, ok)
	newAccess := proto.AccessKey{Active: true, AccessKey: keys[1], ProjectID: ProjectID}
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
	key := proto.GenerateAccessKey(encoding.WithVersion(context.Background(), 1), ProjectID)

	counter := spendingCounter(0)

	cfg := newConfig()
	server, cleanup := mock.NewServer(&cfg)
	t.Cleanup(cleanup)

	logger := logger.NewLogger(logger.LogLevel_INFO)
	client := quotacontrol.NewClient(logger, Service, cfg, nil)

	authOptions := authcontrol.Options{
		JWTSecret: Secret,
	}
	quotaOptions := middleware.Options{
		BaseRequestCost: 1,
	}

	r := chi.NewRouter()
	r.Use(authcontrol.VerifyToken(authOptions))
	r.Use(authcontrol.Session(authOptions))
	r.Use(middleware.VerifyQuota(client, quotaOptions))
	r.Use(middleware.EnsureUsage(client, quotaOptions))
	r.Use(middleware.SpendUsage(client, quotaOptions))
	r.Handle("/*", &counter)

	ctx := context.Background()

	limit := proto.Limit{
		RateLimit: 100,
		FreeWarn:  5,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}
	server.Store.SetAccessLimit(ctx, ProjectID, &limit)

	token := authcontrol.S2SToken(Secret, map[string]any{"project": ProjectID, "account": WalletAddress})

	var expectedHits int64

	t.Run("UnauthorizedUser", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.ErrorIs(t, err, proto.ErrUnauthorizedUser)
		assert.False(t, ok)
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
	})
	server.Store.SetUserPermission(ctx, ProjectID, WalletAddress, proto.UserPermission_READ_WRITE, proto.ResourceAccess{ProjectID: ProjectID})
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
	server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: key, ProjectID: ProjectID})
	t.Run("AccessKeyFound", func(t *testing.T) {
		ok, _, err := executeRequest(ctx, r, "", key, token)
		require.NoError(t, err)
		assert.True(t, ok)
		expectedHits++
	})

	t.Run("AccessKeyMismatch", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", proto.GenerateAccessKey(encoding.WithVersion(context.Background(), 1), ProjectID+1), token)
		require.ErrorIs(t, err, proto.ErrAccessKeyMismatch)
		assert.False(t, ok)
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
	})

	assert.Equal(t, expectedHits, counter.GetValue())
}

func TestJWTAccess(t *testing.T) {
	account := "account"

	counter := hitCounter(0)

	cfg := newConfig()
	server, cleanup := mock.NewServer(&cfg)
	t.Cleanup(cleanup)

	logger := logger.NewLogger(logger.LogLevel_INFO)
	client := quotacontrol.NewClient(logger, Service, cfg, nil)

	limitCounter := quotacontrol.NewLimitCounter(cfg.Redis, logger)

	authOptions := authcontrol.Options{
		JWTSecret: Secret,
	}
	quotaOptions := middleware.Options{}

	r := chi.NewRouter()
	r.Use(authcontrol.VerifyToken(authOptions))
	r.Use(authcontrol.Session(authOptions))
	r.Use(middleware.VerifyQuota(client, quotaOptions))
	r.Use(middleware.RateLimit(cfg.RateLimiter, limitCounter, quotaOptions))
	r.Use(middleware.EnsurePermission(client, proto.UserPermission_READ_WRITE, quotaOptions))

	r.Handle("/*", &counter)

	ctx := context.Background()
	limit := proto.Limit{
		RateLimit: 100,
		FreeWarn:  5,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}
	server.Store.SetAccessLimit(ctx, ProjectID, &limit)

	token := authcontrol.S2SToken(Secret, map[string]any{"account": account, "project": ProjectID})

	var expectedHits int64

	t.Run("NoPermission", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.ErrorIs(t, err, proto.ErrUnauthorizedUser)
		assert.False(t, ok)
		assert.Equal(t, "", headers.Get(middleware.HeaderQuotaLimit))
	})

	server.Store.SetUserPermission(ctx, ProjectID, account, proto.UserPermission_READ, proto.ResourceAccess{ProjectID: ProjectID})
	t.Run("LowPermission", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.ErrorIs(t, err, proto.ErrUnauthorizedUser)
		assert.False(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, strconv.FormatInt(limit.RateLimit, 10), headers.Get(middleware.HeaderCreditsLimit))
	})

	server.Store.SetUserPermission(ctx, ProjectID, account, proto.UserPermission_READ_WRITE, proto.ResourceAccess{ProjectID: ProjectID})
	server.FlushCache(ctx)
	t.Run("EnoughPermission", func(t *testing.T) {
		ok, headers, err := executeRequest(ctx, r, "", "", token)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, strconv.FormatInt(limit.FreeMax, 10), headers.Get(middleware.HeaderQuotaLimit))
		assert.Equal(t, strconv.FormatInt(limit.RateLimit, 10), headers.Get(middleware.HeaderCreditsLimit))
		expectedHits++
	})

	server.Store.SetUserPermission(ctx, ProjectID, account, proto.UserPermission_ADMIN, proto.ResourceAccess{ProjectID: ProjectID})
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

const (
	MethodPublic    = "MethodPublic"
	MethodAccount   = "MethodAccount"
	MethodAccessKey = "MethodAccessKey"
	MethodProject   = "MethodProject"
	MethodUser      = "MethodUser"
	MethodAdmin     = "MethodAdmin"
	MethodService   = "MethodService"
)

var Methods = []string{MethodPublic, MethodAccount, MethodAccessKey, MethodProject, MethodUser, MethodAdmin, MethodService}

var ACL = authcontrol.Config[authcontrol.ACL]{
	"Service": {
		MethodPublic:    authcontrol.NewACL(proto.SessionType_Public.OrHigher()...),
		MethodAccount:   authcontrol.NewACL(proto.SessionType_Wallet.OrHigher()...),
		MethodAccessKey: authcontrol.NewACL(proto.SessionType_AccessKey.OrHigher()...),
		MethodProject:   authcontrol.NewACL(proto.SessionType_Project.OrHigher()...),
		MethodUser:      authcontrol.NewACL(proto.SessionType_User.OrHigher()...),
		MethodAdmin:     authcontrol.NewACL(proto.SessionType_Admin.OrHigher()...),
		MethodService:   authcontrol.NewACL(proto.SessionType_InternalService.OrHigher()...),
	},
}

func TestSession(t *testing.T) {
	counter := hitCounter(0)

	cfg := newConfig()
	server, cleanup := mock.NewServer(&cfg)
	t.Cleanup(cleanup)

	logger := logger.NewLogger(logger.LogLevel_INFO)
	client := quotacontrol.NewClient(logger, Service, cfg, nil)

	authOptions := authcontrol.Options{
		JWTSecret:    Secret,
		UserStore:    server.Store,
		ProjectStore: server.Store,
	}
	quotaOptions := middleware.Options{}

	limitCounter := quotacontrol.NewLimitCounter(cfg.Redis, logger)

	r := chi.NewRouter()
	r.Use(authcontrol.VerifyToken(authOptions))
	r.Use(authcontrol.Session(authOptions))
	r.Use(authcontrol.AccessControl(ACL, authOptions))
	r.Use(middleware.VerifyQuota(client, quotaOptions))
	r.Use(middleware.RateLimit(cfg.RateLimiter, limitCounter, quotaOptions))

	r.Handle("/*", &counter)

	ctx := context.Background()
	limit := proto.Limit{RateLimit: 100, FreeWarn: 5, FreeMax: 5, OverWarn: 7, OverMax: 10}
	server.Store.AddUser(ctx, UserAddress, false)
	server.Store.AddProject(ctx, ProjectID, nil)
	server.Store.SetAccessLimit(ctx, ProjectID, &limit)
	server.Store.SetUserPermission(ctx, ProjectID, WalletAddress, proto.UserPermission_READ, proto.ResourceAccess{ProjectID: ProjectID})
	server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: AccessKey, ProjectID: ProjectID})

	testCases := []struct {
		AccessKey string
		Session   proto.SessionType
	}{
		{Session: proto.SessionType_Public},
		{Session: proto.SessionType_Wallet},
		{Session: proto.SessionType_AccessKey, AccessKey: AccessKey},
		{Session: proto.SessionType_AccessKey, AccessKey: AccessKey + "a"},
		{Session: proto.SessionType_Project},
		{Session: proto.SessionType_Project, AccessKey: AccessKey},
		{Session: proto.SessionType_User},
		{Session: proto.SessionType_Admin},
		{Session: proto.SessionType_Admin, AccessKey: AccessKey},
		{Session: proto.SessionType_InternalService},
		{Session: proto.SessionType_InternalService, AccessKey: AccessKey},
	}

	var (
		publicRPM  = fmt.Sprint(middleware.DefaultPublicRate)
		accountRPM = fmt.Sprint(middleware.DefaultAccountRate)
		serviceRPM = ""
		quotaRPM   = fmt.Sprint(limit.RateLimit)
		quotaLimit = strconv.FormatInt(limit.FreeMax, 10)
	)

	for service := range ACL {
		for _, method := range Methods {
			types := ACL[service][method]
			for _, tc := range testCases {
				name := fmt.Sprintf("%s/%s", method, tc.Session)
				if tc.AccessKey != "" {
					name += "+Key"
				}
				t.Run(name, func(t *testing.T) {
					var claims map[string]any
					switch tc.Session {
					case proto.SessionType_Wallet:
						claims = map[string]any{"account": WalletAddress}
					case proto.SessionType_Project:
						claims = map[string]any{"account": WalletAddress, "project": ProjectID}
					case proto.SessionType_User:
						claims = map[string]any{"account": UserAddress}
					case proto.SessionType_Admin:
						claims = map[string]any{"account": WalletAddress, "admin": true}
					case proto.SessionType_InternalService:
						claims = map[string]any{"service": ServiceName}
					}

					ok, h, err := executeRequest(ctx, r, "/rpc/"+service+"/"+method, tc.AccessKey, authcontrol.S2SToken(Secret, claims))
					if !types.Includes(tc.Session) {
						assert.Error(t, err)
						assert.False(t, ok)
						return
					}

					rateLimit := h.Get(middleware.HeaderCreditsLimit)
					switch tc.Session {
					case proto.SessionType_Public:
						assert.True(t, ok)
						assert.NoError(t, err)
						assert.Equal(t, publicRPM, rateLimit)
					case proto.SessionType_AccessKey:
						if tc.AccessKey == AccessKey {
							assert.True(t, ok)
							assert.NoError(t, err)
							assert.Equal(t, quotaRPM, rateLimit)
							assert.Equal(t, quotaLimit, h.Get(middleware.HeaderQuotaLimit))
						} else {
							assert.False(t, ok)
							assert.ErrorIs(t, err, proto.ErrAccessKeyNotFound)
						}
					case proto.SessionType_Project:
						assert.True(t, ok)
						assert.NoError(t, err)
						assert.Equal(t, quotaRPM, rateLimit)
						assert.Equal(t, quotaLimit, h.Get(middleware.HeaderQuotaLimit))
					case proto.SessionType_Wallet, proto.SessionType_User:
						assert.True(t, ok)
						assert.NoError(t, err)
						limit := accountRPM
						if tc.AccessKey != "" {
							limit = quotaRPM
						}
						assert.Equal(t, limit, rateLimit)
					case proto.SessionType_InternalService:
						assert.True(t, ok)
						assert.NoError(t, err)
						assert.Equal(t, serviceRPM, rateLimit)
					}
				})
			}
		}
	}
}

func TestSessionDisabled(t *testing.T) {
	counter := hitCounter(0)

	cfg := newConfig()
	cfg.Enabled = false
	server, cleanup := mock.NewServer(&cfg)
	t.Cleanup(cleanup)

	logger := logger.NewLogger(logger.LogLevel_INFO)
	client := quotacontrol.NewClient(logger, Service, cfg, nil)

	authOptions := authcontrol.Options{
		JWTSecret:    Secret,
		UserStore:    server.Store,
		ProjectStore: server.Store,
	}
	quotaOptions := middleware.Options{}

	limitCounter := quotacontrol.NewLimitCounter(cfg.Redis, logger)

	r := chi.NewRouter()
	r.Use(authcontrol.VerifyToken(authOptions))
	r.Use(authcontrol.Session(authOptions))
	r.Use(authcontrol.AccessControl(ACL, authOptions))
	r.Use(middleware.VerifyQuota(client, quotaOptions))
	r.Use(middleware.RateLimit(cfg.RateLimiter, limitCounter, quotaOptions))

	r.Handle("/*", &counter)

	ctx := context.Background()
	limit := proto.Limit{RateLimit: 100, FreeWarn: 5, FreeMax: 5, OverWarn: 7, OverMax: 10}
	server.Store.AddUser(ctx, UserAddress, false)
	server.Store.AddProject(ctx, ProjectID, nil)
	server.Store.SetAccessLimit(ctx, ProjectID, &limit)
	server.Store.SetUserPermission(ctx, ProjectID, WalletAddress, proto.UserPermission_READ, proto.ResourceAccess{ProjectID: ProjectID})
	server.Store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: AccessKey, ProjectID: ProjectID})

	testCases := []struct {
		AccessKey string
		Session   proto.SessionType
	}{
		{Session: proto.SessionType_Public},
		{Session: proto.SessionType_Wallet},
		{Session: proto.SessionType_AccessKey, AccessKey: AccessKey},
		{Session: proto.SessionType_Project},
		{Session: proto.SessionType_Project, AccessKey: AccessKey},
		{Session: proto.SessionType_User},
		{Session: proto.SessionType_Admin},
		{Session: proto.SessionType_Admin, AccessKey: AccessKey},
		{Session: proto.SessionType_InternalService},
		{Session: proto.SessionType_InternalService, AccessKey: AccessKey},
	}

	var (
		publicRPM  = fmt.Sprint(middleware.DefaultPublicRate)
		accountRPM = fmt.Sprint(middleware.DefaultAccountRate)
		serviceRPM = ""
		quotaRPM   = fmt.Sprint(limit.RateLimit)
		quotaLimit = strconv.FormatInt(limit.FreeMax, 10)
	)

	for service := range ACL {
		for _, method := range Methods {
			types := ACL[service][method]
			for _, tc := range testCases {
				name := fmt.Sprintf("%s/%s", method, tc.Session)
				if tc.AccessKey != "" {
					name += "+Key"
				}
				t.Run(name, func(t *testing.T) {
					var claims map[string]any
					switch tc.Session {
					case proto.SessionType_Wallet:
						claims = map[string]any{"account": WalletAddress}
					case proto.SessionType_Project:
						claims = map[string]any{"account": WalletAddress, "project": ProjectID}
					case proto.SessionType_User:
						claims = map[string]any{"account": UserAddress}
					case proto.SessionType_Admin:
						claims = map[string]any{"account": WalletAddress, "admin": true}
					case proto.SessionType_InternalService:
						claims = map[string]any{"service": ServiceName}
					}

					ok, h, err := executeRequest(ctx, r, "/rpc/"+service+"/"+method, tc.AccessKey, authcontrol.S2SToken(Secret, claims))
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
						assert.Equal(t, quotaLimit, h.Get(middleware.HeaderQuotaLimit))
					case proto.SessionType_Wallet, proto.SessionType_User:
						assert.Equal(t, accountRPM, rateLimit)
					case proto.SessionType_Admin:
						limit := accountRPM
						if tc.AccessKey != "" {
							limit = quotaRPM
						}
						assert.Equal(t, limit, rateLimit)
					case proto.SessionType_InternalService:
						assert.Equal(t, serviceRPM, rateLimit)
					}
				})
			}
		}
	}
}

func newConfig() quotacontrol.Config {
	return quotacontrol.Config{
		Enabled:    true,
		UpdateFreq: time.Minute,
		Redis: quotacontrol.RedisConfig{
			Enabled: true,
		},
		RateLimiter: quotacontrol.RateLimitConfig{
			Enabled: true,
		},
	}
}

type hitCounter int64

func (c *hitCounter) GetValue() int64 { return atomic.LoadInt64((*int64)(c)) }
func (c *hitCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64((*int64)(c), 1)
	w.WriteHeader(http.StatusOK)
}

type spendingCounter int64

func (c *spendingCounter) GetValue() int64 { return atomic.LoadInt64((*int64)(c)) }
func (c *spendingCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// up the counter only if quota control run
	if middleware.HasSpending(r.Context()) {
		atomic.AddInt64((*int64)(c), 1)
	}
	w.WriteHeader(http.StatusOK)
}

func executeRequest(ctx context.Context, handler http.Handler, path, accessKey, jwt string) (bool, http.Header, error) {
	req, err := http.NewRequest("POST", path, nil)
	if err != nil {
		return false, nil, err
	}
	req.Header.Set("X-Real-IP", "127.0.0.1")
	if accessKey != "" {
		req.Header.Set(authcontrol.HeaderAccessKey, accessKey)
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Result().StatusCode; status < http.StatusOK || status >= http.StatusBadRequest {
		w := proto.WebRPCError{}
		json.Unmarshal(rr.Body.Bytes(), &w)
		return false, rr.Header(), w
	}

	return true, rr.Header(), nil
}
