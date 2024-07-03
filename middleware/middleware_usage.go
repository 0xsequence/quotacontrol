package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/0xsequence/quotacontrol/proto"
)

// EnsureUsage is a middleware that checks if the quota has enough usage left.
func EnsureUsage(client Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return ensure{Client: client, Next: next}
	}
}

// SpendUsage is a middleware that spends the usage from the quota.
func SpendUsage(client Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return spend{Client: client, Next: next}
	}
}

type ensure struct {
	Client Client
	Next   http.Handler
}

func (m ensure) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	quota := GetAccessQuota(ctx)
	if quota == nil {
		m.Next.ServeHTTP(w, r)
		return
	}

	cu, ok := getComputeUnits(ctx)
	if !ok {
		cu = m.Client.GetDefaultUsage()
	}
	if cu == 0 {
		m.Next.ServeHTTP(w, r)
		return
	}

	usage, err := m.Client.FetchUsage(ctx, quota, GetTime(ctx))
	if err != nil {
		proto.RespondWithError(w, err)
		return
	}
	w.Header().Set(HeaderQuotaRemaining, strconv.FormatInt(max(quota.Limit.FreeMax-usage, 0), 10))
	if overage := max(usage-quota.Limit.FreeMax, 0); overage > 0 {
		w.Header().Set(HeaderQuotaOverage, strconv.FormatInt(overage, 10))
	}
	if usage+cu > quota.Limit.OverMax {
		proto.RespondWithError(w, proto.ErrLimitExceeded)
		return
	}

	m.Next.ServeHTTP(w, r)
}

type spend struct {
	Client Client
	Next   http.Handler
}

func (m spend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.Client.IsEnabled() {
		m.Next.ServeHTTP(w, r)
		return
	}

	ctx := r.Context()

	quota := GetAccessQuota(ctx)
	if quota == nil {
		m.Next.ServeHTTP(w, r)
		return
	}

	cu, ok := getComputeUnits(ctx)
	if !ok {
		cu = m.Client.GetDefaultUsage()
	}
	if cu == 0 {
		m.Next.ServeHTTP(w, r)
		return
	}

	ok, total, err := m.Client.SpendQuota(ctx, quota, cu, GetTime(ctx))
	if err != nil && !errors.Is(err, proto.ErrLimitExceeded) {
		proto.RespondWithError(w, err)
		return
	}

	w.Header().Set(HeaderQuotaRemaining, strconv.FormatInt(max(quota.Limit.FreeMax-total, 0), 10))
	if overage := total - quota.Limit.FreeMax; overage > 0 {
		w.Header().Set(HeaderQuotaOverage, strconv.FormatInt(overage, 10))
	}

	if errors.Is(err, proto.ErrLimitExceeded) {
		proto.RespondWithError(w, err)
		return
	}

	if ok {
		ctx = withSpending(ctx)
	}

	m.Next.ServeHTTP(w, r.WithContext(ctx))
}
