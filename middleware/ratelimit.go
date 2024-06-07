package middleware

import (
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
)

func RateLimit(limitCounter httprate.LimitCounter, defaultRPM int, keyFn httprate.KeyFunc, errLimit error) func(next http.Handler) http.Handler {
	if keyFn == nil {
		keyFn = httprate.KeyByRealIP
	}

	if errLimit == nil {
		errLimit = proto.ErrLimitExceeded
	}

	// The rate limiter middleware
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			rpm, ok := getRateLimit(ctx)
			if !ok {
				rpm = defaultRPM
			}

			// if rate limit is set to 0 skip
			if rpm < 1 {
				next.ServeHTTP(w, r)
				return
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

			httprate.Limit(rpm, time.Minute, options...)(next).ServeHTTP(w, r)
		})
	}
}
