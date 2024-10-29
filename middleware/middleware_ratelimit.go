package middleware

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/0xsequence/authcontrol"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	"github.com/goware/logger"
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

type RateLimitConfig struct {
	Enabled     bool `toml:"enabled"`
	PublicRate  int  `toml:"public_requests_per_minute"`
	AccountRate int  `toml:"user_requests_per_minute"`
	ServiceRate int  `toml:"service_requests_per_minute"`
}

func (r RateLimitConfig) getRateLimit(ctx context.Context) int {
	if _, ok := authcontrol.GetService(ctx); ok {
		return r.ServiceRate
	}
	if q, ok := GetAccessQuota(ctx); ok {
		return int(q.Limit.RateLimit)
	}
	if _, ok := authcontrol.GetAccount(ctx); ok {
		return r.AccountRate
	}
	return r.PublicRate
}

func RateLimit(cfg RateLimitConfig, counter httprate.LimitCounter, o *Options) func(next http.Handler) http.Handler {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	eh := errHandler
	if o != nil && o.ErrHandler != nil {
		eh = o.ErrHandler
	}

	logger := logger.NewLogger(logger.LogLevel_INFO)
	if o != nil && o.Logger != nil {
		logger = o.Logger
	}
	logger = logger.With(slog.String("middleware", "rateLimit"))

	cfg.PublicRate = cmp.Or(cfg.PublicRate, DefaultPublicRate)
	cfg.AccountRate = cmp.Or(cfg.AccountRate, DefaultAccountRate)
	cfg.ServiceRate = cmp.Or(cfg.ServiceRate, DefaultServiceRate)

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
			msg := fmt.Sprintf("%s for %s session", proto.ErrRateLimit.Message, session)
			eh(r, w, proto.ErrRateLimit.WithMessage(msg))
		}),
	}

	limiter := httprate.NewRateLimiter(cfg.PublicRate, _RateLimitWindow, options...)

	// The rate limiter middleware
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if limit := cfg.getRateLimit(ctx); limit > 0 {
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
