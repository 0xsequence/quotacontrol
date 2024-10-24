package middleware

import (
	"net/http"

	"github.com/0xsequence/authcontrol"
)

// SetCost middleware that sets the cost of the request.
func SetCost(base int64, cost authcontrol.Config[int64]) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			credits, err := cost.Get(r.URL.Path)
			if err != nil {
				credits = base
			}

			ctx = WithCost(ctx, credits)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
