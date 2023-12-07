package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
)

// RateLimit is a middleware that limits the number of requests per second.
// Accepts a limit counter, defaults to in-memory. Accepts a rate detector, defaults KeyByRealIP.
func RateLimit(lc httprate.LimitCounter, rd RateDetector, cfg *RateConfig, eh ErrorHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = _DefaultErrorHandler
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			cost := GetComputeUnits(ctx)

			if cost == 0 {
				next.ServeHTTP(w, r)
				return
			}
			ctx = httprate.WithIncrement(ctx, int(cost))

			key, rate := "", cfg.getRate()
			if rd != nil {
				key, rate = rd.DetectRate(r)
			}
			if rate < 0 {
				next.ServeHTTP(w, r)
				return
			}
			ctx = withRateKey(ctx, key)

			options := []httprate.Option{
				httprate.WithKeyFuncs(rateKeyFunc),
				httprate.WithLimitHandler(eh.handler(next, cfg.getError())),
			}
			if lc != nil {
				options = append(options, httprate.WithLimitCounter(lc))
			}
			limiter := httprate.NewRateLimiter(rate, cfg.getInterval(), options...)
			limiter.Handler(next).ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// rateKeyFunc returns the key for the rate limiter.
// It prioritizes the AccessQuota key, then the rate limit key.
func rateKeyFunc(r *http.Request) (string, error) {
	ctx := r.Context()
	if q := GetAccessQuota(ctx); q != nil {
		return fmt.Sprintf("project:%d", q.AccessKey.ProjectID), nil
	}
	if key := getRateKey(ctx); key != "" {
		return key, nil
	}
	return httprate.KeyByRealIP(r)
}

// RateDetector retuns key and rate for requests without AccessQuota.
type RateDetector interface {
	DetectRate(r *http.Request) (key string, limit int)
}

// RateConfig is the configuration for the rate limit middleware.
type RateConfig struct {
	Rate     int
	Interval time.Duration

	ErrorMessage string
}

// getRate returns the rate limit, if present, or the default value (120).
func (c *RateConfig) getRate() int {
	if c == nil || c.Rate == 0 {
		return 120
	}
	return c.Rate
}

// getInterval returns the interval, if present, or the default value (1m).
func (c *RateConfig) getInterval() time.Duration {
	if c == nil || c.Rate == 0 {
		return time.Minute
	}
	return c.Interval
}

// getError returns ErrLimitExceeded with custom error message, if present.
func (c *RateConfig) getError() error {
	err := proto.ErrLimitExceeded
	if c != nil && c.ErrorMessage != "" {
		err.Message = c.ErrorMessage
	}
	return err
}
