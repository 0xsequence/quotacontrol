package quotacontrol_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddlewareUseAccessKey(t *testing.T) {
	cfg := newConfig()
	server := newTestServer(t, &cfg)

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
	err := server.store.SetAccessLimit(ctx, project, &limit)
	require.NoError(t, err)
	err = server.store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: key, ProjectID: project})
	require.NoError(t, err)

	client := newQuotaClient(cfg, service)

	counter := spendingCounter(0)

	r := chi.NewRouter()
	r.Use(
		middleware.Credentials(nil),
		middleware.VerifyQuota(client),
		addCredits(2).Middleware,
		addCredits(-1).Middleware,
		RateLimiter(cfg, nil),
		middleware.SpendUsage(client),
	)

	r.Handle("/*", &counter)

	expectedUsage := proto.AccessUsage{}

	t.Run("WithAccessKey", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)
		server.notifications = make(map[uint64][]proto.EventType)

		// Spend Free CU
		for i := int64(1); i < limit.FreeWarn; i++ {
			ok, err := executeRequest(ctx, r, key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Empty(t, server.getEvents(project), i)
			expectedUsage.Add(proto.AccessUsage{ValidCompute: 1})
		}

		// Go over free CU
		ok, err := executeRequest(ctx, r, key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, server.getEvents(project), proto.EventType_FreeMax)
		expectedUsage.Add(proto.AccessUsage{ValidCompute: 1})

		// Get close to soft quota
		for i := limit.FreeWarn + 1; i < limit.OverWarn; i++ {
			ok, err := executeRequest(ctx, r, key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Len(t, server.getEvents(project), 1)
			expectedUsage.Add(proto.AccessUsage{OverCompute: 1})
		}

		// Go over soft quota
		ok, err = executeRequest(ctx, r, key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, server.getEvents(project), proto.EventType_OverWarn)
		expectedUsage.Add(proto.AccessUsage{OverCompute: 1})

		// Get close to hard quota
		for i := limit.OverWarn + 1; i < limit.OverMax; i++ {
			ok, err := executeRequest(ctx, r, key, "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Len(t, server.getEvents(project), 2)
			expectedUsage.Add(proto.AccessUsage{OverCompute: 1})
		}

		// Go over hard quota
		ok, err = executeRequest(ctx, r, key, "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, server.getEvents(project), proto.EventType_OverMax)
		expectedUsage.Add(proto.AccessUsage{OverCompute: 1})

		// Denied
		for i := 0; i < 10; i++ {
			ok, err := executeRequest(ctx, r, key, "")
			assert.ErrorIs(t, err, proto.ErrLimitExceeded)
			assert.False(t, ok)
			expectedUsage.Add(proto.AccessUsage{LimitedCompute: 1})
		}

		// check the usage
		client.Stop(context.Background())
		usage, err := server.store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ChangeLimits", func(t *testing.T) {
		// Increase CreditsOverageLimit which should still allow requests to go through, etc.
		err = server.store.SetAccessLimit(ctx, project, &proto.Limit{
			RateLimit: 100,
			OverWarn:  5,
			OverMax:   110,
		})
		assert.NoError(t, err)
		err = client.ClearQuotaCacheByAccessKey(ctx, key)
		assert.NoError(t, err)

		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)
		server.notifications = make(map[uint64][]proto.EventType)

		ok, err := executeRequest(ctx, r, key, "")
		assert.NoError(t, err)
		assert.True(t, ok)

		client.Stop(context.Background())
		usage, err := server.store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		expectedUsage.Add(proto.AccessUsage{ValidCompute: 0, OverCompute: 1, LimitedCompute: 0})
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("PublicRateLimit", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)

		for i, max := 0, cfg.RateLimiter.PublicRPM*2; i < max; i++ {
			ok, err := executeRequest(ctx, r, "", "")
			if i < cfg.RateLimiter.PublicRPM {
				assert.NoError(t, err, i)
				assert.True(t, ok, i)
			} else {
				assert.ErrorIs(t, err, proto.ErrLimitExceeded, i)
				assert.False(t, ok, i)
			}
		}

		client.Stop(context.Background())
		usage, err := server.store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ServerErrors", func(t *testing.T) {
		server.FlushCache()

		go client.Run(context.Background())

		errList := []error{
			errors.New("unexpected error"),
			proto.ErrWebrpcBadRoute,
			proto.ErrTimeout,
		}

		ctx := middleware.WithTime(context.Background(), now)

		for _, err := range errList {
			server.ErrGetAccessQuota = err
			ok, err := executeRequest(ctx, r, key, "")
			assert.True(t, ok)
			assert.NoError(t, err)
		}
		server.ErrGetAccessQuota = nil

		server.FlushCache()

		for _, err := range errList {
			server.ErrPrepareUsage = err
			ok, err := executeRequest(ctx, r, key, "")
			assert.True(t, ok)
			assert.NoError(t, err)
		}
		server.ErrPrepareUsage = nil

		client.Stop(context.Background())
		usage, err := server.store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ServerTimeout", func(t *testing.T) {
		server.FlushCache()

		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), now)

		server.PrepareUsageDelay = time.Second * 3
		ok, err := executeRequest(ctx, r, key, "")
		assert.True(t, ok)
		assert.NoError(t, err)

		client.Stop(context.Background())
		usage, err := server.store.GetAccountUsage(ctx, project, proto.Ptr(service), now.Add(-time.Hour), now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

}

func TestDefaultKey(t *testing.T) {
	cfg := newConfig()
	server := newTestServer(t, &cfg)

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
	err := server.store.SetAccessLimit(ctx, project, &limit)
	require.NoError(t, err)
	err = server.store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: keys[0], ProjectID: project})
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
	err = server.store.InsertAccessKey(ctx, &newAccess)
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
	const secret = "secret"
	auth := jwtauth.New("HS256", []byte(secret), nil)

	project := uint64(7)
	account := "account"
	key := proto.GenerateAccessKey(project)
	service := proto.Service_Indexer

	counter := spendingCounter(0)

	cfg := newConfig()
	server := newTestServer(t, &cfg)
	client := newQuotaClient(cfg, service)

	r := chi.NewRouter()
	r.Use(
		jwtauth.Verifier(auth),
		middleware.Credentials(auth),
		middleware.VerifyQuota(client),
		addCredits(1).Middleware,
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
	server.store.SetAccessLimit(ctx, project, &limit)

	_, token, err := auth.Encode(map[string]any{"project": project, "account": account})
	require.NoError(t, err)

	var expectedHits int64

	t.Run("UnauthorizedUser", func(t *testing.T) {
		ok, err := executeRequest(ctx, r, "", token)
		require.ErrorIs(t, err, proto.ErrUnauthorizedUser)
		assert.False(t, ok)
	})
	server.store.SetUserPermission(ctx, project, account, proto.UserPermission_READ_WRITE, proto.ResourceAccess{ProjectID: project})
	t.Run("AuthorizedUser", func(t *testing.T) {
		ok, err := executeRequest(ctx, r, "", token)
		require.NoError(t, err)
		assert.True(t, ok)
		expectedHits++
	})
	t.Run("AccessKeyNotFound", func(t *testing.T) {
		ok, err := executeRequest(ctx, r, key, token)
		require.ErrorIs(t, err, proto.ErrAccessKeyNotFound)
		assert.False(t, ok)
	})
	server.store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: key, ProjectID: project})
	t.Run("AccessKeyFound", func(t *testing.T) {
		ok, err := executeRequest(ctx, r, key, token)
		require.NoError(t, err)
		assert.True(t, ok)
		expectedHits++
	})
	t.Run("AccessKeyMismatch", func(t *testing.T) {
		ok, err := executeRequest(ctx, r, proto.GenerateAccessKey(project+1), token)
		require.ErrorIs(t, err, proto.ErrAccessKeyMismatch)
		assert.False(t, ok)
	})

	assert.Equal(t, expectedHits, counter.GetValue())
}

