package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/0xsequence/quotacontrol/proto"
)

const (
	HeaderQuotaLimit     = "Quota-Limit"
	HeaderQuotaRemaining = "Quota-Remaining"
	HeaderQuotaOverage   = "Quota-Overage"
	HeaderQuotaCost      = "Quota-Cost"
)

// EnsureUsage is a middleware that checks if the quota has enough usage left.
func EnsureUsage(client Client, o Options) func(next http.Handler) http.Handler {
	o.ApplyDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !client.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			quota, ok := GetAccessQuota(ctx)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			cu, ok := getCost(ctx)
			if !ok {
				cu = client.GetDefaultUsage()
			}
			if cu == 0 {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("X-RateLimit-Increment", strconv.FormatInt(cu, 10))

			usage, err := client.FetchUsage(ctx, quota, GetTime(ctx))
			if err != nil {
				o.ErrHandler(r, w, err)
				return
			}

			limit, ok := quota.Limit.GetSettings(client.GetService())
			if !ok {
				o.ErrHandler(r, w, proto.ErrAborted.WithCausef("verify quota: service limit not found for %s", client.GetService().GetName()))
				return
			}

			w.Header().Set(HeaderQuotaRemaining, strconv.FormatInt(max(limit.FreeMax-usage, 0), 10))
			if overage := max(usage-limit.FreeMax, 0); overage > 0 {
				w.Header().Set(HeaderQuotaOverage, strconv.FormatInt(overage, 10))
			}
			if usage+cu > limit.OverMax {
				o.ErrHandler(r, w, proto.ErrQuotaExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SpendUsage is a middleware that spends the usage from the quota.
func SpendUsage(client Client, o Options) func(next http.Handler) http.Handler {
	o.ApplyDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !client.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			quota, ok := GetAccessQuota(ctx)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			cu, ok := getCost(ctx)
			if !ok {
				cu = client.GetDefaultUsage()
			}
			if cu == 0 {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set(HeaderQuotaCost, strconv.FormatInt(cu, 10))

			limit, ok := quota.Limit.GetSettings(client.GetService())
			if !ok {
				o.ErrHandler(r, w, proto.ErrAborted.WithCausef("verify quota: service limit not found for %s", client.GetService().GetName()))
				return
			}

			ok, total, err := client.SpendQuota(ctx, quota, cu, GetTime(ctx))
			if err != nil && !errors.Is(err, proto.ErrQuotaExceeded) {
				o.ErrHandler(r, w, err)
				return
			}

			w.Header().Set(HeaderQuotaRemaining, strconv.FormatInt(max(limit.FreeMax-total, 0), 10))
			if overage := total - limit.FreeMax; overage > 0 {
				w.Header().Set(HeaderQuotaOverage, strconv.FormatInt(overage, 10))
			}

			if errors.Is(err, proto.ErrQuotaExceeded) {
				o.ErrHandler(r, w, err)
				return
			}

			if ok {
				ctx = withSpending(ctx)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
