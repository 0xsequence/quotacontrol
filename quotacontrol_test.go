package quotacontrol_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	. "github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_ProjectID  = uint64(777)
	_AccessKeys = []string{proto.GenerateAccessKey(_ProjectID), proto.GenerateAccessKey(_ProjectID)}
	_Now        = time.Date(2023, time.June, 26, 0, 0, 0, 0, time.Local)
)

type addCredits int64

func (i addCredits) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(middleware.AddComputeUnits(r.Context(), int64(i))))
	})
}

func TestMiddlewareUseAccessKey(t *testing.T) {
	cfg := newConfig()
	qc := setupQuotaServer(t, &cfg)

	limit := proto.Limit{
		RateLimit: 100,
		FreeWarn:  5,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}

	ctx := context.Background()
	err := qc.store.SetAccessLimit(ctx, _ProjectID, &limit)
	require.NoError(t, err)
	err = qc.store.InsertAccessKey(ctx, &proto.AccessKey{Active: true, AccessKey: _AccessKeys[0], ProjectID: _ProjectID})
	require.NoError(t, err)

	client := newClient(cfg, proto.Service_Indexer)

	router := chi.NewRouter()

	// we set the compute units to 2, then in another handler we remove 1 before spending
	router.Use(addCredits(2).Middleware)
	router.Use(middleware.SetKey(nil))
	router.Use(middleware.VerifyQuota(client))
	router.Use(NewRateLimiter(cfg, httprate.KeyByRealIP))
	router.Use(addCredits(-1).Middleware)
	router.Use(middleware.SpendUsage(client))

	counter := spendingCounter(0)
	router.Handle("/*", &counter)

	expectedUsage := proto.AccessUsage{}

	t.Run("WithAccessKey", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), _Now)
		qc.notifications = make(map[uint64][]proto.EventType)

		// Spend Free CU
		for i := int64(1); i < limit.FreeWarn; i++ {
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Empty(t, qc.getEvents(_ProjectID), i)
			expectedUsage.Add(proto.AccessUsage{ValidCompute: 1})
		}

		// Go over free CU
		ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, qc.getEvents(_ProjectID), proto.EventType_FreeMax)
		expectedUsage.Add(proto.AccessUsage{ValidCompute: 1})

		// Get close to soft quota
		for i := limit.FreeWarn + 1; i < limit.OverWarn; i++ {
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Len(t, qc.getEvents(_ProjectID), 1)
			expectedUsage.Add(proto.AccessUsage{OverCompute: 1})
		}

		// Go over soft quota
		ok, err = executeRequest(ctx, router, _AccessKeys[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, qc.getEvents(_ProjectID), proto.EventType_OverWarn)
		expectedUsage.Add(proto.AccessUsage{OverCompute: 1})

		// Get close to hard quota
		for i := limit.OverWarn + 1; i < limit.OverMax; i++ {
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Len(t, qc.getEvents(_ProjectID), 2)
			expectedUsage.Add(proto.AccessUsage{OverCompute: 1})
		}

		// Go over hard quota
		ok, err = executeRequest(ctx, router, _AccessKeys[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, qc.getEvents(_ProjectID), proto.EventType_OverMax)
		expectedUsage.Add(proto.AccessUsage{OverCompute: 1})

		// Denied
		for i := 0; i < 10; i++ {
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.ErrorIs(t, err, proto.ErrLimitExceeded)
			assert.False(t, ok)
			expectedUsage.Add(proto.AccessUsage{LimitedCompute: 1})
		}

		// check the usage
		client.Stop(context.Background())
		usage, err := qc.store.GetAccountUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ChangeLimits", func(t *testing.T) {
		// Change limits
		//
		// Increase CreditsOverageLimit which should still allow requests to go through, etc.
		err = qc.store.SetAccessLimit(ctx, _ProjectID, &proto.Limit{
			RateLimit: 100,
			OverWarn:  5,
			OverMax:   110,
		})
		assert.NoError(t, err)
		err = client.ClearQuotaCacheByAccessKey(ctx, _AccessKeys[0])
		assert.NoError(t, err)

		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), _Now)
		qc.notifications = make(map[uint64][]proto.EventType)

		ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)

		client.Stop(context.Background())
		usage, err := qc.store.GetAccountUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		expectedUsage.Add(proto.AccessUsage{ValidCompute: 0, OverCompute: 1, LimitedCompute: 0})
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("PublicRateLimit", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), _Now)

		for i, max := 0, cfg.RateLimiter.DefaultRPM*2; i < max; i++ {
			ok, err := executeRequest(ctx, router, "", "")
			if i < cfg.RateLimiter.DefaultRPM {
				assert.NoError(t, err, i)
				assert.True(t, ok, i)
			} else {
				assert.ErrorIs(t, err, proto.ErrLimitExceeded, i)
				assert.False(t, ok, i)
			}
		}

		client.Stop(context.Background())
		usage, err := qc.store.GetAccountUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ServerErrors", func(t *testing.T) {
		qc.FlushCache()

		go client.Run(context.Background())

		errList := []error{
			errors.New("unexpected error"),
			proto.ErrWebrpcBadRoute,
			proto.ErrTimeout,
		}

		ctx := middleware.WithTime(context.Background(), _Now)

		for _, err := range errList {
			qc.ErrGetAccessQuota = err
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.True(t, ok)
			assert.NoError(t, err)
		}
		qc.ErrGetAccessQuota = nil

		qc.FlushCache()

		for _, err := range errList {
			qc.ErrPrepareUsage = err
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.True(t, ok)
			assert.NoError(t, err)
		}
		qc.ErrPrepareUsage = nil

		client.Stop(context.Background())
		usage, err := qc.store.GetAccountUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

	t.Run("ServerTimeout", func(t *testing.T) {
		qc.FlushCache()

		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), _Now)

		qc.PrepareUsageDelay = time.Second * 3
		ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
		assert.True(t, ok)
		assert.NoError(t, err)

		client.Stop(context.Background())
		usage, err := qc.store.GetAccountUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedUsage.GetTotalUsage()), counter.GetValue())
		assert.Equal(t, &expectedUsage, &usage)
	})

}

