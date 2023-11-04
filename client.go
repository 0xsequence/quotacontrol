package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/logger"
	redisclient "github.com/redis/go-redis/v9"
)

func NewClient(logger logger.Logger, service proto.Service, cfg Config) *Client {
	// TODO: set other options too...
	redisClient := redisclient.NewClient(&redisclient.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		DB:           cfg.Redis.DBIndex,
		MaxIdleConns: cfg.Redis.MaxIdle,
	})

	ticker := (*time.Ticker)(nil)
	if cfg.UpdateFreq.Duration > 0 {
		ticker = time.NewTicker(cfg.UpdateFreq.Duration)
	}

	cache := NewRedisCache(redisClient, time.Minute)
	quotaCache := QuotaCache(cache)
	if cfg.LRUSize > 0 {
		quotaCache = NewLRU(quotaCache, cfg.LRUSize)
	}

	permCache := PermissionCache(cache)

	return &Client{
		cfg:    cfg,
		logger: logger,

		service:      service,
		specialKeys:  newSpecialKeys(cfg.SpecialKeys),
		usageTracker: newUsageTracker(),

		usageCache: UsageCache(cache),
		quotaCache: quotaCache,
		permCache:  permCache,

		quotaClient: proto.NewQuotaControlClient(cfg.URL, &authorizedClient{
			client:      http.DefaultClient,
			bearerToken: cfg.AccessKey,
		}),
		rateLimiter: NewRateLimiter(redisClient),

		ticker: ticker,
	}
}

type Client struct {
	cfg    Config
	logger logger.Logger

	service      proto.Service
	specialKeys  *specialKeys
	usageTracker *usageTracker

	usageCache UsageCache
	quotaCache QuotaCache
	permCache  PermissionCache

	quotaClient proto.QuotaControl
	rateLimiter RateLimiter

	running int32
	ticker  *time.Ticker
}

var _ middleware.Client = &Client{}

// IsEnabled tells us if the service is running with quotacontrol enabled.
func (c *Client) IsEnabled() bool {
	return c.cfg.Enabled //&& c.isRunning()
}

func (c *Client) AddSpecialKey(key string, projectID uint64) {
	c.specialKeys.Set(key, projectID)
}

func (c *Client) RemoveSpecialKey(key string) {
	c.specialKeys.Delete(key)
}

