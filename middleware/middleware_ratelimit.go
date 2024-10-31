package middleware

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"

	"github.com/0xsequence/authcontrol"
)

const (
	HeaderCreditsRemaining = "Credits-Rate-Remaining"
	HeaderCreditsLimit     = "Credits-Rate-Limit"
	HeaderCreditsReset     = "Credits-Rate-Reset"
	HeaderCreditsCost      = "Credits-Rate-Cost"
	HeaderRetryAfter       = "Retry-After"
)

const _RateLimitWindow = 1 * time.Minute

const (
	DefaultPublicRate  = 3000
	DefaultAccountRate = 6000
	DefaultServiceRate = 0
)

// RateLimitConfig is the configuration for the rate limiter middleware.
type RateLimitConfig struct {
	// Enabled turns on/off the middleware.
	Enabled bool `toml:"enabled"`
	// PublicRPM is the rate limit for Public sessions expressed as number of requests per minute.
	PublicRPM int `toml:"public_requests_per_minute"`
	// AccountRPM is the rate limit for Account sessions expressed as number of requests per minute.
	AccountRPM int `toml:"user_requests_per_minute"`
	// ServiceRPM is the rate limit for Service sessions expressed as number of requests per minute.
	ServiceRPM int `toml:"service_requests_per_minute"`
}

func (r RateLimitConfig) GetRateLimit(ctx context.Context, baseRequestCost int) int {
	if _, ok := authcontrol.GetService(ctx); ok {
		return r.ServiceRPM * baseRequestCost
	}
	if q, ok := GetAccessQuota(ctx); ok {
		return int(q.Limit.RateLimit)
	}
	if _, ok := authcontrol.GetAccount(ctx); ok {
		return r.AccountRPM * baseRequestCost
	}
	return r.PublicRPM * baseRequestCost
}

func RateLimit(cfg RateLimitConfig, counter httprate.LimitCounter, o Options) func(next http.Handler) http.Handler {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	o.ApplyDefaults()

	cfg.PublicRPM = cmp.Or(cfg.PublicRPM, DefaultPublicRate)
	cfg.AccountRPM = cmp.Or(cfg.AccountRPM, DefaultAccountRate)
	cfg.ServiceRPM = cmp.Or(cfg.ServiceRPM, DefaultServiceRate)

	options := []httprate.Option{
		httprate.WithLimitCounter(counter),
		httprate.WithResponseHeaders(httprate.ResponseHeaders{
			Limit:      HeaderCreditsLimit,
			Remaining:  HeaderCreditsRemaining,
			Increment:  HeaderCreditsCost,
			Reset:      HeaderCreditsReset,
			RetryAfter: HeaderRetryAfter,
		}),
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			ctx := r.Context()
			if _, ok := authcontrol.GetService(ctx); ok {
				return "", nil
			}
			if project, ok := GetProjectID(ctx); ok {
				return ProjectRateKey(project), nil
			}
			if q, ok := GetAccessQuota(ctx); ok {
				return ProjectRateKey(q.GetProjectID()), nil
			}
			if account, ok := authcontrol.GetAccount(ctx); ok {
				return AccountRateKey(account), nil
			}
			return httprate.KeyByRealIP(r)
		}),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			session, _ := authcontrol.GetSessionType(ctx)
			msg := fmt.Sprintf("%s (%s session)", proto.ErrRateLimit.Message, session)
			o.ErrHandler(r, w, proto.ErrRateLimit.WithMessage(msg))
		}),
	}

	limiter := httprate.NewRateLimiter(cfg.PublicRPM, _RateLimitWindow, options...)

	// The rate limiter middleware
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if limit := cfg.GetRateLimit(ctx, o.BaseRequestCost); limit > 0 {
				ctx = httprate.WithRequestLimit(ctx, limit)
				limiter.Handler(next).ServeHTTP(w, r.WithContext(ctx))
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

func ProjectRateKey(projectID uint64) string {
	return fmt.Sprintf("rl:project:%d", projectID)
}

func AccountRateKey(account string) string {
	return fmt.Sprintf("rl:account:%s", account)
}
