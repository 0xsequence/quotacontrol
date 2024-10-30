package middleware

import (
	"net/http"

	"github.com/0xsequence/quotacontrol/proto"
)

// EnsurePermission middleware that checks if the session type has the required permission.
func EnsurePermission(client Client, minPermission proto.UserPermission, o Options) func(next http.Handler) http.Handler {
	o.ApplyDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !client.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			// check if we alreayd have a project ID from the JWT
			q, ok := GetAccessQuota(ctx)
			if !ok || !q.IsJWT() {
				o.ErrHandler(r, w, proto.ErrUnauthorizedUser)
				return
			}

			ok, err := client.CheckPermission(ctx, q.GetProjectID(), minPermission)
			if err != nil {
				o.ErrHandler(r, w, proto.ErrUnauthorized.WithCause(err))
				return
			}
			if !ok {
				o.ErrHandler(r, w, proto.ErrUnauthorizedUser)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
