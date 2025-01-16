package middleware

import (
	"net/http"

	"github.com/0xsequence/authcontrol"
)

// SetCost middleware that sets the cost of the request, and defaults to Option.BaseRequestCost.
func SetCost(cfg authcontrol.Config[int64], o Options) func(next http.Handler) http.Handler {
	o.ApplyDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			cost := int64(o.BaseRequestCost)
			if v, err := cfg.Get(ctx, r.URL.Path); err == nil {
				cost = v
			}

			ctx = WithCost(ctx, cost)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
