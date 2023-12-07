package middleware

import (
	"net/http"

	"github.com/0xsequence/quotacontrol/proto"
)

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

			usage, err := client.FetchUsage(ctx, quota, getTime(ctx))
			if err != nil {
				eh(w, r, next, err)
				return
			}
			if usage+cu > quota.Limit.HardQuota {
				eh(w, r, next, proto.ErrLimitExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SpendUsage is a middleware that spends the usage of the access key.
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

			ok, err := client.SpendQuota(ctx, quota, cu, getTime(ctx))
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
