package middleware

import (
	"net/http"
	"strconv"

	"github.com/0xsequence/quotacontrol/proto"

	"github.com/0xsequence/authcontrol"
)

func VerifyQuota(client Client, o Options) func(next http.Handler) http.Handler {
	o.ApplyDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			now := GetTime(ctx)

			session, _ := authcontrol.GetSessionType(ctx)

			var (
				projectID uint64
				quota     *proto.AccessQuota
			)

			if session == proto.SessionType_Project {
				id, ok := authcontrol.GetProjectID(ctx)
				if !ok {
					o.ErrHandler(r, w, proto.ErrUnauthorizedUser)
					return
				}
				projectID = id

				q, err := client.FetchProjectQuota(ctx, projectID, now)
				if err != nil {
					o.ErrHandler(r, w, err)
					return
				}

				if q != nil {
					if ok, err := client.CheckPermission(ctx, projectID, proto.UserPermission_READ); !ok {
						if err == nil {
							err = proto.ErrUnauthorizedUser
						}
						o.ErrHandler(r, w, err)
						return
					}
					quota = q
				}
			}

			if session.Is(proto.SessionType_AccessKey, proto.SessionType_Project) {
				accessKey, ok := authcontrol.GetAccessKey(ctx)
				if !ok && session == proto.SessionType_AccessKey {
					o.ErrHandler(r, w, proto.ErrUnauthorizedUser)
					return
				}

				if ok {
					if projectID != 0 {
						if v, _ := proto.GetProjectID(accessKey); v != projectID {
							o.ErrHandler(r, w, proto.ErrAccessKeyMismatch)
							return
						}
					}

					q, err := client.FetchKeyQuota(ctx, accessKey, r.Header.Get(HeaderOrigin), now)
					if err != nil {
						o.ErrHandler(r, w, err)
						return
					}
					if q != nil {
						if !q.IsActive() {
							o.ErrHandler(r, w, proto.ErrAccessKeyNotFound)
							return
						}
						if quota != nil && quota.AccessKey.ProjectID != q.AccessKey.ProjectID {
							o.ErrHandler(r, w, proto.ErrAccessKeyMismatch)
							return
						}
						quota = q
					}
				}
			}
			if quota != nil {
				ctx = withAccessQuota(ctx, quota)
				w.Header().Set(HeaderQuotaLimit, strconv.FormatInt(quota.Limit.FreeMax, 10))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
