package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type KeyFunc func(*http.Request) string

func KeyFromHeader(r *http.Request) string {
	return r.Header.Get(HeaderAccessKey)
}

func Session(client Client, auth *jwtauth.JWTAuth, keyFuncs ...KeyFunc) func(next http.Handler) http.Handler {
	keyFuncs = append([]KeyFunc{KeyFromHeader}, keyFuncs...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			var (
				sessionType proto.SessionType
				quota       *proto.AccessQuota
				accessKey   string
				token       jwt.Token
			)

			for _, f := range keyFuncs {
				if accessKey = f(r); accessKey != "" {
					break
				}
			}

			token, err := jwtauth.VerifyRequest(auth, r, jwtauth.TokenFromHeader, jwtauth.TokenFromCookie)
			if err != nil {
				if errors.Is(err, jwtauth.ErrExpired) {
					proto.RespondWithError(w, proto.ErrSessionExpired)
					return
				}
				if !errors.Is(err, jwtauth.ErrNoTokenFound) {
					proto.RespondWithError(w, proto.ErrUnauthorizedUser)
					return
				}
			}

			now := GetTime(ctx)

			if token != nil {
				claims, err := token.AsMap(r.Context())
				if err != nil {
					proto.RespondWithError(w, err)
					return
				}

				if serviceClaim, ok := claims["service"].(string); ok {
					ctx = withService(ctx, serviceClaim)
					sessionType = proto.SessionType_Service
				} else if accountClaim, ok := claims["account"].(string); ok {
					ctx = withAccount(ctx, accountClaim)
					sessionType = proto.SessionType_Account
					if adminClaim, ok := claims["admin"].(bool); ok && adminClaim {
						sessionType = proto.SessionType_Admin
					} else if projectClaim, ok := claims["project"].(float64); ok {
						projectID := uint64(projectClaim)
						if quota, err = client.FetchProjectQuota(ctx, projectID, now); err != nil {
							proto.RespondWithError(w, err)
							return
						}
						ok, err := client.CheckPermission(ctx, projectID, accountClaim, proto.UserPermission_READ)
						if err != nil {
							proto.RespondWithError(w, err)
							return
						}
						if !ok {
							proto.RespondWithError(w, proto.ErrUnauthorizedUser)
							return
						}
						ctx = withProjectID(ctx, projectID)
						sessionType = proto.SessionType_Project
					}
				}
			}
			if accessKey != "" && sessionType < proto.SessionType_Admin {
				projectID, err := proto.GetProjectID(accessKey)
				if err != nil {
					proto.RespondWithError(w, err)
					return
				}
				if quota != nil && quota.GetProjectID() != projectID {
					proto.RespondWithError(w, proto.ErrAccessKeyMismatch)
					return
				}
				q, err := client.FetchKeyQuota(ctx, accessKey, r.Header.Get(HeaderOrigin), now)
				if err != nil {
					proto.RespondWithError(w, err)
					return
				}
				if q != nil && !q.IsActive() {
					proto.RespondWithError(w, proto.ErrAccessKeyNotFound)
					return
				}
				if quota == nil {
					quota = q
				}
				sessionType = max(sessionType, proto.SessionType_AccessKey)
			}

			ctx = withSessionType(ctx, sessionType)

			if quota != nil {
				ctx = withAccessQuota(ctx, quota)
				w.Header().Set(HeaderQuotaLimit, strconv.FormatInt(quota.Limit.FreeMax, 10))
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
