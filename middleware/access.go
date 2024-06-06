package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

const (
	HeaderAccessKey = "X-Access-Key"
	HeaderOrigin    = "Origin"
)

// Client is the interface that wraps the basic FetchKeyQuota, GetUsage and SpendQuota methods.
type Client interface {
	IsEnabled() bool
	FetchKeyQuota(ctx context.Context, accessKey, origin string, now time.Time) (*proto.AccessQuota, error)
	FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error)
	FetchUserPermission(ctx context.Context, projectID uint64, userID string, useCache bool) (*proto.UserPermission, *proto.ResourceAccess, error)
	SpendQuota(ctx context.Context, quota *proto.AccessQuota, computeUnits int64, now time.Time) (bool, error)
}

// ErrorHandler is a function that handles errors.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, next http.Handler, err error)

// SetAccessKey get the access key from the header and sets it in the context.
func SetAccessKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessKey := r.Header.Get(HeaderAccessKey)

		ctx := r.Context()
		if accessKey != "" {
			ctx = WithAccessKey(ctx, accessKey)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// VerifyAccessKey verifies the accessKey and adds the AccessQuota to the request context.
func VerifyAccessKey(client Client, eh ErrorHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = _DefaultErrorHandler
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// skip with no access key, or quotacontrol is disabled
			accessKey := getAccessKey(ctx)
			if !client.IsEnabled() || accessKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			quota, err := client.FetchKeyQuota(ctx, accessKey, r.Header.Get(HeaderOrigin), GetTime(ctx))
			if err != nil {
				eh(w, r, next, err)
				return
			}

			// set access quota in context
			ctx = withAccessQuota(ctx, quota)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// EnsureUsage is a middleware that checks if the access key has enough usage left.
func EnsureUsage(client Client, eh ErrorHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = _DefaultErrorHandler
	}
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
				eh(w, r, next, err)
				return
			}
			if usage+cu > quota.Limit.OverMax {
				eh(w, r, next, proto.ErrLimitExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ProjectRateKey(projectID uint64) string {
	return fmt.Sprintf("rl:project:%d", projectID)
}

func SpendUsage(client Client, eh ErrorHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = _DefaultErrorHandler
	}
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
				eh(w, r, next, err)
				return
			}

			if !ok {
				eh(w, r, next, proto.ErrLimitExceeded)
				return
			}

			next.ServeHTTP(w, r.WithContext(withResult(ctx)))
		})
	}
}
