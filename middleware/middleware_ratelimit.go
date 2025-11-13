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
	HeaderRateRemaining = "Credits-Rate-Remaining"
	HeaderRateLimit     = "Credits-Rate-Limit"
	HeaderRateReset     = "Credits-Rate-Reset"
	HeaderRateCost      = "Credits-Rate-Cost"
	HeaderRetryAfter    = "Retry-After"
)

const rateLimitWindow = 1 * time.Minute

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

// RateLimit is a middleware that limits the number of requests per minute.
func RateLimit(client Client, cfg RateLimitConfig, counter httprate.LimitCounter, o Options) func(next http.Handler) http.Handler {
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
			Limit:      HeaderRateLimit,
			Remaining:  HeaderRateRemaining,
			Increment:  HeaderRateCost,
			Reset:      HeaderRateReset,
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
			return PublicRateKey(r)
		}),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			err := proto.ErrRateLimited
			if _, ok := GetAccessQuota(r.Context()); ok {
				err = proto.ErrQuotaRateLimit
			}
			o.ErrHandler(r, w, err)
		}),
	}

	limiter := httprate.NewRateLimiter(cfg.PublicRPM, rateLimitWindow, options...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			svc := client.GetService()

			// if the rate limit is 0 or less, skip the rate limiter
			limit, ok := getRateLimit(ctx, cfg, svc, o.BaseRequestCost)
			if !ok {
				o.ErrHandler(r, w, proto.ErrAborted.WithCausef("rate limit not found for service %s", svc.GetName()))
				return
			}
			if limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			// if the cost is set to 0, skip the rate limiter
			if cost, ok := getCost(ctx); ok && cost == 0 {
				next.ServeHTTP(w, r)
				return
			}

			ctx = httprate.WithRequestLimit(ctx, limit)
			limiter.Handler(next).ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func PublicRateKey(r *http.Request) (string, error) {
	return httprate.KeyByRealIP(r)
}

func ProjectRateKey(projectID uint64) string {
	return fmt.Sprintf("rl:project:%d", projectID)
}

func AccountRateKey(account string) string {
	return fmt.Sprintf("rl:account:%s", account)
}

func getRateLimit(ctx context.Context, r RateLimitConfig, svc proto.Service, baseRequestCost int) (int, bool) {
	// service has highest priority; service session + access key = access key quota + service rate limit
	if _, ok := authcontrol.GetService(ctx); ok {
		return r.ServiceRPM * baseRequestCost, true
	}
	if q, ok := GetAccessQuota(ctx); ok {
		cfg, ok := q.Limit.GetSettings(svc)
		if !ok {
			return 0, false
		}
		return int(cfg.RateLimit) * baseRequestCost, true
	}
	if _, ok := authcontrol.GetAccount(ctx); ok {
		return r.AccountRPM * baseRequestCost, true
	}
	return r.PublicRPM * baseRequestCost, true
}
