package middleware

import (
	"cmp"
	"context"
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

func (r RLConfig) getRateLimit(ctx context.Context) int {
	if _, ok := GetService(ctx); ok {
		return r.ServiceRPM
	}
	if q, ok := GetAccessQuota(ctx); ok {
		return int(q.Limit.RateLimit)
	}
	if _, ok := GetAccount(ctx); ok {
		return r.AccountRPM
	}
	return r.PublicRPM
}

func RateLimit(rlCfg RLConfig, redisCfg redis.Config) func(next http.Handler) http.Handler {
	if !rlCfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	rlCfg.PublicRPM = cmp.Or(rlCfg.PublicRPM, 1000)
	rlCfg.AccountRPM = cmp.Or(rlCfg.AccountRPM, 4000)
	rlCfg.ServiceRPM = cmp.Or(rlCfg.ServiceRPM, 0)

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
			if account, ok := GetAccount(ctx); ok {
				return AccountRateKey(account), nil
			}
			if _, ok := GetService(ctx); ok {
				return "", nil
			}
			return httprate.KeyByRealIP(r)
		}),
		httprate.WithLimitHandler(proto.ErrLimitExceeded.WithMessage(rlCfg.ErrorMsg).Handler),
	}

	limiter := httprate.NewRateLimiter(rlCfg.PublicRPM, _RateLimitWindow, options...)

	// The rate limiter middleware
	return func(next http.Handler) http.Handler {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			swapHeader(h, "X-RateLimit-Limit", HeaderQuotaRateLimit)
			swapHeader(h, "X-RateLimit-Remaining", HeaderQuotaRateRemaining)
			swapHeader(h, "X-RateLimit-Reset", HeaderQuotaRateReset)
			next.ServeHTTP(w, r)
		})
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = httprate.WithRequestLimit(ctx, rlCfg.getRateLimit(ctx))
			limiter.Handler(handler).ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// swapHeader swaps the header from one key to another.
func swapHeader(h http.Header, from, to string) {
	if v := h.Get(from); v != "" {
		h.Set(to, v)
		h.Del(from)
	}
}
