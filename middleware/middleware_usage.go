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
	HeaderCreditsCost    = "Credits-Cost"
)

// EnsureUsage is a middleware that checks if the quota has enough usage left.
func EnsureUsage(client Client, eh ErrHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = defaultErrHandler
	}
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

			cu, ok := getComputeUnits(ctx)
			if !ok {
				cu = client.GetDefaultUsage()
			}
			if cu == 0 {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set(HeaderCreditsCost, strconv.FormatInt(cu, 10))

			usage, err := client.FetchUsage(ctx, quota, GetTime(ctx))
			if err != nil {
				eh(r, w, err)
				return
			}
			w.Header().Set(HeaderQuotaRemaining, strconv.FormatInt(max(quota.Limit.FreeMax-usage, 0), 10))
			if overage := max(usage-quota.Limit.FreeMax, 0); overage > 0 {
				w.Header().Set(HeaderQuotaOverage, strconv.FormatInt(overage, 10))
			}
			if usage+cu > quota.Limit.OverMax {
				eh(r, w, proto.ErrLimitExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SpendUsage is a middleware that spends the usage from the quota.
func SpendUsage(client Client, eh ErrHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = defaultErrHandler
	}
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

			cu, ok := getComputeUnits(ctx)
			if !ok {
				cu = client.GetDefaultUsage()
			}
			if cu == 0 {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set(HeaderCreditsCost, strconv.FormatInt(cu, 10))

			ok, total, err := client.SpendQuota(ctx, quota, cu, GetTime(ctx))
			if err != nil && !errors.Is(err, proto.ErrLimitExceeded) {
				eh(r, w, err)
				return
			}

			w.Header().Set(HeaderQuotaRemaining, strconv.FormatInt(max(quota.Limit.FreeMax-total, 0), 10))
			if overage := total - quota.Limit.FreeMax; overage > 0 {
				w.Header().Set(HeaderQuotaOverage, strconv.FormatInt(overage, 10))
			}

			if errors.Is(err, proto.ErrLimitExceeded) {
				eh(r, w, err)
				return
			}

			if ok {
				ctx = withSpending(ctx)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
