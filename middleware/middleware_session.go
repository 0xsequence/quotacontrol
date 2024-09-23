package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const (
	HeaderAccessKey = "X-Access-Key"
	HeaderOrigin    = "Origin"
)

type KeyFunc func(*http.Request) string

func KeyFromHeader(r *http.Request) string {
	return r.Header.Get(HeaderAccessKey)
}

type UserStore interface {
	GetUser(ctx context.Context, address string) (any, error)
}

func Session(client Client, auth *jwtauth.JWTAuth, u UserStore, eh ErrHandler, keyFuncs ...KeyFunc) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = proto.RespondWithError
	}
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
					eh(w, proto.ErrSessionExpired)
					return
				}
				if !errors.Is(err, jwtauth.ErrNoTokenFound) {
					eh(w, proto.ErrUnauthorizedUser)
					return
				}
			}

			now := GetTime(ctx)

			if token != nil {
				claims, err := token.AsMap(r.Context())
				if err != nil {
					eh(w, err)
					return
				}

				serviceClaim, _ := claims["service"].(string)
				accountClaim, _ := claims["account"].(string)
				adminClaim, _ := claims["admin"].(bool)
				projectClaim, _ := claims["project"].(float64)
				switch {
				case serviceClaim != "":
					ctx = withService(ctx, serviceClaim)
					sessionType = proto.SessionType_Service
				case accountClaim != "":
					ctx = WithAccount(ctx, accountClaim)
					sessionType = proto.SessionType_Wallet

					if u != nil {
						user, err := u.GetUser(ctx, accountClaim)
						if err != nil {
							eh(w, err)
							return
						}
						if user != nil {
							sessionType = proto.SessionType_User
							ctx = WithUser(ctx, user)
						}
					}

					if adminClaim {
						sessionType = proto.SessionType_Admin
						break
					}

					if projectClaim > 0 {
						projectID := uint64(projectClaim)
						if quota, err = client.FetchProjectQuota(ctx, projectID, now); err != nil {
							eh(w, err)
							return
						}
						ok, err := client.CheckPermission(ctx, projectID, proto.UserPermission_READ)
						if err != nil {
							eh(w, err)
							return
						}
						if !ok {
							eh(w, proto.ErrUnauthorizedUser)
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
					eh(w, err)
					return
				}
				if quota != nil && quota.GetProjectID() != projectID {
					eh(w, proto.ErrAccessKeyMismatch)
					return
				}
				q, err := client.FetchKeyQuota(ctx, accessKey, r.Header.Get(HeaderOrigin), now)
				if err != nil {
					eh(w, err)
					return
				}
				if q != nil && !q.IsActive() {
					eh(w, proto.ErrAccessKeyNotFound)
					return
				}
				if quota == nil {
					quota = q
				}
				ctx = WithAccessKey(ctx, accessKey)
				sessionType = max(sessionType, proto.SessionType_AccessKey)
			}

			ctx = WithSessionType(ctx, sessionType)

			if quota != nil {
				ctx = withAccessQuota(ctx, quota)
				w.Header().Set(HeaderQuotaLimit, strconv.FormatInt(quota.Limit.FreeMax, 10))
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