func TestJWTAccess(t *testing.T) {
	const secret = "secret"
	auth := jwtauth.New("HS256", []byte(secret), nil)

	project := uint64(7)
	service := proto.Service_Indexer
	account := "account"

	counter := hitCounter(0)

	cfg := newConfig()
	server := newTestServer(t, &cfg)
	client := newQuotaClient(cfg, service)

	r := chi.NewRouter()
	r.Use(
		jwtauth.Verifier(auth),
		middleware.Credentials(auth),
		middleware.VerifyQuota(client),
		middleware.EnsurePermission(client, UserPermission_READ_WRITE),
	)
	r.Handle("/*", &counter)

	ctx := context.Background()

	server.store.SetAccessLimit(ctx, project, &proto.Limit{
		RateLimit: 100,
		FreeWarn:  5,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	})

	_, token, err := auth.Encode(map[string]any{
		"account": account,
		"project": project,
	})
	require.NoError(t, err)

	var expectedHits int64

	t.Run("NoPermission", func(t *testing.T) {
		ok, err := executeRequest(ctx, r, "", token)
		require.ErrorIs(t, err, proto.ErrUnauthorizedUser)
		assert.False(t, ok)
	})

	server.store.SetUserPermission(ctx, project, account, proto.UserPermission_READ, proto.ResourceAccess{ProjectID: project})
	t.Run("LowPermission", func(t *testing.T) {
		ok, err := executeRequest(ctx, r, "", token)
		require.ErrorIs(t, err, proto.ErrUnauthorizedUser)
		assert.False(t, ok)
	})

	server.store.SetUserPermission(ctx, project, account, proto.UserPermission_READ_WRITE, proto.ResourceAccess{ProjectID: project})
	server.FlushCache()
	t.Run("EnoughPermission", func(t *testing.T) {
		ok, err := executeRequest(ctx, r, "", token)
		require.NoError(t, err)
		assert.True(t, ok)
		expectedHits++
	})

	server.store.SetUserPermission(ctx, project, account, proto.UserPermission_ADMIN, proto.ResourceAccess{ProjectID: project})
	server.FlushCache()
	t.Run("MorePermission", func(t *testing.T) {
		ok, err := executeRequest(ctx, r, "", token)
		require.NoError(t, err)
		assert.True(t, ok)
		expectedHits++
	})

	assert.Equal(t, expectedHits, counter.GetValue())
}
