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

func (f RateLimitFunc) KeyFunc(r *http.Request) (string, error) {
	key, _ := f(r)
	return key, nil
}

func RateLimit(limitCounter httprate.LimitCounter, defaultRPM, acccountRPM, serviceRPM int, errLimit error) func(next http.Handler) http.Handler {
	if errLimit == nil {
		errLimit = proto.ErrLimitExceeded
	}

	fn := RateLimitFunc(func(r *http.Request) (key string, rpm *int) {
		if q := GetAccessQuota(r.Context()); q != nil {
			return ProjectRateKey(q.GetProjectID()), proto.Ptr(int(q.Limit.RateLimit))
		}
		if account := GetAccount(r.Context()); account != "" {
			return AccountRateKey(account), proto.Ptr(acccountRPM)
		}
		if service := GetService(r.Context()); service != "" {
			return "", proto.Ptr(0)
		}
		key, _ = httprate.KeyByRealIP(r)
		return key, nil
	})

	options := []httprate.Option{
		httprate.WithLimitCounter(limitCounter),
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			if q := GetAccessQuota(r.Context()); q != nil {
				return ProjectRateKey(q.GetProjectID()), nil
			}
			return fn.KeyFunc(r)
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

func AccountRateKey(account string) string {
	return fmt.Sprintf("rl:account:%s", account)
}

type RateLimiterCfg interface {
	GetKey(r *http.Request) string
	GetRate(r *http.Request) *int
}

func NewRateLimitFunc(rl RateLimiterCfg) RateLimitFunc {
	return func(r *http.Request) (key string, rpm *int) {
		return rl.GetKey(r), rl.GetRate(r)
	}
}
