package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	redisclient "github.com/redis/go-redis/v9"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/rs/zerolog"
)

func NewClient(log zerolog.Logger, service *proto.Service, cfg Config) (*Client, error) {
	if !cfg.Enabled {
		return nil, errors.New("0xsequence/quotacontrol: attempting to create client while config.Enabled is false")
	}

	// TODO: set other options too...
	redisClient := redisclient.NewClient(&redisclient.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		DB:           cfg.Redis.DBIndex,
		MaxIdleConns: cfg.Redis.MaxIdle,
	})

	return &Client{
		service: service,
		usage: &usageTracker{
			Usage: make(map[time.Time]map[string]*proto.AccessTokenUsage),
		},
		cache: NewRedisCache(redisClient, time.Minute),
		quotaClient: proto.NewQuotaControlClient(cfg.URL, &authorizedClient{
			client:      http.DefaultClient,
			bearerToken: cfg.Token,
		}),
		rateLimiter: NewRateLimiter(redisClient),
		ticker:      time.NewTicker(cfg.UpdateFreq.Duration),
		Log:         log,
	}, nil
}

type Client struct {
	service     *proto.Service
	usage       *usageTracker
	cache       CacheStorage
	quotaClient proto.QuotaControl
	rateLimiter RateLimiter

	running int32
	ticker  *time.Ticker
	Log     zerolog.Logger
}

func (c *Client) UseToken(ctx context.Context, tokenKey, origin string) (bool, error) {
	now := GetTime(ctx)

	// fetch token
	token, err := c.cache.GetToken(ctx, tokenKey)
	if err != nil {
		if !errors.Is(err, proto.ErrTokenNotFound) {
			return false, err
		}
		if token, err = c.quotaClient.RetrieveToken(ctx, tokenKey); err != nil {
			return false, err
		}
	}
	cfg := token.Limit

	// validate token
	if err := c.validateToken(token.AccessToken, origin); err != nil {
		return false, err
	}
	key := GetQuotaKey(token.AccessToken.ProjectID, now)

	computeUnits := GetComputeUnits(ctx)
	if computeUnits == 0 {
		return true, nil
	}

	// check rate limit
	if ctx.Value(ctxKeyRateLimitSkip) == nil {
		result, err := c.rateLimiter.RateLimit(ctx, key, int(computeUnits), RateLimit{Rate: cfg.RateLimit, Period: time.Minute})
		if err != nil {
			return false, err
		}
		if result.Allowed == 0 {
			return false, proto.ErrLimitExceeded
		}
	}
	// spend compute units
	for i := time.Duration(0); i < 3; i++ {
		total, err := c.cache.SpendComputeUnits(ctx, key, computeUnits, cfg.ComputeMonthlyHardQuota)
		switch err {
		case nil:
			if total > cfg.ComputeMonthlyHardQuota {
				c.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{LimitedCompute: computeUnits})
				return false, proto.ErrLimitExceeded
			}
			if total > cfg.ComputeMonthlyQuota {
				c.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{OverCompute: computeUnits})
				return true, nil
			}
			c.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{ValidCompute: computeUnits})
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
	if !token.ValidateService(c.service) {
		return proto.ErrInvalidService
	}
	return nil
}

func (c *Client) Run(ctx context.Context) error {
	if c.isRunning() {
		return fmt.Errorf("quota control: already running")
	}

	c.Log.Info().Str("op", "run").Msg("-> quota control: running")

	atomic.StoreInt32(&c.running, 1)
	defer atomic.StoreInt32(&c.running, 0)

	// Handle stop signal to ensure clean shutdown
	go func() {
		<-ctx.Done()
		c.Stop(context.Background())
	}()

	// Start the http server and serve!
	for range c.ticker.C {
		if err := c.usage.SyncUsage(ctx, c.quotaClient, c.service); err != nil {
			c.Log.Error().Err(err).Str("op", "run").Msg("-> quota control: failed to sync usage")
		}
	}
	return nil
}

func (c *Client) Stop(timeoutCtx context.Context) {
	if !c.isRunning() || c.isStopping() {
		return
	}
	atomic.StoreInt32(&c.running, 2)

	c.Log.Info().Str("op", "stop").Msg("-> quota control: stopping..")
	c.ticker.Stop()
	if err := c.usage.SyncUsage(timeoutCtx, c.quotaClient, c.service); err != nil {
		c.Log.Error().Err(err).Str("op", "run").Msg("-> quota control: failed to sync usage")
	}
	c.Log.Info().Str("op", "stop").Msg("-> quota control: stopped.")
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

func GetQuotaKey(projectID uint64, now time.Time) string {
	return fmt.Sprintf("project:%v:%s", projectID, now.Format("2006-01"))
}
