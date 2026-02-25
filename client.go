package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/0xsequence/go-libs/xlog"
	"github.com/0xsequence/quotacontrol/cache"
	"github.com/0xsequence/quotacontrol/internal/usage"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/redis/go-redis/v9"

	"github.com/0xsequence/authcontrol"
	authproto "github.com/0xsequence/authcontrol/proto"
)

type Notifier interface {
	Notify(access *proto.AccessKey) error
}

// NewClient creates a new quota control client.
// - logger can't be nil.
// - service is the service name.
// - cfg is the configuration.
// - if qc is not nil, it will be used instead of the proto client.
func NewClient(log *slog.Logger, service proto.Service, cfg Config, qc proto.QuotaControlClient) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		DB:           cfg.Redis.DBIndex,
		MaxIdleConns: cfg.Redis.MaxIdle,
	})
	backend := cache.NewBackend(rdb, cfg.Redis.KeyTTL)
	cch := Cache{
		AccessKeys:      cache.NewRedisCache[cache.KeyAccessKey, *proto.AccessQuota](backend),
		Projects:        cache.NewRedisCache[cache.KeyProject, *proto.AccessQuota](backend),
		UsageCache:      cache.NewUsageCache[cache.KeyAccessKey](backend),
		PermissionCache: cache.NewRedisCache[cache.KeyPermission, UserPermission](backend),
	}
	if cfg.LRUSize > 0 {
		cch.AccessKeys = cache.NewMemory(cch.AccessKeys, cfg.LRUSize, cfg.LRUExpiration)
		cch.Projects = cache.NewMemory(cch.Projects, cfg.LRUSize, cfg.LRUExpiration)
	}

	if qc == nil {
		qc = proto.NewQuotaControlClient(cfg.URL, &http.Client{
			Transport: bearerToken(cfg.AuthToken),
		})
	}

	tick := time.Minute * 5
	if cfg.UpdateFreq > 0 {
		tick = cfg.UpdateFreq
	}

	return &Client{
		cfg:         cfg,
		service:     service,
		usage:       usage.NewTracker(),
		cache:       cch,
		quotaClient: qc,
		ticker:      time.NewTicker(tick),
		logger:      log.With(slog.String("qc-version", proto.WebRPCSchemaVersion())),
	}
}

type Client struct {
	cfg    Config
	logger *slog.Logger

	service     proto.Service
	usage       *usage.Tracker
	cache       Cache
	quotaClient proto.QuotaControlClient

	running int32
	ticker  *time.Ticker
}

var _ middleware.Client = &Client{}

// IsEnabled tells us if the service is running with quotacontrol enabled.
func (c *Client) IsEnabled() bool {
	return c != nil && c.cfg.Enabled //&& c.isRunning()
}

// IsDangerMode tells us if the quotacontrol client is configured in debug mode.
// This is useful for testing and debugging purposes.
func (c *Client) IsDangerMode() bool {
	return c != nil && c.cfg.DangerMode
}

// GetDefaultUsage returns the default usage value.
func (c *Client) GetDefaultUsage() int64 {
	if c.cfg.DefaultUsage != nil {
		return *c.cfg.DefaultUsage
	}
	return 1
}

// GetService returns the client service.
func (c *Client) GetService() proto.Service {
	if c == nil {
		return math.MaxUint16
	}
	return c.service
}

// FetchProjectQuota fetches the project quota from cache or from the quota server.
func (c *Client) FetchProjectQuota(ctx context.Context, projectID uint64, chainIDs []uint64, now time.Time) (*proto.AccessQuota, error) {
	logger := c.logger.With(
		slog.String("op", "fetch_project_quota"),
		slog.Uint64("projectId", projectID),
	)
	// fetch access quota
	quota, ok, err := c.cache.Projects.Get(ctx, cache.KeyProject{ProjectID: projectID})
	if err != nil {
		logger.Warn("unexpected cache error", xlog.Error(err))
		return nil, nil
	}
	if !ok {
		if quota, err = c.quotaClient.GetProjectQuota(ctx, projectID, now); err != nil {
			if !errors.Is(err, proto.ErrAccessKeyNotFound) && !errors.Is(err, proto.ErrProjectNotFound) {
				logger.Warn("unexpected client error", xlog.Error(err))
				return nil, nil
			}
			return nil, err
		}
		if err := c.cache.Projects.Set(ctx, cache.KeyProject{ProjectID: quota.AccessKey.ProjectID}, quota); err != nil {
			logger.Warn("failed to cache project quota", xlog.Error(err))
		}
	}
	if err := quota.Info.ValidateChains(chainIDs); err != nil {
		return quota, proto.ErrInvalidChain.WithCause(err)
	}
	return quota, nil
}

