package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/logger"
	"github.com/redis/go-redis/v9"
)

type Notifier interface {
	Notify(access *proto.AccessKey) error
}

func NewClient(logger logger.Logger, service proto.Service, cfg Config) *Client {
	options := redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		DB:           cfg.Redis.DBIndex,
		MaxIdleConns: cfg.Redis.MaxIdle,
	}

	redisExpiration := time.Hour
	if cfg.Redis.KeyTTL > 0 {
		redisExpiration = cfg.Redis.KeyTTL
	}
	cache := NewRedisCache(redis.NewClient(&options), redisExpiration)

	quotaCache := QuotaCache(cache)
	if cfg.LRUSize > 0 {
		lruExpiration := time.Minute
		if cfg.LRUExpiration.Duration > 0 {
			lruExpiration = cfg.LRUExpiration.Duration
		}
		quotaCache = NewLRU(quotaCache, cfg.LRUSize, lruExpiration)
	}

	ticker := (*time.Ticker)(nil)
	if cfg.UpdateFreq.Duration > 0 {
		ticker = time.NewTicker(cfg.UpdateFreq.Duration)
	}

	return &Client{
		cfg:     cfg,
		service: service,
		usage: &usageTracker{
			Usage: make(map[time.Time]usageRecord),
		},
		usageCache: cache,
		quotaCache: quotaCache,
		permCache:  PermissionCache(cache),
		quotaClient: proto.NewQuotaControlClient(cfg.URL, &authClient{
			client:      http.DefaultClient,
			bearerToken: cfg.AuthToken,
		}),
		ticker: ticker,
		logger: logger,
	}
}

type Client struct {
	cfg    Config
	logger logger.Logger

	service     proto.Service
	usage       *usageTracker
	usageCache  UsageCache
	quotaCache  QuotaCache
	permCache   PermissionCache
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

// FetchProjectQuota fetches the project quota from cache or from the quota server.
func (c *Client) FetchProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	logger := c.logger.With(
		slog.String("op", "fetch_project_quota"),
		slog.Uint64("project_id", projectID),
	)
	// fetch access quota
	quota, err := c.quotaCache.GetProjectQuota(ctx, projectID)
	if err != nil {
		if !errors.Is(err, proto.ErrAccessKeyNotFound) {
			logger.Warn("unexpected cache error", slog.Any("err", err))
			return nil, nil
		}
		if quota, err = c.quotaClient.GetProjectQuota(ctx, projectID, now); err != nil {
			if !errors.Is(err, proto.ErrAccessKeyNotFound) {
				logger.Warn("unexpected client error", slog.Any("err", err))
				return nil, nil
			}
			return nil, err
		}
	}
	return quota, nil
}

// FetchKeyQuota fetches and validates the accessKey from cache or from the quota server.
func (c *Client) FetchKeyQuota(ctx context.Context, accessKey, origin string, now time.Time) (*proto.AccessQuota, error) {
	logger := c.logger.With(
		slog.String("op", "fetch_key_quota"),
		slog.String("access_key", accessKey),
	)
	// fetch access quota
	quota, err := c.quotaCache.GetAccessQuota(ctx, accessKey)
	if err != nil {
		if !errors.Is(err, proto.ErrAccessKeyNotFound) {
			logger.Warn("unexpected cache error", slog.Any("err", err))
			return nil, nil
		}
		if quota, err = c.quotaClient.GetAccessQuota(ctx, accessKey, now); err != nil {
			if !errors.Is(err, proto.ErrAccessKeyNotFound) {
				logger.Warn("unexpected client error", slog.Any("err", err))
				return nil, nil
			}
			return nil, err
		}
	}
	// validate access key
	if err := c.validateAccessKey(quota.AccessKey, origin); err != nil {
		return quota, err
	}
	return quota, nil
}

