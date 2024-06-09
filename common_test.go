package quotacontrol_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/alicebob/miniredis/v2"
	"github.com/goware/logger"

	"github.com/goware/cachestore/redis"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newConfig() quotacontrol.Config {
	return quotacontrol.Config{
		Enabled:    true,
		UpdateFreq: quotacontrol.Duration{time.Minute},
		Redis: redis.Config{
			Enabled: true,
		},
		RateLimiter: quotacontrol.RateLimiterConfig{
			Enabled:    true,
			DefaultRPM: 10,
		},
	}
}

func newQuotaClient(cfg quotacontrol.Config, service proto.Service) *quotacontrol.Client {
	logger := logger.NewLogger(logger.LogLevel_DEBUG).With(slog.String("client", "client"))
	return quotacontrol.NewClient(logger, service, cfg)
}

func newTestServer(t *testing.T, cfg *quotacontrol.Config) *testServer {
	s := miniredis.NewMiniRedis()
	s.Start()
	t.Cleanup(s.Close)
	cfg.Redis.Host = s.Host()
	cfg.Redis.Port = uint16(s.Server().Addr().Port)
	client := redisclient.NewClient(&redisclient.Options{Addr: s.Addr()})

	store := quotacontrol.NewMemoryStore()

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	cfg.URL = "http://" + listener.Addr().String()

	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	qc := testServer{
		logger:        logger.NewLogger(logger.LogLevel_DEBUG),
		listener:      listener,
		cache:         client,
		store:         store,
		notifications: make(map[uint64][]proto.EventType),
	}

	qcCache := quotacontrol.Cache{
		QuotaCache:      quotacontrol.NewRedisCache(client, time.Minute),
		UsageCache:      quotacontrol.NewRedisCache(client, time.Minute),
		PermissionCache: quotacontrol.NewRedisCache(client, time.Minute),
	}
	qcStore := quotacontrol.Store{
		LimitStore:      store,
		AccessKeyStore:  store,
		UsageStore:      store,
		CycleStore:      store,
		PermissionStore: nil,
	}

	logger := qc.logger.With(slog.String("server", "server"))
	qc.QuotaControl = quotacontrol.NewHandler(logger, qcCache, qcStore, nil)

	go func() {
		http.Serve(listener, proto.NewQuotaControlServer(&qc))
	}()

	return &qc
}

// testServer is a wrapper of quotacontrol that tracks the events that are notified and allows to inject errors
type testServer struct {
	logger   logger.Logger
	listener net.Listener
	cache    *redisclient.Client
	store    *quotacontrol.MemoryStore

	proto.QuotaControl

	sync.Mutex
	notifications map[uint64][]proto.EventType

	ErrGetProjectQuota error
	ErrGetAccessQuota  error
	ErrPrepareUsage    error
	PrepareUsageDelay  time.Duration
}

func (qc *testServer) FlushCache() {
	qc.cache.FlushAll(context.Background())
}

func (qc *testServer) GetProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	if qc.ErrGetProjectQuota != nil {
		return nil, qc.ErrGetProjectQuota
	}
	return qc.QuotaControl.GetProjectQuota(ctx, projectID, now)
}

func (qc *testServer) GetAccessQuota(ctx context.Context, accessKey string, now time.Time) (*proto.AccessQuota, error) {
	if qc.ErrGetAccessQuota != nil {
		return nil, qc.ErrGetAccessQuota
	}
	return qc.QuotaControl.GetAccessQuota(ctx, accessKey, now)
}

func (qc *testServer) PrepareUsage(ctx context.Context, projectID uint64, cycle *proto.Cycle, now time.Time) (bool, error) {
	if qc.ErrPrepareUsage != nil {
		return false, qc.ErrPrepareUsage
	}
	if qc.PrepareUsageDelay > 0 {
		go func() {
			time.Sleep(qc.PrepareUsageDelay)
			qc.ClearUsage(ctx, projectID, now)
		}()
		return true, nil
	}
	return qc.QuotaControl.PrepareUsage(ctx, projectID, cycle, now)
}

func (q *testServer) getEvents(projectID uint64) []proto.EventType {
	q.Lock()
	v := q.notifications[projectID]
	q.Unlock()
	return v
}

func (q *testServer) NotifyEvent(ctx context.Context, projectID uint64, eventType *proto.EventType) (bool, error) {
	q.Lock()
	q.notifications[projectID] = append(q.notifications[projectID], *eventType)
	q.Unlock()
	return true, nil
}

type spendingCounter int64

func (c *spendingCounter) GetValue() int64 {
	return atomic.LoadInt64((*int64)(c))
}

func (c *spendingCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// up the counter only if quota control run
	if middleware.HasSpending(r.Context()) {
		atomic.AddInt64((*int64)(c), 1)
	}
	w.WriteHeader(http.StatusOK)
}

func executeRequest(ctx context.Context, handler http.Handler, accessKey, jwt string) (bool, error) {
	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Real-IP", "127.0.0.1")
	if accessKey != "" {
		req.Header.Set(middleware.HeaderAccessKey, accessKey)
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Result().StatusCode; status < http.StatusOK || status >= http.StatusBadRequest {
		w := proto.WebRPCError{}
		json.Unmarshal(rr.Body.Bytes(), &w)
		return false, w
	}

	return true, nil
}

type addCredits int64

func (i addCredits) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(middleware.AddComputeUnits(r.Context(), int64(i))))
	})
}
