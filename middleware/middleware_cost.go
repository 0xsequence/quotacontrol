package middleware

import (
	"log/slog"
	"net/http"

	"github.com/0xsequence/authcontrol"
)

// SetCost middleware that sets the cost of the request, and defaults to Option.BaseRequestCost.
func SetCost(cost authcontrol.Config[int64], o Options) func(next http.Handler) http.Handler {
	o.ApplyDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			cost, err := cost.Get(ctx, r.URL.Path)
			if err != nil {
				if o.Logger != nil {
					o.Logger.Error("get cost", slog.Any("error", err))
				}
				cost = int64(o.BaseRequestCost)
			}

			ctx = WithCost(ctx, cost)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
