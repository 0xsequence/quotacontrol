package test

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/alicebob/miniredis/v2"
	"github.com/goware/logger"
	redisclient "github.com/redis/go-redis/v9"
)

func NewServer(cfg *quotacontrol.Config) (server *Server, cleanup func()) {
	s := miniredis.NewMiniRedis()
	s.Start()
	cfg.Redis.Host = s.Host()
	cfg.Redis.Port = uint16(s.Server().Addr().Port)
	client := redisclient.NewClient(&redisclient.Options{Addr: s.Addr()})

	store := quotacontrol.NewMemoryStore()

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatal(err)
	}

	cfg.URL = "http://" + listener.Addr().String()

	qc := Server{
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

	return &qc, func() {
		s.Close()
		listener.Close()
	}
}

// Server is a wrapper of quotacontrol that tracks the events that are notified and allows to inject errors
type Server struct {
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

func (s *Server) FlushNotifications() {
	s.mu.Lock()
	s.notifications = make(map[uint64][]proto.EventType)
	s.mu.Unlock()
}

func (s *Server) FlushCache(ctx context.Context) {
	s.cache.FlushAll(ctx)
}

// GetProjectQuota returns the quota for a project unless ErrGetProjectQuota is set
func (s *Server) GetProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	if s.ErrGetProjectQuota != nil {
		return nil, s.ErrGetProjectQuota
	}
	return s.QuotaControl.GetProjectQuota(ctx, projectID, now)
}

// GetAccessQuota returns the quota for an access key unless ErrGetAccessQuota is set
func (s *Server) GetAccessQuota(ctx context.Context, accessKey string, now time.Time) (*proto.AccessQuota, error) {
	if s.ErrGetAccessQuota != nil {
		return nil, s.ErrGetAccessQuota
	}
	return s.QuotaControl.GetAccessQuota(ctx, accessKey, now)
}

// PrepareUsage prepares the usage for a project unless ErrPrepareUsage is set.
// If PrepareUsageDelay is set, it will clear the usage after that delay
func (s *Server) PrepareUsage(ctx context.Context, projectID uint64, cycle *proto.Cycle, now time.Time) (bool, error) {
	if s.ErrPrepareUsage != nil {
		return false, s.ErrPrepareUsage
	}
	if s.PrepareUsageDelay > 0 {
		go func() {
			time.Sleep(s.PrepareUsageDelay)
			s.ClearUsage(ctx, projectID, now)
		}()
		return true, nil
	}
	return s.QuotaControl.PrepareUsage(ctx, projectID, cycle, now)
}

// GetEvents returns the events that have been notified for a project
func (s *Server) GetEvents(projectID uint64) []proto.EventType {
	s.mu.Lock()
	v := s.notifications[projectID]
	s.mu.Unlock()
	return v
}

// NotifyEvent ovverides the default NotifyEvent method to track the events that are notified
func (s *Server) NotifyEvent(ctx context.Context, projectID uint64, eventType proto.EventType) (bool, error) {
	s.mu.Lock()
	s.notifications[projectID] = append(s.notifications[projectID], eventType)
	s.mu.Unlock()
	return s.QuotaControl.NotifyEvent(ctx, projectID, eventType)
}
