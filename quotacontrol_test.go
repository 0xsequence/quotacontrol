package quotacontrol_test

import (
	"context"
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
	_Host       = "localhost:8080"
	_ProjectID  = uint64(777)
	_AccessKeys = []string{"abc", "cde"}
	_Now        = time.Date(2023, time.June, 26, 0, 0, 0, 0, time.Local)

	cfg = Config{
		Enabled:    true,
		URL:        `http://` + _Host,
		UpdateFreq: Duration{time.Minute},
		RateLimiter: RateLimiterConfig{
			Enabled:                 true,
			PublicRequestsPerMinute: 10,
		},
		LRUSize: 100,
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

func TestMiddlewareUseAccessKey(t *testing.T) {
	limit := proto.Limit{RateLimit: 100, FreeCU: 5, SoftQuota: 7, HardQuota: 10}
	access := proto.AccessKey{Active: true, AccessKey: _AccessKeys[0], ProjectID: _ProjectID}
	expectedCounter := proto.AccessUsage{}
	wlog := logger.NewLogger(logger.LogLevel_DEBUG)

	s := miniredis.NewMiniRedis()
	s.Start()
	t.Cleanup(s.Close)

	cfg.Redis.Host = s.Host()
	cfg.Redis.Port = uint16(s.Server().Addr().Port)

	redisClient := redisclient.NewClient(&redisclient.Options{Addr: s.Addr()})
	cache := NewRedisCache(redisClient, time.Minute)
	store := NewMemoryStore()

	// populate store
	ctx := context.Background()
	err := store.SetAccessLimit(ctx, _ProjectID, &limit)
	require.NoError(t, err)
	err = store.InsertAccessKey(ctx, &access)
	require.NoError(t, err)

	client := NewClient(wlog.With("client", "client"), proto.Service_Indexer, cfg)

	qc := quotaControl{
		QuotaControl:  NewQuotaControlHandler(wlog.With("server", "server"), cache, cache, store, store, store, nil),
		notifications: make(map[uint64][]proto.EventType),
	}
	server := http.Server{
		Addr:    _Host,
		Handler: proto.NewQuotaControlServer(&qc),
	}
	go func() { require.ErrorIs(t, server.ListenAndServe(), http.ErrServerClosed) }()
	defer server.Close()
	time.Sleep(1 * time.Second)

	router := chi.NewRouter()

	// we set the compute units to 2, then in another handler we remove 1 before spending
	router.Use(middlewareCU(2))
	router.Use(middleware.SetAccessKey)
	router.Use(middleware.VerifyAccessKey(client, nil))
	router.Use(NewPublicRateLimiter(cfg))
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
		time.Sleep(1 * time.Second)

		ctx := middleware.WithTime(context.Background(), _Now)
		qc.notifications = make(map[uint64][]proto.EventType)
		qc.notifications = make(map[uint64][]proto.EventType)

		// Spend Free CU
		for i := int64(1); i < limit.FreeCU; i++ {
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
		assert.Contains(t, qc.getEvents(_ProjectID), proto.EventType_FreeCU)
		expectedCounter.Add(proto.AccessUsage{ValidCompute: 1})

		// Get close to soft quota
		for i := limit.FreeCU + 1; i < limit.SoftQuota; i++ {
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
		assert.Contains(t, qc.getEvents(_ProjectID), proto.EventType_SoftQuota)
		expectedCounter.Add(proto.AccessUsage{OverCompute: 1})

		// Get close to hard quota
		for i := limit.SoftQuota + 1; i < limit.HardQuota; i++ {
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
		assert.Contains(t, qc.getEvents(_ProjectID), proto.EventType_HardQuota)
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
		// Increase HardQuota which should still allow requests to go through, etc.
		err = store.SetAccessLimit(ctx, _ProjectID, &proto.Limit{RateLimit: 100, SoftQuota: 5, HardQuota: 110})
		assert.NoError(t, err)
		err = client.ClearQuotaCacheByAccessKey(ctx, _AccessKeys[0])
		assert.NoError(t, err)

		go client.Run(context.Background())
		time.Sleep(1 * time.Second)

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
		time.Sleep(1 * time.Second)

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
