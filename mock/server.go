package mock

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
	redisclient "github.com/redis/go-redis/v9"
)

type Event struct {
	Service proto.Service
	Type    proto.EventType
}

func NewServer(cfg *quotacontrol.Config) (server *Server, cleanup func()) {
	s := miniredis.NewMiniRedis()
	s.Start()
	cfg.Redis.Host = s.Host()
	cfg.Redis.Port = uint16(s.Server().Addr().Port)
	client := redisclient.NewClient(&redisclient.Options{Addr: s.Addr()})

	store := NewMemoryStore()

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatal(err)
	}

	cfg.URL = "http://" + listener.Addr().String()

	qc := Server{
		logger:        slog.Default(),
		listener:      listener,
		cache:         client,
		Store:         store,
		notifications: make(map[uint64][]Event),
	}

	qcCache := quotacontrol.NewCache(client, time.Minute)
	qcStore := quotacontrol.Store{
		ProjectInfoStore: store,
		LimitStore:       store,
		AccessKeyStore:   store,
		UsageStore:       store,
		PermissionStore:  store,
	}

	logger := qc.logger.With(slog.Bool("mock", true))
	qc.QuotaControlServer = quotacontrol.NewServer(cfg.Redis, logger, qcCache, qcStore)

	go func() {
		logger.Info("server starting...", slog.String("url", cfg.URL))
		http.Serve(listener, proto.NewQuotaControlServer(&qc))
	}()

	return &qc, func() {
		s.Close()
		listener.Close()
		logger.Info("server stopped")
	}
}

// Server is a wrapper of quotacontrol that tracks the events that are notified and allows to inject errors
type Server struct {
	logger   *slog.Logger
	listener net.Listener
	cache    *redisclient.Client

	Store *MemoryStore

	proto.QuotaControlServer

	mu            sync.Mutex
	notifications map[uint64][]Event

	ErrGetProjectQuota error
	ErrGetAccessQuota  error
	PrepareUsageDelay  time.Duration
}

func (s *Server) FlushNotifications() {
	s.mu.Lock()
	s.notifications = make(map[uint64][]Event)
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
	return s.QuotaControlServer.GetProjectQuota(ctx, projectID, now)
}

// GetAccessQuota returns the quota for an access key unless ErrGetAccessQuota is set
func (s *Server) GetAccessQuota(ctx context.Context, accessKey string, now time.Time) (*proto.AccessQuota, error) {
	if s.ErrGetAccessQuota != nil {
		return nil, s.ErrGetAccessQuota
	}
	return s.QuotaControlServer.GetAccessQuota(ctx, accessKey, now)
}

// GetEvents returns the events that have been notified for a project
func (s *Server) GetEvents(projectID uint64) []Event {
	s.mu.Lock()
	v := s.notifications[projectID]
	s.mu.Unlock()
	return v
}

// NotifyEvent ovverides the default NotifyEvent method to track the events that are notified
func (s *Server) NotifyEvent(ctx context.Context, projectID uint64, service proto.Service, eventType proto.EventType) (bool, error) {
	s.mu.Lock()
	s.notifications[projectID] = append(s.notifications[projectID], Event{
		Service: service,
		Type:    eventType,
	})
	s.mu.Unlock()
	return s.QuotaControlServer.NotifyEvent(ctx, projectID, service, eventType)
}
