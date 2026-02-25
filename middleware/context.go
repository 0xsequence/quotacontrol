package middleware

import (
	"context"
	"time"

	"github.com/0xsequence/authcontrol"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
)

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "quotacontrol context value " + k.name
}

var (
	ctxKeyAccessQuota = &contextKey{"AccessQuota"}
	ctxKeyCost        = &contextKey{"Cost"}
	ctxKeyTime        = &contextKey{"Time"}
	ctxKeySpending    = &contextKey{"Spending"}
)

// withAccessQuota adds the quota to the context.
func withAccessQuota(ctx context.Context, quota *proto.AccessQuota) context.Context {
	return context.WithValue(ctx, ctxKeyAccessQuota, quota)
}

// GetAccessQuota returns the access quota from the context.
func GetAccessQuota(ctx context.Context) (*proto.AccessQuota, bool) {
	v, ok := ctx.Value(ctxKeyAccessQuota).(*proto.AccessQuota)
	return v, ok
}

// GetProjectID returns the projectID and if its active from the context.
// In case its not set, it will return 0.
func GetProjectID(ctx context.Context) (uint64, bool) {
	if v, ok := authcontrol.GetProjectID(ctx); ok {
		return v, true
	}
	if q, ok := GetAccessQuota(ctx); ok {
		return q.GetProjectID(), q.IsActive()
	}
	return 0, false
}

// WithCost sets the cost and rate limit increment to the context.
func WithCost(ctx context.Context, cu int64) context.Context {
	ctx = httprate.WithIncrement(ctx, int(cu))
	return context.WithValue(ctx, ctxKeyCost, cu)
}

// getCost returns the cost from the context. If the cost is not set, it returns 1.
func getCost(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxKeyCost).(int64)
	return v, ok
}

// AddCost adds the cost to the context.
func AddCost(ctx context.Context, cu int64) context.Context {
	v, _ := ctx.Value(ctxKeyCost).(int64)
	return WithCost(ctx, v+cu)
}

// WithTime sets the time to the context.
func WithTime(ctx context.Context, now time.Time) context.Context {
	return context.WithValue(ctx, ctxKeyTime, now)
}

// GetTime returns the time from the context. If the time is not set, it returns the current time.
func GetTime(ctx context.Context) time.Time {
	v, ok := ctx.Value(ctxKeyTime).(time.Time)
	if !ok {
		return time.Now().Truncate(time.Hour * 24)
	}
	return v
}

// withSpending sets the result of spending in the context.
func withSpending(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeySpending, struct{}{})
}

// HasSpending returns the result of spending from the context.
func HasSpending(ctx context.Context) bool {
	_, ok := ctx.Value(ctxKeySpending).(struct{})
	return ok
}
