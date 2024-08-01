package middleware

import (
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
)

type RateLimiterConfig struct {
	Enabled                  bool   `toml:"enabled"`
	PublicRequestsPerMinute  int    `toml:"public_requests_per_minute"`
	UserRequestsPerMinute    int    `toml:"user_requests_per_minute"`
	ServiceRequestsPerMinute int    `toml:"service_requests_per_minute"`
	ErrorMessage             string `toml:"error_message"`
}

func RateLimit(config RateLimiterConfig, limitCounter httprate.LimitCounter, eh ErrorHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = _DefaultErrorHandler
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			quota, cu := GetAccessQuota(ctx), GetComputeUnits(ctx)

			if quota == nil || cu == 0 {
				next.ServeHTTP(w, r)
				return
			}

			limitErr := proto.ErrLimitExceeded
			if config.ErrorMessage != "" {
				limitErr.Message = config.ErrorMessage
			}

			ctx = httprate.WithIncrement(ctx, int(cu))
			options := []httprate.Option{
				httprate.WithKeyFuncs(httprateKey),
				httprate.WithLimitCounter(limitCounter),
				httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
					eh(w, r, nil, limitErr)
				}),
			}
			limiter := httprate.NewRateLimiter(int(quota.Limit.RateLimit), time.Minute, options...)
			limiter.Handler(next).ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
