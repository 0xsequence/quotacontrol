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
func NewClient(logger *slog.Logger, service proto.Service, cfg Config, qc proto.QuotaControl) *Client {
	backend := NewRedisCache(redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		DB:           cfg.Redis.DBIndex,
		MaxIdleConns: cfg.Redis.MaxIdle,
	}), cfg.Redis.KeyTTL)

	cache := Cache{
		UsageCache:      backend,
		QuotaCache:      backend,
		PermissionCache: backend,
	}
	// LRU cache for Quota
	if cfg.LRUSize > 0 {
		cache.QuotaCache = NewLRU(backend, cfg.LRUSize, cfg.LRUExpiration)
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
		cache:       cache,
		quotaClient: qc,
		ticker:      time.NewTicker(tick),
		logger:      logger.With(slog.String("service", "quotacontrol")),
	}
}

type Client struct {
	cfg    Config
	logger *slog.Logger

	service     proto.Service
	usage       *usage.Tracker
	cache       Cache
	quotaClient proto.QuotaControl

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
	// fetch access quota
	quota, err := c.cache.QuotaCache.GetProjectQuota(ctx, projectID)
	if err != nil {
		logger := c.logger.With(
			slog.String("op", "fetch_project_quota"),
			slog.Uint64("project_id", projectID),
		)
		if !errors.Is(err, proto.ErrAccessKeyNotFound) && !errors.Is(err, proto.ErrProjectNotFound) {
			logger.Warn("unexpected cache error", slog.Any("error", err))
			return nil, nil
		}
		if quota, err = c.quotaClient.GetProjectQuota(ctx, projectID, now); err != nil {
			if !errors.Is(err, proto.ErrAccessKeyNotFound) && !errors.Is(err, proto.ErrProjectNotFound) {
				logger.Warn("unexpected client error", slog.Any("error", err))
				return nil, nil
			}
			return nil, err
		}
	}
	if err := quota.AccessKey.ValidateChains(chainIDs); err != nil {
		return quota, err
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
	quota, err := c.cache.QuotaCache.GetAccessQuota(ctx, accessKey)
	if err != nil {
		if !errors.Is(err, proto.ErrAccessKeyNotFound) {
			logger.Warn("unexpected cache error", slog.Any("error", err))
			return nil, nil
		}
		if quota, err = c.quotaClient.GetAccessQuota(ctx, accessKey, now); err != nil {
			if !errors.Is(err, proto.ErrAccessKeyNotFound) && !errors.Is(err, proto.ErrProjectNotFound) {
				logger.Warn("unexpected client error", slog.Any("error", err))
				return nil, nil
			}
			return nil, err
		}
	}
	// validate access key
	if err := c.validateAccessKey(quota.AccessKey, origin, chainIDs); err != nil {
		return quota, err
	}
	return quota, nil
}

// FetchUsage fetches the current usage of the access key.
func (c *Client) FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error) {
	key := cacheKeyQuota(quota.AccessKey.ProjectID, quota.Cycle, now)

	logger := c.logger.With(
		slog.String("op", "fetch_usage"),
		slog.Uint64("project_id", quota.AccessKey.ProjectID),
		slog.String("access_key", quota.AccessKey.AccessKey),
	)

	for i := range 3 {
		usage, err := c.cache.UsageCache.PeekUsage(ctx, key)
		if err != nil {
			// ping the server to prepare usage
			if errors.Is(err, ErrCachePing) {
				if _, err := c.quotaClient.PrepareUsage(ctx, quota.AccessKey.ProjectID, quota.Cycle, now); err != nil {
					logger.Error("unexpected client error", slog.Any("error", err))
					if _, err := c.cache.UsageCache.ClearUsage(ctx, key); err != nil {
						logger.Error("unexpected cache error", slog.Any("error", err))
					}
					return 0, nil
				}
				continue
			}

			// wait for cache to be ready
			if errors.Is(err, ErrCacheWait) {
				time.Sleep(time.Millisecond * 100 * time.Duration(i+1))
				continue
			}

			logger.Error("unexpected cache error", slog.Any("error", err))
			return 0, err
		}

		return usage, nil
	}
	logger.Error("operation timed out")
	return 0, nil
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
		slog.Uint64("project_id", projectID),
		slog.String("user_id", userID),
	)
	// Check short-lived cache if requested. Note using the cache TTL from config (default 1m).
	perm, access, err := c.cache.PermissionCache.GetUserPermission(ctx, projectID, userID)
	if err != nil {
		// log the error, but don't stop
		logger.Error("unexpected cache error", slog.Any("error", err))
	}
	if perm != proto.UserPermission_UNAUTHORIZED {
		return perm, access, nil
	}

	// Ask quotacontrol server via client
	perm, access, err = c.quotaClient.GetUserPermission(ctx, projectID, userID)
	if err != nil {
		logger.Error("unexpected client error", slog.Any("error", err))
		return proto.UserPermission_UNAUTHORIZED, nil, fmt.Errorf("get user permission from quotacontrol server: %w", err)
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
		slog.Uint64("project_id", projectID),
		slog.String("access_key", accessKey),
	)

	cfg := quota.Limit

	// spend compute units
	key := cacheKeyQuota(projectID, quota.Cycle, now)

	for i := range 3 {
		total, err := c.cache.UsageCache.SpendUsage(ctx, key, cost, cfg.OverMax)
		if err != nil {
			// limit exceeded
			if errors.Is(err, proto.ErrQuotaExceeded) {
				usage := proto.AccessUsage{LimitedCompute: cost}
				if accessKey == "" {
					c.usage.AddProjectUsage(projectID, now, usage)
				} else {
					c.usage.AddKeyUsage(accessKey, now, usage)
				}
				return false, total, proto.ErrQuotaExceeded
			}
			// ping the server to prepare usage
			if errors.Is(err, ErrCachePing) {
				if _, err := c.quotaClient.PrepareUsage(ctx, projectID, quota.Cycle, now); err != nil {
					logger.Error("unexpected client error", slog.Any("error", err))
					if _, err := c.cache.UsageCache.ClearUsage(ctx, key); err != nil {
						logger.Error("unexpected cache error", slog.Any("error", err))
					}
					return false, 0, nil
				}
				continue
			}

			// wait for cache to be ready
			if errors.Is(err, ErrCacheWait) {
				time.Sleep(time.Millisecond * 100 * time.Duration(i+1))
				continue
			}

			logger.Error("unexpected cache error", slog.Any("error", err))
			return false, 0, err

		}

		usage, event := cfg.GetSpendResult(cost, total)
		if accessKey == "" {
			c.usage.AddProjectUsage(projectID, now, usage)
		} else {
			c.usage.AddKeyUsage(accessKey, now, usage)
		}
		if usage.LimitedCompute != 0 {
			return false, total, proto.ErrQuotaExceeded
		}
		if event != nil {
			if _, err := c.quotaClient.NotifyEvent(ctx, projectID, *event); err != nil {
				logger.Error("notify event failed", slog.Any("error", err))
			}
		}
		return true, total, nil
	}
	logger.Error("operation timed out")
	return false, 0, nil
}

