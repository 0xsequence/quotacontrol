package quotacontrol_test

import (
	"context"
	"encoding/json"
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
	"github.com/goware/logger"

	"github.com/goware/cachestore/redis"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newConfig() Config {
	return Config{
		Enabled:    true,
		UpdateFreq: Duration{time.Minute},
		Redis: redis.Config{
			Enabled: true,
		},
		RateLimiter: RateLimiterConfig{
			Enabled:    true,
			DefaultRPM: 10,
		},
	}
}

func newClient(cfg Config, service proto.Service) *Client {
	return NewClient(logger.NewLogger(logger.LogLevel_DEBUG).With("client", "client"), service, cfg)
}

// qcTest is a wrapper of quotacontrol that tracks the events that are notified and allows to inject errors
type qcTest struct {
	logger   logger.Logger
	listener net.Listener
	cache    *redisclient.Client
	store    *MemoryStore

	proto.QuotaControl

	sync.Mutex
	notifications map[uint64][]proto.EventType

	ErrGetProjectQuota error
	ErrGetAccessQuota  error
	ErrPrepareUsage    error
	PrepareUsageDelay  time.Duration
}

func (qc *qcTest) FlushCache() {
	qc.cache.FlushAll(context.Background())
}

func (qc *qcTest) GetProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	if qc.ErrGetProjectQuota != nil {
		return nil, qc.ErrGetProjectQuota
	}
	return qc.QuotaControl.GetProjectQuota(ctx, projectID, now)
}

func (qc *qcTest) GetAccessQuota(ctx context.Context, accessKey string, now time.Time) (*proto.AccessQuota, error) {
	if qc.ErrGetAccessQuota != nil {
		return nil, qc.ErrGetAccessQuota
	}
	return qc.QuotaControl.GetAccessQuota(ctx, accessKey, now)
}

func (qc *qcTest) PrepareUsage(ctx context.Context, projectID uint64, cycle *proto.Cycle, now time.Time) (bool, error) {
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

func (q *qcTest) getEvents(projectID uint64) []proto.EventType {
	q.Lock()
	v := q.notifications[projectID]
	q.Unlock()
	return v
}

func (q *qcTest) NotifyEvent(ctx context.Context, projectID uint64, eventType *proto.EventType) (bool, error) {
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

func setupQuotaServer(t *testing.T, cfg *Config) *qcTest {
	s := miniredis.NewMiniRedis()
	s.Start()
	t.Cleanup(s.Close)
	cfg.Redis.Host = s.Host()
	cfg.Redis.Port = uint16(s.Server().Addr().Port)
	client := redisclient.NewClient(&redisclient.Options{Addr: s.Addr()})

	store := NewMemoryStore()

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	qc := qcTest{
		logger:        logger.NewLogger(logger.LogLevel_DEBUG),
		listener:      listener,
		cache:         client,
		store:         store,
		notifications: make(map[uint64][]proto.EventType),
	}

	qcCache := Cache{
		QuotaCache:      NewRedisCache(client, time.Minute),
		UsageCache:      NewRedisCache(client, time.Minute),
		PermissionCache: NewRedisCache(client, time.Minute),
	}
	qcStore := Store{
		LimitStore:      store,
		AccessKeyStore:  store,
		UsageStore:      store,
		CycleStore:      store,
		PermissionStore: nil,
	}

	qc.QuotaControl = NewQuotaControlHandler(qc.logger.With("server", "server"), qcCache, qcStore, nil)
	cfg.URL = "http://" + listener.Addr().String()
	go func() {
		http.Serve(listener, proto.NewQuotaControlServer(&qc))
	}()

	return &qc
}
