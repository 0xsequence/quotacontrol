package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
)

const _RateLimitWindow = 1 * time.Minute

// RateLimitFunc is a function that returns the rate limit key and rate per minute for a given request.
type RateLimitFunc func(r *http.Request) (key string, rpm *int)

func RateLimit(limitCounter httprate.LimitCounter, defaultRPM int, keyFn RateLimitFunc, errLimit error) func(next http.Handler) http.Handler {
	if errLimit == nil {
		errLimit = proto.ErrLimitExceeded
	}

	if keyFn == nil {
		keyFn = func(r *http.Request) (key string, rpm *int) {
			key, _ = httprate.KeyByRealIP(r)
			return key, nil
		}
	}

	fn := func(r *http.Request) (key string, rpm *int) {
		if q := GetAccessQuota(r.Context()); q != nil {
			return ProjectRateKey(q.GetProjectID()), proto.Ptr(int(q.Limit.RateLimit))
		}
		return keyFn(r)
	}

	options := []httprate.Option{
		httprate.WithLimitCounter(limitCounter),
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			if q := GetAccessQuota(r.Context()); q != nil {
				return ProjectRateKey(q.GetProjectID()), nil
			}
			key, _ := fn(r)
			return key, nil
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

			if _, rpm := fn(r); rpm != nil {
				if *rpm == 0 {
					next.ServeHTTP(w, r)
					return
				}
				ctx = httprate.WithRequestLimit(ctx, *rpm)
			}

			limiter.Handler(next).ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ProjectRateKey(projectID uint64) string {
	return fmt.Sprintf("rl:project:%d", projectID)
}
