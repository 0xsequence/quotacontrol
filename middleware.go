package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/0xsequence/quotacontrol/proto"
)

var (
	ErrTokenNotFound  = proto.Errorf(proto.ErrNotFound, "token not found")
	ErrInvalidOrigin  = proto.Errorf(proto.ErrPermissionDenied, "invalid origin")
	ErrInvalidService = proto.Errorf(proto.ErrPermissionDenied, "invalid service")
	ErrLimitExceeded  = proto.Errorf(proto.ErrResourceExhausted, "limit exceeded")
	ErrTimeout        = proto.Errorf(proto.ErrDeadlineExceeded, "timeout")
)

const (
	HeaderSequenceTokenKey = "X-Sequence-Token-Key"
	HeaderOrigin           = "Origin"
)

func NewClient(log zerolog.Logger, s *proto.Service, cache CacheStorage, qc proto.QuotaControl, rl RateLimiter) *Client {
	return &Client{
		service: s,
		usage: &usageTracker{
			Usage: make(map[time.Time]map[string]*proto.AccessTokenUsage),
		},
		cache:       cache,
		quotaClient: qc,
		rateLimiter: rl,
		Log:         log,
	}
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

func NewMiddleware(c *Client, onSuccess func(context.Context) context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenKey := r.Header.Get(HeaderSequenceTokenKey)
			if tokenKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			ok, err := c.UseToken(ctx, tokenKey, r.Header.Get(HeaderOrigin))
			if err != nil {
				proto.RespondWithError(w, err)
				return
			}
			if !ok {
				proto.RespondWithError(w, ErrLimitExceeded)
				return
			}
			next.ServeHTTP(w, r.WithContext(onSuccess(ctx)))
		})
	}
}

func (c *Client) UseToken(ctx context.Context, tokenKey, origin string) (bool, error) {
	now := GetTime(ctx)
	computeUnits := int64(1)
	if v, ok := ctx.Value(ctxKeyComputeUnits).(int64); ok {
		computeUnits = int64(v)
	}
	// fetch token
	token, err := c.cache.GetToken(ctx, tokenKey)
	if err != nil {
		if !errors.Is(err, ErrTokenNotFound) {
			return false, err
		}
		if token, err = c.quotaClient.RetrieveToken(ctx, tokenKey); err != nil {
			return false, err
		}
	}
	// validate token
	cfg, err := c.validateToken(token, origin)
	if err != nil {
		return false, err
	}
	key := c.service.GetQuotaKey(token.AccessToken.DappID, now)
	// check rate limit
	result, err := c.rateLimiter.RateLimit(ctx, key, int(computeUnits), RateLimit{Rate: cfg.ComputeRateLimit, Period: time.Hour})
	if err != nil {
		return false, err
	}
	if result.Allowed == 0 {
		return false, ErrLimitExceeded
	}
	// spend compute units
	for i := time.Duration(0); i < 3; i++ {
		resp, err := c.cache.SpendComputeUnits(ctx, key, computeUnits, cfg.ComputeMonthlyQuota)
		if err != nil {
			return false, err
		}
		switch *resp {
		case ALLOWED:
			c.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{ValidCompute: computeUnits})
			return true, nil
		case LIMITED:
			c.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{LimitedCompute: computeUnits})
			return false, ErrLimitExceeded
		case PING_BUILDER:
			ok, err := c.quotaClient.PrepareUsage(ctx, token.AccessToken.DappID, c.service, now)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, ErrTimeout
			}
			fallthrough
		case WAIT_AND_RETRY:
			time.Sleep(time.Millisecond * 100 * (i + 1))
		}
	}
	return false, ErrTimeout
}

func (c *Client) validateToken(token *proto.CachedToken, origin string) (cfg *proto.ServiceLimit, err error) {
	if !token.AccessLimit.Active || !token.AccessToken.Active {
		return nil, ErrTokenNotFound
	}
	if !token.AccessToken.ValidateOrigin(origin) {
		return nil, ErrInvalidOrigin
	}
	if !token.AccessToken.ValidateService(c.service) {
		return nil, ErrInvalidOrigin
	}
	for _, cfg = range token.AccessLimit.Config {
		if *cfg.Service == *c.service {
			return cfg, nil
		}
	}
	return nil, ErrInvalidService
}

func (c *Client) Run(ctx context.Context, updateFreq time.Duration) error {
	if c.IsRunning() {
		return fmt.Errorf("quota control: already running")
	}

	c.Log.Info().Str("op", "run").Msg("-> quota control: running")

	atomic.StoreInt32(&c.running, 1)
	c.ticker = time.NewTicker(updateFreq)
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
	if !c.IsRunning() || c.IsStopping() {
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

func (c *Client) IsRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

func (c *Client) IsStopping() bool {
	return atomic.LoadInt32(&c.running) == 2
}