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
	redisclient "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type Notifier interface {
	Notify(access *proto.AccessKey) error
}

func NewClient(logger zerolog.Logger, service proto.Service, cfg Config) *Client {
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
		quotaCache, _ = NewLRU(quotaCache, cfg.LRUSize)
	}
	return &Client{
		cfg:     cfg,
		service: service,
		usage: &usageTracker{
			Usage: make(map[time.Time]map[string]*proto.AccessUsage),
		},
		usageCache: UsageCache(cache),
		quotaCache: quotaCache,
		quotaClient: proto.NewQuotaControlClient(cfg.URL, &authorizedClient{
			client:      http.DefaultClient,
			bearerToken: cfg.AccessKey,
		}),
		rateLimiter: NewRateLimiter(redisClient),
		ticker:      ticker,
		logger:      logger,
	}
}

type Client struct {
	cfg Config

	service     proto.Service
	usage       *usageTracker
	usageCache  UsageCache
	quotaCache  QuotaCache
	quotaClient proto.QuotaControl
	rateLimiter RateLimiter

	running int32
	ticker  *time.Ticker
	logger  zerolog.Logger
}

var _ middleware.Client = &Client{}

// FetchQuota fetches and validates the accessKey from cache or from the quota server.
func (c *Client) FetchQuota(ctx context.Context, accessKey, origin string) (*proto.AccessQuota, error) {
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

// GetUsage returns the current usage of the access key.
func (c *Client) GetUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error) {
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

func (c *Client) SpendQuota(ctx context.Context, quota *proto.AccessQuota, computeUnits int64, now time.Time) (bool, error) {
	accessKey := quota.AccessKey.AccessKey
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
			c.usage.AddUsage(accessKey, now, usage)
			if usage.LimitedCompute != 0 {
				return false, proto.ErrLimitExceeded
			}
			if event != nil {
				if _, err := c.quotaClient.NotifyEvent(ctx, quota.AccessKey.ProjectID, event); err != nil {
					c.logger.Error().Err(err).Str("op", "use_access_key").Stringer("event", event).Msg("-> quota control: failed to notify")
				}
			}
			return true, nil
		case proto.ErrLimitExceeded:
			c.usage.AddUsage(accessKey, now, proto.AccessUsage{LimitedCompute: computeUnits})
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

	c.logger.Info().Str("op", "run").Msg("-> quota control: running")

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
			c.logger.Error().Err(err).Str("op", "run").Msg("-> quota control: failed to sync usage")
			continue
		}
		c.logger.Info().Str("op", "run").Msg("-> quota control: synced usage")
	}
	return nil
}

func (c *Client) Stop(timeoutCtx context.Context) {
	if !c.isRunning() || c.isStopping() {
		return
	}
	atomic.StoreInt32(&c.running, 2)

	c.logger.Info().Str("op", "stop").Msg("-> quota control: stopping..")
	if c.ticker != nil {
		c.ticker.Stop()
	}
	if err := c.usage.SyncUsage(timeoutCtx, c.quotaClient, &c.service); err != nil {
		c.logger.Error().Err(err).Str("op", "run").Msg("-> quota control: failed to sync usage")
	}
	c.logger.Info().Str("op", "stop").Msg("-> quota control: stopped.")
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