// FetchQuota fetches and validates the accessKey from cache or from the quota server.
func (c *Client) FetchQuota(ctx context.Context, accessKey, origin string) (*proto.AccessQuota, error) {
	// check special keys
	if projectID, ok := c.specialKeys.Get(accessKey); ok {
		return &proto.AccessQuota{
			AccessKey: &proto.AccessKey{
				ProjectID:   projectID,
				AccessKey:   accessKey,
				DisplayName: "special",
				Active:      true,
			},
			Limit: &proto.Limit{
				RateLimit: 1_000_000_000_000_000_000,
				FreeCU:    1_000_000_000_000_000_000,
				SoftQuota: 1_000_000_000_000_000_000,
				HardQuota: 1_000_000_000_000_000_000,
			},
		}, nil
	}
	// fetch access quota
	quota, err := c.quotaCache.GetAccessQuota(ctx, accessKey)
	if err != nil {
		if !errors.Is(err, proto.ErrAccessKeyNotFound) {
			return nil, err
		}
		if quota, err = c.quotaClient.GetAccessQuota(ctx, accessKey); err != nil {
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
	// check special keys
	if _, ok := c.specialKeys.Get(quota.AccessKey.AccessKey); ok {
		return 0, nil
	}

	key := getQuotaKey(quota.AccessKey.ProjectID, now)
	for i := time.Duration(0); i < 3; i++ {
		usage, err := c.usageCache.PeekComputeUnits(ctx, key)
		switch err {
		case nil:
			return usage, nil
		case ErrCachePing:
			ok, err := c.quotaClient.PrepareUsage(ctx, quota.AccessKey.ProjectID, now)
			if err != nil {
				return 0, err
			}
			if !ok {
				return 0, proto.ErrTimeout
			}
			fallthrough
		case ErrCacheWait:
			time.Sleep(time.Millisecond * 100 * (i + 1))
		default:
			return 0, err
		}
	}
	return 0, proto.ErrTimeout
}

func (c *Client) FetchUserPermission(ctx context.Context, projectID uint64, userID string, useCache bool) (*proto.UserPermission, map[string]any, error) {
	var userPerm *proto.UserPermission
	var resourceAccess map[string]interface{}
	var err error

	// Check short-lived cache if requested. Note, the cache ttl is 10 seconds.
	if useCache {
		userPerm, resourceAccess, err = c.permCache.GetUserPermission(ctx, projectID, userID)
		if err != nil {
			// log the error, but don't stop
			c.logger.With("err", err).Error("FetchUserPermission failed to query the permCache")
		}
	}

	// Ask quotacontrol server via client
	if userPerm == nil {
		userPerm, resourceAccess, err = c.quotaClient.GetUserPermission(ctx, projectID, userID)
		if err != nil {
			return userPerm, resourceAccess, err
		}
	}

	// Check if userPerm is still nil, in which case return unauthorized
	if userPerm == nil {
		v := proto.UserPermission_UNAUTHORIZED
		return &v, resourceAccess, nil
	}

	return userPerm, resourceAccess, nil
}

func (c *Client) SpendQuota(ctx context.Context, quota *proto.AccessQuota, computeUnits int64, now time.Time) (bool, error) {
	// check special keys
	accessKey := quota.AccessKey.AccessKey
	if _, ok := c.specialKeys.Get(accessKey); ok {
		return true, nil
	}
	cfg := quota.Limit

	// spend compute units
	key := getQuotaKey(quota.AccessKey.ProjectID, now)

	// check rate limit
	if !middleware.IsSkipRateLimit(ctx) {
		result, err := c.rateLimiter.RateLimit(ctx, key, int(computeUnits), RateLimit{Rate: cfg.RateLimit, Period: time.Minute})
		if err != nil {
			return false, err
		}
		if result.Allowed == 0 {
			return false, proto.ErrLimitExceeded
		}
	}

	for i := time.Duration(0); i < 3; i++ {
		total, err := c.usageCache.SpendComputeUnits(ctx, key, computeUnits, cfg.HardQuota)
		switch err {
		case nil:
			usage, event := cfg.GetSpendResult(computeUnits, total)
			c.usageTracker.AddUsage(accessKey, now, usage)
			if usage.LimitedCompute != 0 {
				return false, proto.ErrLimitExceeded
			}
			if event != nil {
				if _, err := c.quotaClient.NotifyEvent(ctx, quota.AccessKey.ProjectID, event); err != nil {
					c.logger.With("err", err, "op", "use_access_key", "event", event).Error("-> quota control: failed to notify")
				}
			}
			return true, nil
		case proto.ErrLimitExceeded:
			c.usageTracker.AddUsage(accessKey, now, proto.AccessUsage{LimitedCompute: computeUnits})
			return false, err
		case ErrCachePing:
			ok, err := c.quotaClient.PrepareUsage(ctx, quota.AccessKey.ProjectID, now)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, proto.ErrTimeout
			}
			fallthrough
		case ErrCacheWait:
			time.Sleep(time.Millisecond * 100 * (i + 1))
		default:
			return false, err
		}
	}

	return false, proto.ErrTimeout
}

func (c *Client) ClearQuotaCacheByAccessKey(ctx context.Context, accessKey string) error {
	return c.quotaCache.DeleteAccessKey(ctx, accessKey)
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
		if err := c.usageTracker.SyncUsage(ctx, c.quotaClient, &c.service); err != nil {
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
	if err := c.usageTracker.SyncUsage(timeoutCtx, c.quotaClient, &c.service); err != nil {
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

type authorizedClient struct {
	client      *http.Client
	bearerToken string
}

func (c *authorizedClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", c.bearerToken))
	return c.client.Do(req)
}

func getQuotaKey(projectID uint64, now time.Time) string {
	return fmt.Sprintf("project:%v:%s", projectID, now.Format("2006-01"))
}
