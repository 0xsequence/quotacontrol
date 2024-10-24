package middleware

import (
	"net/http"

	"github.com/0xsequence/authcontrol"
	"github.com/0xsequence/quotacontrol/proto"
)

// EnsurePermission middleware that checks if the session type has the required permission.
func EnsurePermission(client Client, minPermission proto.UserPermission, eh authcontrol.ErrHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = errHandler
	}

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
				eh(r, w, proto.ErrUnauthorizedUser)
				return
			}

			ok, err := client.CheckPermission(ctx, q.GetProjectID(), minPermission)
			if err != nil {
				eh(r, w, proto.ErrUnauthorized.WithCause(err))
				return
			}
			if !ok {
				eh(r, w, proto.ErrUnauthorizedUser)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
