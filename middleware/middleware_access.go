package middleware

import (
	"net/http"

	"github.com/0xsequence/quotacontrol/proto"
)

func defaultErrHandler(r *http.Request, w http.ResponseWriter, err error) {
	proto.RespondWithError(w, err)
}

// AccessControl middleware that checks if the session type is allowed to access the endpoint.
// It also sets the compute units on the context if the endpoint requires it.
func AccessControl(acl ServiceConfig[ACL], cost ServiceConfig[int64], defaultCost int64, eh ErrHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = defaultErrHandler
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := newRequest(r.URL.Path)
			if req == nil {
				eh(r, w, proto.ErrUnauthorized.WithCausef("invalid rpc method called"))
				return
			}

			types, ok := acl.GetConfig(req)
			if !ok {
				eh(r, w, proto.ErrUnauthorized.WithCausef("rpc method not found"))
				return
			}

			if session := GetSessionType(r.Context()); !types.Includes(session) {
				eh(r, w, proto.ErrUnauthorized)
				return
			}

			ctx := r.Context()

			credits := defaultCost
			if v, ok := cost.GetConfig(req); ok {
				credits = v
			}
			ctx = WithComputeUnits(ctx, credits)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// EnsurePermission middleware that checks if the session type has the required permission.
func EnsurePermission(client Client, minPermission proto.UserPermission, eh ErrHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = defaultErrHandler
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
