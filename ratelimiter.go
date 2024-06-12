package quotacontrol

import (
	"net/http"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
)

func RateLimiter(cfg Config, keyFn middleware.RateLimitFunc) func(next http.Handler) http.Handler {
	// Short-cut the middleware if the rate limiter is disabled
	if !cfg.RateLimiter.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// httprate limit counter
	defaultRPM := 120
	if cfg.RateLimiter.PublicRPM != 0 {
		defaultRPM = cfg.RateLimiter.PublicRPM
	}
	accountRPM := 4000
	if cfg.RateLimiter.AccountRPM != 0 {
		accountRPM = cfg.RateLimiter.AccountRPM
	}

	var counter httprate.LimitCounter
	if cfg.Redis.Enabled {
		counter, _ = httprateredis.NewRedisLimitCounter(cfg.RateLimitCfg())
	}

	limitErr := proto.ErrLimitExceeded
	if cfg.RateLimiter.ErrorMsg != "" {
		limitErr.Message = cfg.RateLimiter.ErrorMsg
	}

	return middleware.RateLimit(counter, defaultRPM, accountRPM, keyFn, limitErr)
}
