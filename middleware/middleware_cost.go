package middleware

import (
	"net/http"

	"github.com/0xsequence/authcontrol"
)

// SetCost
func SetCost(defaultCost int64, cost authcontrol.Config[int64]) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			credits := defaultCost
			if v, ok := cost.Get(authcontrol.ParseRequest(r.URL.Path)); ok {
				credits = v
			}
			ctx = WithCost(ctx, credits)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
