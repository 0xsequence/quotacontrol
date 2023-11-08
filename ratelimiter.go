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
		return func(next http.Handler) http.Handler {
			return next
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

	return func(next http.Handler) http.Handler {
		rl := httprate.Limit(rpm, time.Minute, options...)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if middleware.GetAccessQuota(ctx) == nil && !middleware.IsSkipRateLimit(ctx) {
				rl.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func NewHTTPRateLimiter(cfg Config, vary RateLimitVaryFn) func(next http.Handler) http.Handler {
	// Short-cut the middleware if the rate limiter is disabled
	if !cfg.RateLimiter.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// httprate limit counter
	const _DefaultRPM = 120

	limitCounter, _ := httprateredis.NewRedisLimitCounter(&httprateredis.Config{
		Host:      cfg.Redis.Host,
		Port:      cfg.Redis.Port,
		MaxIdle:   cfg.Redis.MaxIdle,
		MaxActive: cfg.Redis.MaxActive,
		DBIndex:   cfg.Redis.DBIndex,
	})

	limitErr := proto.ErrLimitExceeded
	if cfg.RateLimiter.ErrorMessage != "" {
		limitErr.Message = cfg.RateLimiter.ErrorMessage
	}

	// Public rate limiter
	rpmPublic := _DefaultRPM
	if cfg.RateLimiter.PublicRequestsPerMinute != 0 {
		rpmPublic = cfg.RateLimiter.PublicRequestsPerMinute
	}
	optsPublic := []httprate.Option{
		httprate.WithKeyFuncs(httprate.KeyByRealIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			proto.RespondWithError(w, limitErr)
		}),
		httprate.WithLimitCounter(limitCounter),
	}

	// User rate limiter
	rpmUser := _DefaultRPM
	if cfg.RateLimiter.UserRequestsPerMinute != 0 {
		rpmUser = cfg.RateLimiter.UserRequestsPerMinute
	}
	optsUser := []httprate.Option{
		httprate.WithKeyFuncs(ratelimitKeyFunc),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			proto.RespondWithError(w, limitErr)
		}),
		httprate.WithLimitCounter(limitCounter),
	}

	// Service rate limiter
	rpmService := _DefaultRPM
	if cfg.RateLimiter.ServiceRequestsPerMinute != 0 {
		rpmService = cfg.RateLimiter.ServiceRequestsPerMinute
	}
	optsService := []httprate.Option{
		httprate.WithKeyFuncs(ratelimitKeyFunc),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			proto.RespondWithError(w, limitErr)
		}),
		httprate.WithLimitCounter(limitCounter),
	}

	// The rate limiter middleware
	return func(next http.Handler) http.Handler {
		var rlPublic, rlUser, rlService http.Handler = next, next, next
		if rpmPublic > 0 {
			rlPublic = httprate.Limit(rpmPublic, time.Minute, optsPublic...)(next)
		}
		if rpmUser > 0 {
			rlUser = httprate.Limit(rpmUser, time.Minute, optsUser...)(next)
		}
		if rpmService > 0 {
			rlService = httprate.Limit(rpmService, time.Minute, optsService...)(next)
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// skip rate limiter in case of quota system is in use, or if the request
			// is marked as skip rate limit
			if middleware.GetAccessQuota(ctx) != nil || middleware.IsSkipRateLimit(ctx) {
				next.ServeHTTP(w, r)
				return
			}

			// Rate limit
			rateLimitType, rlKey := vary(r)
			if rlKey != "" {
				r = r.WithContext(context.WithValue(r.Context(), ctxRateLimitKey, rlKey))
			}

			switch rateLimitType {
			case RateLimitType_Public:
				rlPublic.ServeHTTP(w, r)
			case RateLimitType_User:
				rlUser.ServeHTTP(w, r)
			case RateLimitType_Service:
				rlService.ServeHTTP(w, r)
			default:
				// RateLimitType_None
				next.ServeHTTP(w, r)
			}
		})
	}
}

type RateLimitVaryFn func(r *http.Request) (RateLimitType, string)

type RateLimitType uint16

const (
	RateLimitType_Public RateLimitType = iota
	RateLimitType_User
	RateLimitType_Service
	RateLimitType_None
)

func ratelimitKeyFunc(r *http.Request) (string, error) {
	rlKey, _ := r.Context().Value(ctxRateLimitKey).(string)
	return rlKey, nil
}

var ctxRateLimitKey = &contextKey{"rateLimitKey"}

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "quotacontrol/ratelimiter context value " + k.name
}
