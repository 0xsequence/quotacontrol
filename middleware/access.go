package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const (
	HeaderAccessKey = "X-Access-Key"
	HeaderOrigin    = "Origin"
)

// Client is the interface that wraps the basic FetchKeyQuota, GetUsage and SpendQuota methods.
type Client interface {
	IsEnabled() bool
	FetchProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error)
	FetchKeyQuota(ctx context.Context, accessKey, origin string, now time.Time) (*proto.AccessQuota, error)
	FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error)
	FetchUserPermission(ctx context.Context, projectID uint64, userID string, useCache bool) (*proto.UserPermission, *proto.ResourceAccess, error)
	SpendQuota(ctx context.Context, quota *proto.AccessQuota, computeUnits int64, now time.Time) (bool, error)
}

// SetKey gets the access key header and sets it in the context.
// It uses JWT from `jwtauth.Verifier` to extract the project claim and sets it in the context as well.
// When both are present, it checks project mismatch between the access key and the JWT token.
func SetKey(ja *jwtauth.JWTAuth) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var (
				ctx       = r.Context()
				projectID = uint64(0)
				accessKey = r.Header.Get(HeaderAccessKey)
			)

			if accessKey != "" {
				projectID, _ = proto.GetProjectID(accessKey)
				ctx = WithAccessKey(ctx, accessKey)
			}

			if ja == nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			token, claims, err := jwtauth.FromContext(ctx)
			if err != nil {
				if errors.Is(err, jwtauth.ErrNoTokenFound) {
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				proto.RespondWithError(w, err)
				return
			}

			if err := jwt.Validate(token, ja.ValidateOptions()...); err != nil {
				proto.RespondWithError(w, err)
				return
			}

			if v, ok := claims["project"].(float64); ok {
				if projectID != 0 && uint64(v) != projectID {
					proto.RespondWithError(w, proto.ErrAccessKeyMismatch)
					return
				}
				projectID = uint64(v)
			}

			if projectID != 0 {
				ctx = withProjectID(ctx, projectID)
			}

			if account, ok := claims["account"].(string); ok {
				ctx = withAccount(ctx, account)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// VerifyQuota checks if the project is in the context and fetches the quota for the project.
// If it's not, but an access key is present, it fetches the quota for the access key.
func VerifyQuota(client Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !client.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			var (
				quota *proto.AccessQuota
				now   = GetTime(ctx)
			)

			// check if we alreayd have a project ID from the JWT
			if projectID, ok := GetProjectID(ctx); ok {
				q, err := client.FetchProjectQuota(ctx, projectID, now)
				if err != nil {
					proto.RespondWithError(w, err)
					return
				}
				quota = q
			}

			// check if we have an access key
			if accessKey := getAccessKey(ctx); accessKey != "" {
				q, err := client.FetchKeyQuota(ctx, accessKey, r.Header.Get(HeaderOrigin), now)
				if err != nil {
					proto.RespondWithError(w, err)
					return
				}
				quota = q
			}

			if quota != nil {
				ctx = withAccessQuota(ctx, quota)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// EnsureUsage is a middleware that checks if the quota has enough usage left.
func EnsureUsage(client Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			quota := GetAccessQuota(ctx)
			if quota == nil {
				next.ServeHTTP(w, r)
				return
			}

			cu := GetComputeUnits(ctx)
			if cu == 0 {
				next.ServeHTTP(w, r)
				return
			}

			usage, err := client.FetchUsage(ctx, quota, GetTime(ctx))
			if err != nil {
				proto.RespondWithError(w, err)
				return
			}
			if usage+cu > quota.Limit.OverMax {
				proto.RespondWithError(w, proto.ErrLimitExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SpendUsage is a middleware that spends the usage from the quota.
func SpendUsage(client Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			quota, cu := GetAccessQuota(ctx), GetComputeUnits(ctx)

			if quota == nil || cu == 0 {
				next.ServeHTTP(w, r)
				return
			}

			ok, err := client.SpendQuota(ctx, quota, cu, GetTime(ctx))
			if err != nil {
				proto.RespondWithError(w, err)
				return
			}

			if ok {
				ctx = withSpending(ctx)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func EnsurePermission(client Client, minPermission proto.UserPermission) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !client.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			// check if we alreayd have a project ID from the JWT
			q := GetAccessQuota(ctx)
			if q == nil || !q.IsJWT() {
				proto.RespondWithError(w, proto.ErrUnauthorizedUser)
				return
			}

			perm, _, err := client.FetchUserPermission(ctx, q.GetProjectID(), getAccount(ctx), true)
			if err != nil {
				proto.RespondWithError(w, err)
				return
			}

			if perm == nil || *perm < minPermission {
				proto.RespondWithError(w, proto.ErrUnauthorizedUser)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
