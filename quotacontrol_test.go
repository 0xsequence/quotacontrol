package quotacontrol_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/goware/logger"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_ProjectID  = uint64(777)
	_AccessKeys = []string{GenerateAccessKey(_ProjectID), GenerateAccessKey(_ProjectID)}
	_Now        = time.Date(2023, time.June, 26, 0, 0, 0, 0, time.Local)

	_Config = Config{
		Enabled:    true,
		UpdateFreq: Duration{time.Minute},
		RateLimiter: RateLimiterConfig{
			Enabled:                 true,
			PublicRequestsPerMinute: 10,
		},
	}
)

func middlewareCU(i int64) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = middleware.AddComputeUnits(ctx, i)
			h.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SetupRedis(t *testing.T, cfg *Config) *redisclient.Client {
	s := miniredis.NewMiniRedis()
	s.Start()
	t.Cleanup(s.Close)
	cfg.Redis.Host = s.Host()
	cfg.Redis.Port = uint16(s.Server().Addr().Port)
	return redisclient.NewClient(&redisclient.Options{Addr: s.Addr()})
}

func setupListener(t *testing.T) net.Listener {
	var err error
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	return l
}

func TestMiddlewareUseAccessKey(t *testing.T) {
	limit := proto.Limit{
		RateLimit: 100,
		FreeWarn:  5,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}
	access := proto.AccessKey{Active: true, AccessKey: _AccessKeys[0], ProjectID: _ProjectID}
	expectedCounter := proto.AccessUsage{}
	wlog := logger.NewLogger(logger.LogLevel_DEBUG)

	cfg := _Config
	cfg.LRUSize = 100
	redisClient := SetupRedis(t, &cfg)

	cache := NewRedisCache(redisClient, time.Minute)
	store := NewMemoryStore()

	// populate store
	ctx := context.Background()
	err := store.SetAccessLimit(ctx, _ProjectID, &limit)
	require.NoError(t, err)
	err = store.InsertAccessKey(ctx, &access)
	require.NoError(t, err)

	qcCache := Cache{
		QuotaCache:      cache,
		UsageCache:      cache,
		PermissionCache: cache,
	}
	qcStore := Store{
		LimitStore:      store,
		AccessKeyStore:  store,
		UsageStore:      store,
		CycleStore:      store,
		PermissionStore: nil,
	}

	qc := quotaControl{
		QuotaControl:  NewQuotaControlHandler(wlog.With("server", "server"), qcCache, qcStore, nil),
		notifications: make(map[uint64][]proto.EventType),
	}

	listener := setupListener(t)
	defer listener.Close()

	cfg.URL = "http://" + listener.Addr().String()
	go func() {
		http.Serve(listener, proto.NewQuotaControlServer(&qc))
	}()

	client := NewClient(wlog.With("client", "client"), proto.Service_Indexer, cfg)

	router := chi.NewRouter()

	// we set the compute units to 2, then in another handler we remove 1 before spending
	router.Use(middlewareCU(2))
	router.Use(middleware.SetAccessKey)
	router.Use(middleware.VerifyAccessKey(client, nil))
	router.Use(NewHTTPRateLimiter(cfg, nil))
	router.Use(middlewareCU(-1))
	router.Use(middleware.SpendUsage(client, nil))

	var counter int64
	router.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		// up the counter only if quota control run
		if middleware.GetResult(r.Context()) {
			atomic.AddInt64(&counter, 1)
		}
		w.WriteHeader(http.StatusOK)
	})

	t.Run("WithAccessKey", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), _Now)
		qc.notifications = make(map[uint64][]proto.EventType)
		qc.notifications = make(map[uint64][]proto.EventType)

		// Spend Free CU
		for i := int64(1); i < limit.FreeWarn; i++ {
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Empty(t, qc.getEvents(_ProjectID), i)
			expectedCounter.Add(proto.AccessUsage{ValidCompute: 1})
		}

		// Go over free CU
		ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, qc.getEvents(_ProjectID), proto.EventType_FreeMax)
		expectedCounter.Add(proto.AccessUsage{ValidCompute: 1})

		// Get close to soft quota
		for i := limit.FreeWarn + 1; i < limit.OverWarn; i++ {
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Len(t, qc.getEvents(_ProjectID), 1)
			expectedCounter.Add(proto.AccessUsage{OverCompute: 1})
		}

		// Go over soft quota
		ok, err = executeRequest(ctx, router, _AccessKeys[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, qc.getEvents(_ProjectID), proto.EventType_OverWarn)
		expectedCounter.Add(proto.AccessUsage{OverCompute: 1})

		// Get close to hard quota
		for i := limit.OverWarn + 1; i < limit.OverMax; i++ {
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Len(t, qc.getEvents(_ProjectID), 2)
			expectedCounter.Add(proto.AccessUsage{OverCompute: 1})
		}

		// Go over hard quota
		ok, err = executeRequest(ctx, router, _AccessKeys[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, qc.getEvents(_ProjectID), proto.EventType_OverMax)
		expectedCounter.Add(proto.AccessUsage{OverCompute: 1})

		// Denied
		for i := 0; i < 10; i++ {
			ok, err := executeRequest(ctx, router, _AccessKeys[0], "")
			assert.ErrorIs(t, err, proto.ErrLimitExceeded)
			assert.False(t, ok)
			expectedCounter.Add(proto.AccessUsage{LimitedCompute: 1})
		}

		// check the usage
		client.Stop(context.Background())
		usage, err := store.GetAccountUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedCounter.ValidCompute+expectedCounter.OverCompute), atomic.LoadInt64(&counter))
		assert.Equal(t, &expectedCounter, &usage)
	})

	t.Run("ChangeLimits", func(t *testing.T) {
		// Change limits
		//
		// Increase CreditsOverageLimit which should still allow requests to go through, etc.
		err = store.SetAccessLimit(ctx, _ProjectID, &proto.Limit{
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
		usage, err := store.GetAccountUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		expectedCounter.Add(proto.AccessUsage{ValidCompute: 0, OverCompute: 1, LimitedCompute: 0})
		assert.Equal(t, int64(expectedCounter.ValidCompute+expectedCounter.OverCompute), atomic.LoadInt64(&counter))
		assert.Equal(t, &expectedCounter, &usage)
	})

	t.Run("PublicRateLimit", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), _Now)

		for i, max := 0, cfg.RateLimiter.PublicRequestsPerMinute*2; i < max; i++ {
			ok, err := executeRequest(ctx, router, "", "")
			if i < cfg.RateLimiter.PublicRequestsPerMinute {
				assert.NoError(t, err, i)
				assert.True(t, ok, i)
			} else {
				assert.ErrorIs(t, err, proto.ErrLimitExceeded, i)
				assert.False(t, ok, i)
			}
		}

		client.Stop(context.Background())
		usage, err := store.GetAccountUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedCounter.ValidCompute+expectedCounter.OverCompute), atomic.LoadInt64(&counter))
		assert.Equal(t, &expectedCounter, &usage)
	})

}