// FetchKeyQuota fetches and validates the accessKey from cache or from the quota server.
func (c *Client) FetchKeyQuota(ctx context.Context, accessKey, origin string, chainIDs []uint64, now time.Time) (*proto.AccessQuota, error) {
	logger := c.logger.With(
		slog.String("op", "fetch_key_quota"),
		slog.String("access_key", accessKey),
	)
	// fetch access quota
	quota, ok, err := c.cache.AccessKeys.Get(ctx, cache.KeyAccessKey{AccessKey: accessKey})
	if err != nil {
		logger.Error("unexpected cache error", xlog.Error(err))
		return nil, nil
	}
	if !ok {
		if quota, err = c.quotaClient.GetAccessQuota(ctx, accessKey, now); err != nil {
			if !errors.Is(err, proto.ErrAccessKeyNotFound) && !errors.Is(err, proto.ErrProjectNotFound) {
				logger.Error("unexpected client error", xlog.Error(err))
				return nil, nil
			}
			return nil, err
		}
		if err := c.cache.AccessKeys.Set(ctx, cache.KeyAccessKey{AccessKey: quota.AccessKey.AccessKey}, quota); err != nil {
			logger.Warn("failed to cache access quota", xlog.Error(err))
		}
	}
	if err := quota.Info.ValidateChains(chainIDs); err != nil {
		return quota, proto.ErrInvalidChain.WithCause(err)
	}
	// validate access key
	if err := c.validateAccessKey(quota.AccessKey, origin); err != nil {
		return quota, err
	}
	return quota, nil
}

// FetchUsage fetches the current usage of the access key.
func (c *Client) FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error) {
	logger := c.logger.With(
		slog.String("op", "fetch_usage"),
		slog.Uint64("projectId", quota.AccessKey.ProjectID),
		slog.String("access_key", quota.AccessKey.AccessKey),
	)

	usage, err := c.EnsureUsage(ctx, quota.AccessKey.ProjectID, quota.Cycle, now)
	if err != nil {
		logger.Error("unexpected error", xlog.Error(err))
		return 0, err
	}
	return usage, nil
}

func (c *Client) EnsureUsage(ctx context.Context, projectID uint64, cycle *proto.Cycle, now time.Time) (int64, error) {
	key := cacheKeyQuota(projectID, cycle, &c.service, now)
	fetcher := func(ctx context.Context, key cache.KeyAccessKey) (int64, error) {
		min, max := cycle.GetStart(now), cycle.GetEnd(now)
		return c.quotaClient.GetUsage(ctx, projectID, nil, &c.service, &min, &max)
	}
	return c.cache.UsageCache.Ensure(ctx, fetcher, key)
}

func (c *Client) CheckPermission(ctx context.Context, projectID uint64, minPermission proto.UserPermission) (bool, error) {
	if sessionType, _ := authcontrol.GetSessionType(ctx); sessionType >= authproto.SessionType_Admin {
		return true, nil
	}
	perm, _, err := c.FetchPermission(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("fetch permission: %w", err)
	}
	return perm >= minPermission, nil
}

// FetchPermission fetches the user permission from cache or from the quota server.
// If an error occurs, it returns nil.
func (c *Client) FetchPermission(ctx context.Context, projectID uint64) (proto.UserPermission, *proto.ResourceAccess, error) {
	userID, _ := authcontrol.GetAccount(ctx)
	logger := c.logger.With(
		slog.String("op", "fetch_permission"),
		slog.Uint64("projectId", projectID),
		slog.String("user_id", userID),
	)
	// Check short-lived cache if requested. Note using the cache TTL from config (default 1m).
	v, ok, err := c.cache.PermissionCache.Get(ctx, cache.KeyPermission{ProjectID: projectID, UserID: userID})
	if err != nil {
		// log the error, but don't stop
		logger.Error("unexpected cache error", xlog.Error(err))
	}
	if ok {
		return v.UserPermission, v.ResourceAccess, nil
	}

	// Ask quotacontrol server via client
	perm, access, err := c.quotaClient.GetUserPermission(ctx, projectID, userID)
	if err != nil {
		logger.Error("unexpected client error", xlog.Error(err))
		return proto.UserPermission_UNAUTHORIZED, nil, fmt.Errorf("get user permission from quotacontrol server: %w", err)
	}
	if !perm.Is(proto.UserPermission_UNAUTHORIZED) {
		if err := c.cache.PermissionCache.Set(ctx, cache.KeyPermission{ProjectID: projectID, UserID: userID}, UserPermission{UserPermission: perm, ResourceAccess: access}); err != nil {
			c.logger.Warn("set user perm in cache", xlog.Error(err))
		}
	}
	return perm, access, nil
}

