package middleware

import (
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
)

const _RateLimitWindow = 1 * time.Minute

func RateLimit(limitCounter httprate.LimitCounter, defaultRPM int, keyFn httprate.KeyFunc, errLimit error) func(next http.Handler) http.Handler {
	if keyFn == nil {
		keyFn = httprate.KeyByRealIP
	}

	if errLimit == nil {
		errLimit = proto.ErrLimitExceeded
	}

	options := []httprate.Option{
		httprate.WithLimitCounter(limitCounter),
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			if q := GetAccessQuota(r.Context()); q != nil {
				return ProjectRateKey(q.GetProjectID()), nil
			}
			return keyFn(r)
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

			if rpm, ok := getRateLimit(ctx); ok {
				if rpm == 0 {
					next.ServeHTTP(w, r)
					return
				}
				ctx = httprate.WithRequestLimit(ctx, rpm)
			}

			limiter.Handler(next).ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