func TestDefaultKey(t *testing.T) {
	cfg := newConfig()
	qc := setupQuotaServer(t, &cfg)

	service := proto.Ptr(proto.Service_Metadata)
	limit := proto.Limit{
		RateLimit: 100,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}
	access := &proto.AccessKey{Active: true, AccessKey: _AccessKeys[0], ProjectID: _ProjectID}

	// populate store
	ctx := context.Background()
	err := qc.store.SetAccessLimit(ctx, _ProjectID, &limit)
	require.NoError(t, err)
	err = qc.store.InsertAccessKey(ctx, access)
	require.NoError(t, err)

	client := newClient(cfg, *service)

	aq, err := client.FetchKeyQuota(ctx, _AccessKeys[0], "", _Now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	aq, err = client.FetchKeyQuota(ctx, _AccessKeys[0], "", _Now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	access, err = qc.UpdateAccessKey(ctx, _AccessKeys[0], proto.Ptr("new name"), nil, []*proto.Service{service})
	require.NoError(t, err)

	aq, err = client.FetchKeyQuota(ctx, _AccessKeys[0], "", _Now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	ok, err := qc.DisableAccessKey(ctx, _AccessKeys[0])
	require.ErrorIs(t, err, proto.ErrAtLeastOneKey)
	assert.False(t, ok)
	newAccess := proto.AccessKey{Active: true, AccessKey: _AccessKeys[1], ProjectID: _ProjectID}
	err = qc.store.InsertAccessKey(ctx, &newAccess)
	require.NoError(t, err)

	ok, err = qc.DisableAccessKey(ctx, _AccessKeys[0])
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = client.FetchKeyQuota(ctx, _AccessKeys[0], "", _Now)
	require.ErrorIs(t, err, proto.ErrAccessKeyNotFound)

	newAccess.Default = true
	aq, err = client.FetchKeyQuota(ctx, newAccess.AccessKey, "", _Now)
	require.NoError(t, err)
	assert.Equal(t, &newAccess, aq.AccessKey)
}

func TestJWT(t *testing.T) {
	const secret = "secret"
	auth := jwtauth.New("HS256", []byte(secret), nil)

	project := uint64(7)
	accessKey := proto.GenerateAccessKey(project)
	counter := spendingCounter(0)

	cfg := newConfig()
	server := setupQuotaServer(t, &cfg)
	client := newClient(cfg, proto.Service_Indexer)

	_ = server

	r := chi.NewRouter()
	r.Use(jwtauth.Verifier(auth), middleware.SetKey(auth), middleware.VerifyQuota(client))
	r.Handle("/*", &counter)

	_, token, err := auth.Encode(map[string]any{"project": project})
	require.NoError(t, err)

	ctx := context.Background()
	ok, err := executeRequest(ctx, r, accessKey, token)
	require.ErrorIs(t, err, proto.ErrAccessKeyNotFound)
	assert.False(t, ok)

}
