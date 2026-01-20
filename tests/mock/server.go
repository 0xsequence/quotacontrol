package mock

import (
	"cmp"
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type Event struct {
	Service proto.Service
	Type    proto.EventType
}

type Options struct {
	RedisClient *redis.Client
	Logger      *slog.Logger
}

func NewServer(cfg *quotacontrol.Config, options *Options) (server *Server, cleanup func()) {
	var client *redis.Client
	if options != nil {
		client = options.RedisClient
	}
	if client == nil {
		s := miniredis.NewMiniRedis()
		s.Start()
		cleanup = s.Close
		cfg.Redis.Host = s.Host()
		cfg.Redis.Port = uint16(s.Server().Addr().Port)
		client = redis.NewClient(&redis.Options{Addr: s.Addr()})
	}

	store := NewMemoryStore()

	listener, err := net.Listen("tcp", cmp.Or(strings.TrimPrefix(cfg.URL, "http://"), "localhost:0"))
	if err != nil {
		log.Fatal(err)
	}

	cfg.URL = "http://" + listener.Addr().String()

	logger := slog.Default()
	if options.Logger != nil {
		logger = options.Logger
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

	server = &Server{
		logger:             logger,
		QuotaControlServer: quotacontrol.NewServer(cfg.Redis, logger, qcCache, qcStore),
		listener:           listener,
		cache:              client,
		Store:              store,
		notifications:      make(map[uint64][]Event),
	}

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/project/{project_id}/limit", server.HandleSetLimit)
		mux.Handle("/", proto.NewQuotaControlServer(server))
		http.Serve(listener, mux)
	}()

	fn := cleanup
	return server, func() {
		if fn != nil {
			fn()
		}
		listener.Close()
	}
}

// Server is a wrapper of quotacontrol that tracks the events that are notified and allows to inject errors
type Server struct {
	logger   *slog.Logger
	listener net.Listener
	cache    *redis.Client

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

func (s *Server) HandleSetLimit(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseUint(r.PathValue("project_id"), 10, 64)
	if err != nil {
		proto.RespondWithError(w, proto.ErrWebrpcBadRequest.WithCausef("invalid project ID: %v", err))
		return
	}

	var limit proto.Limit
	if err := json.NewDecoder(r.Body).Decode(&limit); err != nil {
		proto.RespondWithError(w, proto.ErrWebrpcBadRequest.WithCausef("invalid request body: %v", err))
		return
	}

	if err := s.Store.SetAccessLimit(r.Context(), projectID, &limit); err != nil {
		proto.RespondWithError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	respBody, _ := json.Marshal(map[string]interface{}{"ok": true})
	w.Write(respBody)
}
