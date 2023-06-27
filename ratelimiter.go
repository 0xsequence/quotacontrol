package quotacontrol

import (
	"context"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

// RateLimit is a representation of a time constrain (rate/duration).
type RateLimit struct {
	Rate   int64
	Period time.Duration
}

// Result is the result of a rate limit.
type Result struct {
	Limit      RateLimit
	Allowed    int64
	Remaining  int64
	RetryAfter time.Duration
	ResetAfter time.Duration
}

// RateLimiter leaves to the client how to define the key.
type RateLimiter interface {
	RateLimit(ctx context.Context, key string, computeUnits int, limit RateLimit) (*Result, error)
}

// NewRateLimiter returns a new redis backed rate limiter.
func NewRateLimiter(client *redis.Client) RateLimiter {
	return &redisRateLimit{client: redis_rate.NewLimiter(client)}
}

type redisRateLimit struct {
	client *redis_rate.Limiter
}

func (r *redisRateLimit) RateLimit(ctx context.Context, key string, computeUnits int, l RateLimit) (*Result, error) {
	res, err := r.client.AllowN(ctx, key, redis_rate.Limit{Rate: int(l.Rate), Period: l.Period, Burst: int(l.Rate)}, computeUnits)
	if err != nil {
		return nil, err
	}
	return &Result{
		Limit: RateLimit{
			Rate:   int64(res.Limit.Rate),
			Period: res.Limit.Period,
		},
		Allowed:    int64(res.Allowed),
		Remaining:  int64(res.Remaining),
		RetryAfter: res.RetryAfter,
	}, nil
}
