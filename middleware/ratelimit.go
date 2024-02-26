package middleware

import (
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/ratelimit"
	"github.com/go-chi/httprate"
)

func NewRateLimit(rl ratelimit.Settings, window time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if GetAccessQuota(ctx) != nil || IsSkipRateLimit(ctx) {
				next.ServeHTTP(w, r)
				return
			}

			limit, options, ok := rl.GetRateLimit(GetRateLimitType(ctx))
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			httprate.NewRateLimiter(int(limit), window, options...).Handler(next).ServeHTTP(w, r)
		})
	}
}
