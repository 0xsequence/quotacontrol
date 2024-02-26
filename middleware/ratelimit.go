package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

var (
	ErrAlreadyRegistered = errors.New("ratelimit: already registered")
)

type RateType string

type RateLimitSettings struct {
	counter       httprate.LimitCounter
	commonOptions []httprate.Option
	typeSettings  map[RateType]rateLimit
}

type rateLimit struct {
	Limit   int64
	Options []httprate.Option
}

func NewRateLimitSettings(counter httprate.LimitCounter, commonOptions ...httprate.Option) RateLimitSettings {
	return RateLimitSettings{
		counter:       counter,
		commonOptions: commonOptions,
		typeSettings:  make(map[RateType]rateLimit),
	}
}

func (rl *RateLimitSettings) RegisterRateLimit(rateType RateType, limit int64, options ...httprate.Option) error {
	if _, ok := rl.typeSettings[rateType]; ok {
		return ErrAlreadyRegistered
	}

	rl.typeSettings[rateType] = rateLimit{
		Limit:   limit,
		Options: options,
	}
	return nil
}

func (rl *RateLimitSettings) GetRateLimit(rateType RateType) (int64, []httprate.Option, bool) {
	limiter, ok := rl.typeSettings[rateType]
	if !ok {
		return 0, nil, false
	}

	options := make([]httprate.Option, 1, len(rl.commonOptions)+len(limiter.Options))
	options[0] = httprate.WithLimitCounter(rl.counter)
	options = append(options, rl.commonOptions...)
	options = append(options, limiter.Options...)

	return limiter.Limit, options, true
}

func NewRateLimit(rl RateLimitSettings, window time.Duration) func(next http.Handler) http.Handler {
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
