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

type Notifier interface {
	Notify(access *proto.AccessKey) error
}

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
		quotaCache = NewLRU(quotaCache, cfg.LRUSize, cfg.LRUExpiration.Duration)
	}

	permCache := PermissionCache(cache)

	return &Client{
		cfg:     cfg,
		service: service,
		usage: &usageTracker{
			Usage: make(map[time.Time]usageRecord),
		},
		usageCache: UsageCache(cache),
		quotaCache: quotaCache,
		permCache:  permCache,
		quotaClient: proto.NewQuotaControlClient(cfg.URL, &authorizedClient{
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

// FetchProjectQuota fetches the project quota from cache or from the quota server.
func (c *Client) FetchProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	// fetch access quota
	quota, err := c.quotaCache.GetProjectQuota(ctx, projectID)
	if err != nil {
		if !errors.Is(err, proto.ErrAccessKeyNotFound) {
			return nil, err
		}
		if quota, err = c.quotaClient.GetProjectQuota(ctx, projectID, now); err != nil {
			return nil, err
		}
	}
	return quota, nil
}

// FetchKeyQuota fetches and validates the accessKey from cache or from the quota server.
func (c *Client) FetchKeyQuota(ctx context.Context, accessKey, origin string, now time.Time) (*proto.AccessQuota, error) {
	// fetch access quota
	quota, err := c.quotaCache.GetAccessQuota(ctx, accessKey)
	if err != nil {
		if !errors.Is(err, proto.ErrAccessKeyNotFound) {
			return nil, err
		}
		if quota, err = c.quotaClient.GetAccessQuota(ctx, accessKey, now); err != nil {
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
	for i := time.Duration(0); i < 3; i++ {
		usage, err := c.usageCache.PeekComputeUnits(ctx, key)
		switch err {
		case nil:
			return usage, nil
		case ErrCachePing:
			ok, err := c.quotaClient.PrepareUsage(ctx, quota.AccessKey.ProjectID, quota.Cycle, now)
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
	accessKey := quota.AccessKey.AccessKey
	cfg := quota.Limit

	// spend compute units
	key := getQuotaKey(quota.AccessKey.ProjectID, quota.Cycle, now)

	for i := time.Duration(0); i < 3; i++ {
		total, err := c.usageCache.SpendComputeUnits(ctx, key, computeUnits, cfg.OverMax)
		switch err {
		case nil:
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
					c.logger.With("err", err, "op", "use_access_key", "event", event).Error("-> quota control: failed to notify")
				}
			}
			return true, nil
		case proto.ErrLimitExceeded:
			c.usage.AddKeyUsage(accessKey, now, proto.AccessUsage{LimitedCompute: computeUnits})
			return false, err
		case ErrCachePing:
			ok, err := c.quotaClient.PrepareUsage(ctx, quota.AccessKey.ProjectID, quota.Cycle, now)
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

type authorizedClient struct {
	client      *http.Client
	bearerToken string
}

func (c *authorizedClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", c.bearerToken))
	return c.client.Do(req)
}

func getQuotaKey(projectID uint64, cycle *proto.Cycle, now time.Time) string {
	start, end := cycle.GetStart(now), cycle.GetEnd(now)
	return fmt.Sprintf("project:%v:%s:%s", projectID, start.Format("2006-01-02"), end.Format("2006-01-02"))
}