// FetchUsage fetches the current usage of the access key.
func (c *Client) FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error) {
	key := getQuotaKey(quota.AccessKey.ProjectID, quota.Cycle, now)

	logger := c.logger.With(
		slog.String("op", "fetch_usage"),
		slog.Uint64("project_id", quota.AccessKey.ProjectID),
		slog.String("access_key", quota.AccessKey.AccessKey),
	)

	for i := time.Duration(0); i < 3; i++ {
		usage, err := c.usageCache.PeekComputeUnits(ctx, key)
		if err != nil {
			// ping the server to prepare usage
			if errors.Is(err, ErrCachePing) {
				if _, err := c.quotaClient.PrepareUsage(ctx, quota.AccessKey.ProjectID, quota.Cycle, now); err != nil {
					logger.Error("unexpected client error", slog.Any("err", err))
					if _, err := c.usageCache.ClearComputeUnits(ctx, key); err != nil {
						logger.Error("unexpected cache error", slog.Any("err", err))
					}
					return 0, nil
				}
				continue
			}

			// wait for cache to be ready
			if errors.Is(err, ErrCacheWait) {
				time.Sleep(time.Millisecond * 100 * (i + 1))
				continue
			}

			logger.Error("unexpected cache error", slog.Any("err", err))
			return 0, err
		}

		return usage, nil
	}
	logger.Error("operation timed out")
	return 0, nil
}

func (c *Client) FetchUserPermission(ctx context.Context, projectID uint64, userID string, useCache bool) (*proto.UserPermission, *proto.ResourceAccess, error) {
	logger := c.logger.With(
		slog.String("op", "spend_quota"),
		slog.Uint64("project_id", projectID),
		slog.String("user_id", userID),
	)
	// Check short-lived cache if requested. Note, the cache ttl is 10 seconds.
	if useCache {
		perm, access, err := c.permCache.GetUserPermission(ctx, projectID, userID)
		if err != nil {
			// log the error, but don't stop
			logger.Error("unexpected cache error", slog.Any("err", err))
		}
		if perm != nil {
			return perm, access, nil
		}
	}

	// Ask quotacontrol server via client
	perm, access, err := c.quotaClient.GetUserPermission(ctx, projectID, userID)
	if err != nil {
		logger.Error("unexpected client error", slog.Any("err", err))
		return nil, nil, err
	}
	// if userPerm is still nil, return unauthorized
	if perm == nil {
		perm = proto.Ptr(proto.UserPermission_UNAUTHORIZED)
	}

	return perm, access, nil
}

func (c *Client) SpendQuota(ctx context.Context, quota *proto.AccessQuota, computeUnits int64, now time.Time) (bool, error) {
	// quota is nil only on unexpected errors from quota fetch
	if quota == nil || computeUnits == 0 {
		return false, nil
	}

	logger := c.logger.With(
		slog.String("op", "spend_quota"),
		slog.Uint64("project_id", quota.AccessKey.ProjectID),
		slog.String("access_key", quota.AccessKey.AccessKey),
	)

	accessKey := quota.AccessKey.AccessKey
	cfg := quota.Limit

	// spend compute units
	key := getQuotaKey(quota.AccessKey.ProjectID, quota.Cycle, now)

	for i := time.Duration(0); i < 3; i++ {
		total, err := c.usageCache.SpendComputeUnits(ctx, key, computeUnits, cfg.OverMax)
		if err != nil {
			// limit exceeded
			if errors.Is(err, proto.ErrLimitExceeded) {
				c.usage.AddKeyUsage(accessKey, now, proto.AccessUsage{LimitedCompute: computeUnits})
				return false, proto.ErrLimitExceeded
			}
			// ping the server to prepare usage
			if errors.Is(err, ErrCachePing) {
				if _, err := c.quotaClient.PrepareUsage(ctx, quota.AccessKey.ProjectID, quota.Cycle, now); err != nil {
					logger.Error("unexpected client error", slog.Any("err", err))
					if _, err := c.usageCache.ClearComputeUnits(ctx, key); err != nil {
						logger.Error("unexpected cache error", slog.Any("err", err))
					}
					return false, nil
				}
				continue
			}

			// wait for cache to be ready
			if errors.Is(err, ErrCacheWait) {
				time.Sleep(time.Millisecond * 100 * (i + 1))
				continue
			}

			logger.Error("unexpected cache error", slog.Any("err", err))
			return false, err

		}

		usage, event := cfg.GetSpendResult(computeUnits, total)
		if quota.AccessKey.AccessKey == "" {
			c.usage.AddProjectUsage(quota.AccessKey.ProjectID, now, usage)
		} else {
			c.usage.AddKeyUsage(accessKey, now, usage)
		}
		if usage.LimitedCompute != 0 {
			return false, proto.ErrLimitExceeded
		}
		if event != nil {
			if _, err := c.quotaClient.NotifyEvent(ctx, quota.AccessKey.ProjectID, event); err != nil {
				logger.Error("notify event failed", slog.Any("err", err))
			}
		}
		return true, nil
	}
	logger.Error("operation timed out")
	return false, nil
}

