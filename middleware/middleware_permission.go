package middleware

import (
	"log/slog"
	"net/http"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/logger"
)

// EnsurePermission middleware that checks if the session type has the required permission.
func EnsurePermission(client Client, minPermission proto.UserPermission, o *Options) func(next http.Handler) http.Handler {
	eh := errHandler
	if o != nil && o.ErrHandler != nil {
		eh = o.ErrHandler
	}

	logger := logger.NewLogger(logger.LogLevel_INFO)
	if o != nil && o.Logger != nil {
		logger = o.Logger
	}
	logger = logger.With(slog.String("middleware", "ensurePermission"))

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
