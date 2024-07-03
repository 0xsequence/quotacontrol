package middleware

import (
	"cmp"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/goware/cachestore/redis"
)

const _RateLimitWindow = 1 * time.Minute

type RLConfig struct {
	Enabled    bool   `toml:"enabled"`
	PublicRPM  int    `toml:"public_rpm"`
	AccountRPM int    `toml:"account_rpm"`
	ServiceRPM int    `toml:"service_rpm"`
	ErrorMsg   string `toml:"error_message"`
}

func RateLimit(rlCfg RLConfig, redisCfg redis.Config) func(next http.Handler) http.Handler {
	if !rlCfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	defaultRPM := cmp.Or(rlCfg.PublicRPM, 120)
	accountRPM := cmp.Or(rlCfg.AccountRPM, 4000)
	serviceRPM := cmp.Or(rlCfg.ServiceRPM, 0)

	var limitCounter httprate.LimitCounter
	if redisCfg.Enabled {
		limitCounter, _ = httprateredis.NewRedisLimitCounter(&httprateredis.Config{
			Host:      redisCfg.Host,
			Port:      redisCfg.Port,
			MaxIdle:   redisCfg.MaxIdle,
			MaxActive: redisCfg.MaxActive,
			DBIndex:   redisCfg.DBIndex,
		})
	}

	options := []httprate.Option{
		httprate.WithLimitCounter(limitCounter),
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			ctx := r.Context()
			if q, ok := GetAccessQuota(ctx); ok {
				return ProjectRateKey(q.GetProjectID()), nil
			}
			if account := GetAccount(ctx); account != "" {
				return AccountRateKey(account), nil
			}
			if service := GetService(ctx); service != "" {
				return "", nil
			}
			return httprate.KeyByRealIP(r)
		}),
		httprate.WithLimitHandler(proto.ErrLimitExceeded.WithMessage(rlCfg.ErrorMsg).Handler),
	}

	limiter := httprate.NewRateLimiter(defaultRPM, _RateLimitWindow, options...)

	// The rate limiter middleware
	return func(next http.Handler) http.Handler {
		return rateLimit{
			AccountRPM: accountRPM,
			ServiceRPM: serviceRPM,
			Next:       limiter.Handler(next),
		}
	}
}

type rateLimit struct {
	AccountRPM int
	ServiceRPM int
	Next       http.Handler
}

func (m rateLimit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if q, ok := GetAccessQuota(ctx); ok {
		ctx = httprate.WithRequestLimit(ctx, int(q.Limit.RateLimit))
	}
	if account := GetAccount(ctx); account != "" {
		ctx = httprate.WithRequestLimit(ctx, m.AccountRPM)
	}
	if service := GetService(ctx); service != "" {
		ctx = httprate.WithRequestLimit(ctx, m.ServiceRPM)
	}

	m.Next.ServeHTTP(w, r.WithContext(ctx))
}