func (c *Client) ClearQuotaCacheByProjectID(ctx context.Context, projectID uint64) error {
	return c.quotaCache.DeleteProjectQuota(ctx, projectID)
}

func (c *Client) ClearQuotaCacheByAccessKey(ctx context.Context, accessKey string) error {
	return c.quotaCache.DeleteAccessQuota(ctx, accessKey)
}

func (c *Client) validateAccessKey(access *proto.AccessKey, origin string) (err error) {
	if !access.Active {
		return proto.ErrAccessKeyNotFound
	}
	if !access.ValidateOrigin(origin) {
		return proto.ErrInvalidOrigin
	}
	if !access.ValidateService(&c.service) {
		return proto.ErrInvalidService
	}
	return nil
}

func (c *Client) Run(ctx context.Context) error {
	if c.isRunning() {
		return fmt.Errorf("quota control: already running")
	}

	c.logger.With("op", "run").Info("-> quota control: running")

	atomic.StoreInt32(&c.running, 1)
	defer atomic.StoreInt32(&c.running, 0)

	// Handle stop signal to ensure clean shutdown
	go func() {
		<-ctx.Done()
		c.Stop(context.Background())
	}()
	if c.ticker == nil {
		return nil
	}
	// Start the sync
	for range c.ticker.C {
		if err := c.usage.SyncUsage(ctx, c.quotaClient, &c.service); err != nil {
			c.logger.With("err", err, "op", "run").Error("-> quota control: failed to sync usage")
			continue
		}
		c.logger.With("op", "run").Info("-> quota control: synced usage")
	}
	return nil
}

func (c *Client) Stop(timeoutCtx context.Context) {
	if !c.isRunning() || c.isStopping() {
		return
	}
	atomic.StoreInt32(&c.running, 2)

	c.logger.With("op", "stop").Info("-> quota control: stopping..")
	if c.ticker != nil {
		c.ticker.Stop()
	}
	if err := c.usage.SyncUsage(timeoutCtx, c.quotaClient, &c.service); err != nil {
		c.logger.With("err", err, "op", "run").Error("-> quota control: failed to sync usage")
	}
	c.logger.With("op", "stop").Info("-> quota control: stopped.")
}

func (c *Client) isRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

func (c *Client) isStopping() bool {
	return atomic.LoadInt32(&c.running) == 2
}

type authClient struct {
	client      *http.Client
	bearerToken string
}

func (c *authClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", c.bearerToken))
	return c.client.Do(req)
}

func getQuotaKey(projectID uint64, cycle *proto.Cycle, now time.Time) string {
	start, end := cycle.GetStart(now), cycle.GetEnd(now)
	return fmt.Sprintf("project:%v:%s:%s", projectID, start.Format("2006-01-02"), end.Format("2006-01-02"))
}
