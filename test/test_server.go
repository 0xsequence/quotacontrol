package test

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/alicebob/miniredis/v2"
	"github.com/goware/logger"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func NewServer(t *testing.T, cfg *quotacontrol.Config) *TestServer {
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

	qc := TestServer{
		logger:        logger.NewLogger(logger.LogLevel_DEBUG),
		listener:      listener,
		cache:         client,
		Store:         store,
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
		PermissionStore: store,
	}

	logger := qc.logger.With(slog.String("server", "server"))
	qc.QuotaControl = quotacontrol.NewHandler(logger, qcCache, qcStore, nil)

	go func() {
		http.Serve(listener, proto.NewQuotaControlServer(&qc))
	}()

	return &qc
}

// TestServer is a wrapper of quotacontrol that tracks the events that are notified and allows to inject errors
type TestServer struct {
	logger   logger.Logger
	listener net.Listener
	cache    *redisclient.Client

	Store *quotacontrol.MemoryStore

	proto.QuotaControl

	mu            sync.Mutex
	notifications map[uint64][]proto.EventType

	ErrGetProjectQuota error
	ErrGetAccessQuota  error
	ErrPrepareUsage    error
	PrepareUsageDelay  time.Duration
}

func (qc *TestServer) FlushNotifications() {
	qc.mu.Lock()
	qc.notifications = make(map[uint64][]proto.EventType)
	qc.mu.Unlock()
}

func (qc *TestServer) FlushCache(ctx context.Context) {
	qc.cache.FlushAll(ctx)
}

func (qc *TestServer) GetProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	if qc.ErrGetProjectQuota != nil {
		return nil, qc.ErrGetProjectQuota
	}
	return qc.QuotaControl.GetProjectQuota(ctx, projectID, now)
}

func (qc *TestServer) GetAccessQuota(ctx context.Context, accessKey string, now time.Time) (*proto.AccessQuota, error) {
	if qc.ErrGetAccessQuota != nil {
		return nil, qc.ErrGetAccessQuota
	}
	return qc.QuotaControl.GetAccessQuota(ctx, accessKey, now)
}

func (qc *TestServer) PrepareUsage(ctx context.Context, projectID uint64, cycle *proto.Cycle, now time.Time) (bool, error) {
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

func (q *TestServer) GetEvents(projectID uint64) []proto.EventType {
	q.mu.Lock()
	v := q.notifications[projectID]
	q.mu.Unlock()
	return v
}

func (q *TestServer) NotifyEvent(ctx context.Context, projectID uint64, eventType proto.EventType) (bool, error) {
	q.mu.Lock()
	q.notifications[projectID] = append(q.notifications[projectID], eventType)
	q.mu.Unlock()
	return true, nil
}
