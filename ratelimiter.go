package quotacontrol

import (
	"net/http"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
)

func NewRateLimiter(cfg Config, keyFn httprate.KeyFunc) func(next http.Handler) http.Handler {
	// Short-cut the middleware if the rate limiter is disabled
	if !cfg.RateLimiter.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// httprate limit counter
	rpm := 120
	if cfg.RateLimiter.DefaultRPM != 0 {
		rpm = cfg.RateLimiter.DefaultRPM
	}

	counter, _ := httprateredis.NewRedisLimitCounter(cfg.RateLimitCfg())

	limitErr := proto.ErrLimitExceeded
	if cfg.RateLimiter.ErrorMsg != "" {
		limitErr.Message = cfg.RateLimiter.ErrorMsg
	}

	return middleware.RateLimit(counter, rpm, keyFn, limitErr)
}