func (c *Client) SpendQuota(ctx context.Context, quota *proto.AccessQuota, cost int64, now time.Time) (spent bool, total int64, err error) {
	// quota is nil only on unexpected errors from quota fetch
	if quota == nil || cost == 0 {
		return false, 0, nil
	}

	accessKey := quota.AccessKey.AccessKey
	projectID := quota.AccessKey.ProjectID

	logger := c.logger.With(
		slog.String("op", "spend_quota"),
		slog.Uint64("projectId", projectID),
		slog.String("access_key", accessKey),
	)

	cfg, ok := quota.Limit.GetSettings(c.service)
	if !ok {
		logger.Error("service limit not found", slog.String("service", c.service.GetName()))
		return false, 0, nil
	}

	key := cacheKeyQuota(projectID, quota.Cycle, &c.service, now)
	fetcher := func(ctx context.Context, _ cache.KeyAccessKey) (int64, error) {
		min, max := quota.Cycle.GetStart(now), quota.Cycle.GetEnd(now)
		return c.quotaClient.GetUsage(ctx, projectID, nil, &c.service, &min, &max)
	}

	// spend compute units
	total, delta, err := c.cache.UsageCache.Spend(ctx, fetcher, key, cost, cfg.OverMax)
	if err != nil {
		logger.Error("unexpected cache error", xlog.Error(err))
		return false, 0, err
	}

	usage, event := cfg.GetSpendResult(delta, total)
	if accessKey == "" {
		c.usage.AddProjectUsage(projectID, now, usage)
	} else {
		c.usage.AddKeyUsage(accessKey, now, usage)
	}
	if usage < cost {
		return false, total, proto.ErrQuotaExceeded
	}
	if event != nil {
		if _, err := c.quotaClient.NotifyEvent(ctx, projectID, c.service, *event); err != nil {
			logger.Error("notify event failed", xlog.Error(err))
		}
	}
	return true, total, nil
}

func (c *Client) ClearQuotaCacheByProjectID(ctx context.Context, projectID uint64) error {
	_, err := c.cache.Projects.Clear(ctx, cache.KeyProject{ProjectID: projectID})
	return err
}

func (c *Client) ClearQuotaCacheByAccessKey(ctx context.Context, accessKey string) error {
	_, err := c.cache.AccessKeys.Clear(ctx, cache.KeyAccessKey{AccessKey: accessKey})
	return err
}

func (c *Client) validateAccessKey(access *proto.AccessKey, origin string) (err error) {
	if !access.Active {
		return proto.ErrAccessKeyNotFound
	}
	if !access.ValidateOrigin(origin) {
		return proto.ErrInvalidOrigin
	}
	if !access.ValidateService(c.service) {
		return proto.ErrInvalidService
	}
	return nil
}

func (c *Client) Run(ctx context.Context) error {
	if c.isRunning() {
		return fmt.Errorf("quota control: already running")
	}

	logger := c.logger.With("op", "run")
	logger.Info("running...")

	atomic.StoreInt32(&c.running, 1)
	defer atomic.StoreInt32(&c.running, 0)

	// Handle stop signal to ensure clean shutdown
	go func() {
		<-ctx.Done()
		c.Stop(context.Background())
	}()

	// Start the sync
	for range c.ticker.C {
		if err := c.usage.SyncUsage(ctx, c.quotaClient, c.service); err != nil {
			logger.Error("sync usage", xlog.Error(err))
			continue
		}
		logger.Debug("sync usage")
	}
	return nil
}

func (c *Client) Stop(timeoutCtx context.Context) {
	if !c.isRunning() || c.isStopping() {
		return
	}
	atomic.StoreInt32(&c.running, 2)

	logger := c.logger.With("op", "stop")

	logger.Info("stopping...")

	c.ticker.Stop()

	if err := c.usage.SyncUsage(timeoutCtx, c.quotaClient, c.service); err != nil {
		logger.Error("sync usage", xlog.Error(err))
	}
	logger.Info("stopped.")
}

func (c *Client) isRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

func (c *Client) isStopping() bool {
	return atomic.LoadInt32(&c.running) == 2
}

type bearerToken string

func (t bearerToken) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", t))
	return http.DefaultTransport.RoundTrip(req)
}

func cacheKeyQuota(projectID uint64, cycle *proto.Cycle, service *proto.Service, now time.Time) cache.KeyAccessKey {
	start, end := cycle.GetStart(now), cycle.GetEnd(now)
	if service == nil {
		return cache.KeyAccessKey{AccessKey: fmt.Sprintf("project:%v:%s:%s", projectID, start.Format("2006-01-02"), end.Format("2006-01-02"))}
	}
	return cache.KeyAccessKey{AccessKey: fmt.Sprintf("project:%v:%s:%s:%s", projectID, service.GetName(), start.Format("2006-01-02"), end.Format("2006-01-02"))}
}
