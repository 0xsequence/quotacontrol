package quotacontrol

import (
	"context"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

// RateLimit is a representation of a time constrain (rate/duration).
type RateLimit struct {
	Rate   int64
	Period time.Duration
}

// RateLimitResult is the result of a rate limit.
type RateLimitResult struct {
	Limit      RateLimit
	Allowed    int64
	Remaining  int64
	RetryAfter time.Duration
	ResetAfter time.Duration
}

// RateLimiter leaves to the client how to define the key.
type RateLimiter interface {
	RateLimit(ctx context.Context, key string, computeUnits int, limit RateLimit) (*RateLimitResult, error)
}

// NewRateLimiter returns a new redis backed rate limiter.
func NewRateLimiter(client *redis.Client) RateLimiter {
	return &redisRateLimit{client: redis_rate.NewLimiter(client)}
}

type redisRateLimit struct {
	client *redis_rate.Limiter
}

func (r *redisRateLimit) RateLimit(ctx context.Context, key string, computeUnits int, l RateLimit) (*RateLimitResult, error) {
	res, err := r.client.AllowN(ctx, key, redis_rate.Limit{Rate: int(l.Rate), Period: l.Period, Burst: int(l.Rate)}, computeUnits)
	if err != nil {
		return nil, err
	}
	return &RateLimitResult{
		Limit: RateLimit{
			Rate:   int64(res.Limit.Rate),
			Period: res.Limit.Period,
		},
		Allowed:    int64(res.Allowed),
		Remaining:  int64(res.Remaining),
		RetryAfter: res.RetryAfter,
	}, nil
}

func NewPublicRateLimiter(cfg Config) func(next http.Handler) http.Handler {
	const _DefaultRPM = 120

	if !cfg.RateLimiter.Enabled {
		return func(h http.Handler) http.Handler {
			return h
		}
	}

	limitCounter, _ := httprateredis.NewRedisLimitCounter(&httprateredis.Config{
		Host:      cfg.Redis.Host,
		Port:      cfg.Redis.Port,
		MaxIdle:   cfg.Redis.MaxIdle,
		MaxActive: cfg.Redis.MaxActive,
		DBIndex:   cfg.Redis.DBIndex,
	})

	rpm := _DefaultRPM
	if cfg.RateLimiter.PublicRequestsPerMinute > 0 {
		rpm = cfg.RateLimiter.PublicRequestsPerMinute
	}

	err := proto.ErrLimitExceeded
	if cfg.RateLimiter.ErrorMessage != "" {
		err.Message = cfg.RateLimiter.ErrorMessage
	}

	options := []httprate.Option{
		httprate.WithKeyFuncs(httprate.KeyByRealIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			proto.RespondWithError(w, err)
		}),
		httprate.WithLimitCounter(limitCounter),
	}
	return func(h http.Handler) http.Handler {
		rl := httprate.Limit(rpm, time.Minute, options...)(h)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if middleware.GetAccessQuota(ctx) == nil && !middleware.IsSkipRateLimit(ctx) {
				rl.ServeHTTP(w, r)
				return
			}
			h.ServeHTTP(w, r)
		})
	}
}
