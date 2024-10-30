package middleware

import (
	"log/slog"
	"net/http"

	"github.com/0xsequence/authcontrol"
	"github.com/goware/logger"
)

// SetCost middleware that sets the cost of the request, and defaults to Option.BaseRequestCost.
func SetCost(cost authcontrol.Config[int64], o *Options) func(next http.Handler) http.Handler {
	baseRequestCost := int64(1)
	if o != nil {
		baseRequestCost = max(baseRequestCost, int64(o.BaseRequestCost))
	}

	logger := logger.NewLogger(logger.LogLevel_INFO)
	if o != nil && o.Logger != nil {
		logger = o.Logger
	}
	logger = logger.With(slog.String("middleware", "setCost"))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			cost, err := cost.Get(r.URL.Path)
			if err != nil {
				if o != nil && o.Logger != nil {
					logger.With(slog.Any("error", err)).Error("get cost")
				}
				cost = baseRequestCost
			}

			ctx = WithCost(ctx, cost)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
