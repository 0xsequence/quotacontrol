package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	redisclient "github.com/redis/go-redis/v9"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/rs/zerolog"
)

type Notifier interface {
	Notify(token *proto.AccessToken) error
}

func NewClient(logger zerolog.Logger, service proto.Service, notifer Notifier, cfg Config) *Client {
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

	return &Client{
		cfg:     cfg,
		service: service,
		usage: &usageTracker{
			Usage: make(map[time.Time]map[string]*proto.AccessTokenUsage),
		},
		cache:    NewRedisCache(redisClient, time.Minute),
		notifier: notifer,
		quotaClient: proto.NewQuotaControlClient(cfg.URL, &authorizedClient{
			client:      http.DefaultClient,
			bearerToken: cfg.Token,
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
	cache       Cache
	notifier    Notifier
	quotaClient proto.QuotaControl
	rateLimiter RateLimiter

	running int32
	ticker  *time.Ticker
	logger  zerolog.Logger
}

// FetchToken fetches and validates the token from cache or from the quota server.
func (c *Client) FetchToken(ctx context.Context, tokenKey, origin string) (*proto.CachedToken, error) {
	// fetch token
	token, err := c.cache.GetToken(ctx, tokenKey)
	if err != nil {
		if !errors.Is(err, proto.ErrTokenNotFound) {
			return nil, err
		}
		if token, err = c.quotaClient.RetrieveToken(ctx, tokenKey); err != nil {
			return nil, err
		}
	}
	// validate token
	if err := c.validateToken(token.AccessToken, origin); err != nil {
		return token, err
	}
	return token, nil
}

// GetUsage returns the current usage of the token.
func (c *Client) GetUsage(ctx context.Context, token *proto.CachedToken, now time.Time) (int64, error) {
	key := getQuotaKey(token.AccessToken.ProjectID, now)
	for i := time.Duration(0); i < 3; i++ {
		usage, err := c.cache.PeekComputeUnits(ctx, key)
		switch err {
		case nil:
			return usage, nil
		case ErrCachePing:
			ok, err := c.quotaClient.PrepareUsage(ctx, token.AccessToken.ProjectID, now)
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

func (c *Client) SpendToken(ctx context.Context, token *proto.CachedToken, computeUnits int64, now time.Time) (bool, error) {
	tokenKey := token.AccessToken.TokenKey
	cfg := token.Limit
	// spend compute units
	key := getQuotaKey(token.AccessToken.ProjectID, now)
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
		total, err := c.cache.SpendComputeUnits(ctx, key, computeUnits, cfg.HardQuota)
		switch err {
		case nil:
			if total > cfg.HardQuota {
				c.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{LimitedCompute: computeUnits})
				return false, proto.ErrLimitExceeded
			}
			if total <= cfg.FreeCU {
				c.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{ValidCompute: computeUnits})
				return true, nil
			}
			if total-computeUnits <= cfg.SoftQuota && total > cfg.SoftQuota {
				if c.notifier != nil {
					if err := c.notifier.Notify(token.AccessToken); err != nil {
						c.logger.Error().Err(err).Str("op", "use_token").Msg("-> quota control: failed to notify")
					}
				}
			}
			c.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{OverCompute: computeUnits})
			return true, nil
		case proto.ErrLimitExceeded:
			c.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{LimitedCompute: computeUnits})
			return false, err
		case ErrCachePing:
			ok, err := c.quotaClient.PrepareUsage(ctx, token.AccessToken.ProjectID, now)
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

func (c *Client) validateToken(token *proto.AccessToken, origin string) (err error) {
	if !token.Active {
		return proto.ErrTokenNotFound
	}
	if !token.ValidateOrigin(origin) {
		return proto.ErrInvalidOrigin
	}
	if !token.ValidateService(&c.service) {
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