func executeRequest(ctx context.Context, handler http.Handler, accessKey, origin string) (bool, error) {
	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Real-IP", "127.0.0.1")
	if accessKey != "" {
		req.Header.Set(middleware.HeaderAccessKey, accessKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	status := rr.Result().StatusCode
	if status < http.StatusOK || status >= http.StatusBadRequest {
		return false, proto.ErrLimitExceeded
	}
	return true, nil
}

// quotaControl is a wrapper of quotacontrol
type quotaControl struct {
	proto.QuotaControl
	sync.Mutex
	notifications map[uint64][]proto.EventType
}

func (q *quotaControl) getEvents(projectID uint64) []proto.EventType {
	q.Lock()
	v := q.notifications[projectID]
	q.Unlock()
	return v
}

func (q *quotaControl) NotifyEvent(ctx context.Context, projectID uint64, eventType *proto.EventType) (bool, error) {
	q.Lock()
	q.notifications[projectID] = append(q.notifications[projectID], *eventType)
	q.Unlock()

	return true, nil
}

func TestDefaultKey(t *testing.T) {
	service := proto.Ptr(proto.Service_Metadata)

	limit := proto.Limit{
		RateLimit: 100,
		FreeMax:   5,
		OverWarn:  7,
		OverMax:   10,
	}
	access := &proto.AccessKey{Active: true, AccessKey: _AccessKeys[0], ProjectID: _ProjectID}

	wlog := logger.NewLogger(logger.LogLevel_DEBUG)

	s := miniredis.NewMiniRedis()
	s.Start()
	t.Cleanup(s.Close)

	cfg := _Config

	redisClient := SetupRedis(t, &cfg)

	cache := NewRedisCache(redisClient, time.Minute)
	store := NewMemoryStore()

	// populate store
	ctx := context.Background()
	err := store.SetAccessLimit(ctx, _ProjectID, &limit)
	require.NoError(t, err)
	err = store.InsertAccessKey(ctx, access)
	require.NoError(t, err)

	qcCache := Cache{
		QuotaCache:      cache,
		UsageCache:      cache,
		PermissionCache: cache,
	}
	qcStore := Store{
		LimitStore:      store,
		AccessKeyStore:  store,
		UsageStore:      store,
		CycleStore:      store,
		PermissionStore: nil,
	}

	qc := quotaControl{
		QuotaControl:  NewQuotaControlHandler(wlog.With("server", "server"), qcCache, qcStore, nil),
		notifications: make(map[uint64][]proto.EventType),
	}

	listener := setupListener(t)
	defer listener.Close()

	cfg.URL = "http://" + listener.Addr().String()
	go func() {
		http.Serve(listener, proto.NewQuotaControlServer(&qc))
	}()

	client := NewClient(wlog.With("client", "client"), *service, cfg)

	aq, err := client.FetchKeyQuota(ctx, access.AccessKey, "", _Now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	aq, err = client.FetchKeyQuota(ctx, access.AccessKey, "", _Now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	access, err = qc.UpdateAccessKey(ctx, access.AccessKey, proto.Ptr("new name"), nil, []*proto.Service{service})
	require.NoError(t, err)

	aq, err = client.FetchKeyQuota(ctx, access.AccessKey, "", _Now)
	require.NoError(t, err)
	assert.Equal(t, access, aq.AccessKey)
	assert.Equal(t, &limit, aq.Limit)

	ok, err := qc.DisableAccessKey(ctx, access.AccessKey)
	require.ErrorIs(t, err, proto.ErrAtLeastOneKey)
	assert.False(t, ok)
	newAccess := proto.AccessKey{Active: true, AccessKey: _AccessKeys[1], ProjectID: _ProjectID}
	err = store.InsertAccessKey(ctx, &newAccess)
	require.NoError(t, err)

	ok, err = qc.DisableAccessKey(ctx, access.AccessKey)
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = client.FetchKeyQuota(ctx, access.AccessKey, "", _Now)
	require.ErrorIs(t, err, proto.ErrAccessKeyNotFound)

	newAccess.Default = true
	aq, err = client.FetchKeyQuota(ctx, newAccess.AccessKey, "", _Now)
	require.NoError(t, err)
	assert.Equal(t, &newAccess, aq.AccessKey)
}
