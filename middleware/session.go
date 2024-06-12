package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Session middleware that detects the session type and sets the account or service on the context.
func Session(ja *jwtauth.JWTAuth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			token, claims, err := getJWT(ctx)
			if err != nil {
				proto.RespondWithError(w, err)
				return
			}
			if token == nil {
				ctx = withSessionType(ctx, proto.SessionType_Public)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// When JWT token is found, ensure it verifies, or error
			if err := jwt.Validate(token, ja.ValidateOptions()...); err != nil {
				proto.RespondWithError(w, proto.ErrUnauthorized.WithCause(err))
				return
			}

			// Origin check
			if originClaim, ok := claims["ogn"].(string); ok {
				originClaim = strings.TrimSuffix(originClaim, "/")
				originHeader := strings.TrimSuffix(r.Header.Get("Origin"), "/")
				if originHeader != "" && originHeader != originClaim {
					proto.RespondWithError(w, proto.ErrUnauthorized.WithCausef("invalid origin claim"))
					return
				}
			}

			// Set account or service on the context
			accountClaim, _ := claims["account"].(string)
			serviceClaim, _ := claims["service"].(string)
			adminClaim, _ := claims["admin"].(bool)

			if quota := GetAccessQuota(ctx); quota != nil {
				sessionType := proto.SessionType_AccessKey
				if quota.IsJWT() {
					sessionType = proto.SessionType_Project
					ctx = withAccount(ctx, accountClaim)
				}
				ctx = withSessionType(ctx, sessionType)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			switch {
			case adminClaim:
				if accountClaim == "" {
					proto.RespondWithError(w, proto.ErrUnauthorized)
					return
				}
				ctx = withAccount(ctx, accountClaim)
				ctx = withSessionType(ctx, proto.SessionType_Admin)
			case accountClaim != "":
				ctx = withAccount(ctx, accountClaim)
				ctx = withSessionType(ctx, proto.SessionType_Account)
			case serviceClaim != "":
				ctx = withSessionType(ctx, proto.SessionType_Service)
				ctx = withService(ctx, serviceClaim)
			default:
				proto.RespondWithError(w, proto.ErrUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AccessControl middleware that checks if the session type is allowed to access the endpoint.
// It also sets the compute units on the context if the endpoint requires it.
func AccessControl(acl ACL, cost Cost, defaultCost int64) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := newRequest(r.URL.Path)
			if req == nil {
				proto.RespondWithError(w, proto.ErrUnauthorized.WithCausef("invalid rpc method called"))
				return
			}

			sessions, ok := acl.GetConfig(req)
			if !ok {
				proto.RespondWithError(w, proto.ErrUnauthorized.WithCausef("rpc method not found"))
				return
			}

			if !slices.Contains(sessions, GetSessionType(r.Context())) {
				proto.RespondWithError(w, proto.ErrUnauthorized)
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
