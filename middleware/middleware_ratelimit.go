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

type RLConfig struct {
	Enabled     bool `toml:"enabled"`
	PublicRate  int  `toml:"public_requests_per_minute"`
	AccountRate int  `toml:"user_requests_per_minute"`
	ServiceRate int  `toml:"service_requests_per_minute"`
}

func (r RLConfig) getRateLimit(ctx context.Context) int {
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

func RateLimit(rlCfg RLConfig, counter httprate.LimitCounter, o *Options) func(next http.Handler) http.Handler {
	if !rlCfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	eh := errHandler
	if o != nil && o.ErrHandler != nil {
		eh = o.ErrHandler
	}

	logger := slog.Default()
	if o != nil && o.Logger != nil {
		logger = o.Logger
	}
	logger = logger.With(slog.String("middleware", "ratelimit"))

	rlCfg.PublicRate = cmp.Or(rlCfg.PublicRate, DefaultPublicRate)
	rlCfg.AccountRate = cmp.Or(rlCfg.AccountRate, DefaultAccountRate)
	rlCfg.ServiceRate = cmp.Or(rlCfg.ServiceRate, DefaultServiceRate)

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

	limiter := httprate.NewRateLimiter(rlCfg.PublicRate, _RateLimitWindow, options...)

	// The rate limiter middleware
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if limit := rlCfg.getRateLimit(ctx); limit > 0 {
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
