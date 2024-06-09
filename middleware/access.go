package middleware

import (
	"context"
	"errors"
	"fmt"
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
// It uses the JWT token to extract the project ID.
// It also checks for project mmismatch between the access key and the JWT token.
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
			if err != nil && !errors.Is(err, jwtauth.ErrNoTokenFound) {
				proto.RespondWithError(w, err)
				return
			}

			if token == nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if err := jwt.Validate(token, ja.ValidateOptions()...); err == nil {
				if v, ok := claims["project"].(float64); ok {
					if uint64(v) != projectID {
						proto.RespondWithError(w, proto.ErrAccessKeyMismatch)
						return
					}
					projectID = uint64(v)
				}
			}

			if projectID != 0 {
				ctx = withProjectID(ctx, projectID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// VerifyAccessKey verifies the accessKey and adds the AccessQuota to the request context.
func VerifyAccessKey(client Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// skip with no access key, or quotacontrol is disabled
			if !client.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			now := GetTime(ctx)
			origin := r.Header.Get(HeaderOrigin)

			var quota *proto.AccessQuota

			projectID, ok := GetProjectID(ctx)
			if ok {
				q, err := client.FetchProjectQuota(ctx, projectID, now)
				if err != nil {
					proto.RespondWithError(w, err)
					return
				}
				quota = q
			}

			if accessKey := getAccessKey(ctx); accessKey != "" {
				q, err := client.FetchKeyQuota(ctx, accessKey, origin, now)
				if err != nil {
					proto.RespondWithError(w, err)
					return
				}
				quota = q
			}

			if quota != nil {
				ctx = WithAccessQuota(ctx, quota)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// EnsureUsage is a middleware that checks if the access key has enough usage left.
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

func ProjectRateKey(projectID uint64) string {
	return fmt.Sprintf("rl:project:%d", projectID)
}

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
				ctx = withResult(ctx)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
