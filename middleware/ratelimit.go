package middleware

import (
	"fmt"
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

	// httprate limit counter
	defaultRPM := 120
	if rlCfg.PublicRPM != 0 {
		defaultRPM = rlCfg.PublicRPM
	}
	accountRPM := 4000
	if rlCfg.AccountRPM != 0 {
		accountRPM = rlCfg.AccountRPM
	}
	serviceRPM := 0
	if rlCfg.ServiceRPM != 0 {
		serviceRPM = rlCfg.ServiceRPM
	}

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

	errLimit := proto.ErrLimitExceeded
	if rlCfg.ErrorMsg != "" {
		errLimit.Message = rlCfg.ErrorMsg
	}

	options := []httprate.Option{
		httprate.WithLimitCounter(limitCounter),
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			if q := GetAccessQuota(r.Context()); q != nil {
				return ProjectRateKey(q.GetProjectID()), nil
			}
			if account := GetAccount(r.Context()); account != "" {
				return AccountRateKey(account), nil
			}
			if service := GetService(r.Context()); service != "" {
				return "", nil
			}
			return httprate.KeyByRealIP(r)
		}),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			proto.RespondWithError(w, errLimit)
		}),
	}

	limiter := httprate.NewRateLimiter(defaultRPM, _RateLimitWindow, options...)

	// The rate limiter middleware
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if q := GetAccessQuota(r.Context()); q != nil {
				ctx = httprate.WithRequestLimit(ctx, int(q.Limit.RateLimit))
			}
			if account := GetAccount(r.Context()); account != "" {
				ctx = httprate.WithRequestLimit(ctx, accountRPM)
			}
			if service := GetService(r.Context()); service != "" {
				ctx = httprate.WithRequestLimit(ctx, serviceRPM)
			}

			limiter.Handler(next).ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ProjectRateKey(projectID uint64) string {
	return fmt.Sprintf("rl:project:%d", projectID)
}

func AccountRateKey(account string) string {
	return fmt.Sprintf("rl:account:%s", account)
}