func (c *Client) ClearQuotaCacheByProjectID(ctx context.Context, projectID uint64) error {
	return c.cache.QuotaCache.DeleteProjectQuota(ctx, projectID)
}

func (c *Client) ClearQuotaCacheByAccessKey(ctx context.Context, accessKey string) error {
	return c.cache.QuotaCache.DeleteAccessQuota(ctx, accessKey)
}

func (c *Client) validateAccessKey(access *proto.AccessKey, origin string, chainIDs []uint64) (err error) {
	if !access.Active {
		return proto.ErrAccessKeyNotFound
	}
	if !access.ValidateOrigin(origin) {
		return proto.ErrInvalidOrigin
	}
	if !access.ValidateService(c.service) {
		return proto.ErrInvalidService
	}
	if err := access.ValidateChains(chainIDs); err != nil {
		return proto.ErrInvalidChain.WithCause(err)
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
			logger.Error("sync usage", slog.Any("error", err))
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
		logger.Error("sync usage", slog.Any("error", err))
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

func cacheKeyQuota(projectID uint64, cycle *proto.Cycle, now time.Time) string {
	start, end := cycle.GetStart(now), cycle.GetEnd(now)
	return fmt.Sprintf("project:%v:%s:%s", projectID, start.Format("2006-01-02"), end.Format("2006-01-02"))
}
