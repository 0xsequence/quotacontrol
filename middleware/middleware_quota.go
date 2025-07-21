package middleware

import (
	"net/http"
	"strconv"

	"github.com/0xsequence/authcontrol"
	authproto "github.com/0xsequence/authcontrol/proto"
	"github.com/0xsequence/quotacontrol/proto"
)

// VerifyQuota middleware fetches and verify the quota from access key or project ID.
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
				chainIDs  []uint64
			)

			if o.ChainFunc != nil {
				chainIDs = o.ChainFunc(r)
			}

			if session == authproto.SessionType_Project {
				// fetch and verify project quota
				id, ok := authcontrol.GetProjectID(ctx)
				if !ok {
					o.ErrHandler(r, w, proto.ErrUnauthorizedUser.WithCausef("verify quota: no project ID found in context"))
					return
				}
				projectID = id

				q, err := client.FetchProjectQuota(ctx, projectID, chainIDs, now)
				if err != nil {
					o.ErrHandler(r, w, err)
					return
				}

				if q != nil {
					if _, ok := authcontrol.GetAccount(ctx); ok {
						// if the jwt has an account, check if the account permission
						if ok, err := client.CheckPermission(ctx, projectID, proto.UserPermission_READ); !ok {
							if err == nil {
								err = proto.ErrUnauthorizedUser.WithCausef("verify quota: no read permission")
							}
							o.ErrHandler(r, w, err)
							return
						}
					} else if _, ok := authcontrol.GetAccessKey(ctx); !ok {
						// otherwise make sure the request has an access key
						o.ErrHandler(r, w, proto.ErrUnauthorizedUser.WithCausef("verify quota: no access key found in context"))
						return
					}
					quota = q
				}
			}

			// fetch and verify access key quota
			accessKey, ok := authcontrol.GetAccessKey(ctx)
			if !ok && session == authproto.SessionType_AccessKey {
				o.ErrHandler(r, w, proto.ErrUnauthorizedUser.WithCausef("verify quota: no access key found in context"))
				return
			}

			if ok {
				// check that project ID matches
				if projectID != 0 {
					if v, _ := authcontrol.GetProjectIDFromAccessKey(accessKey); v != projectID {
						o.ErrHandler(r, w, proto.ErrAccessKeyMismatch)
						return
					}
				}

				// fetch and verify access key quota
				q, err := client.FetchKeyQuota(ctx, accessKey, r.Header.Get(HeaderOrigin), chainIDs, now)
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

			if quota != nil {
				ctx = withAccessQuota(ctx, quota)

				svc := client.GetService()
				cfg, ok := quota.Limit.ServiceLimit[svc]
				if !ok {
					o.ErrHandler(r, w, proto.ErrAborted.WithCausef("limit not found for %s", svc.GetName()))
					return
				}
				w.Header().Set(HeaderQuotaLimit, strconv.FormatInt(cfg.FreeMax, 10))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
