package middleware

import (
	"context"
	"time"

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
	ctxKeySessionType  = &contextKey{"SessionType"}
	ctxKeyAccount      = &contextKey{"Account"}
	ctxKeyUser         = &contextKey{"User"}
	ctxKeyService      = &contextKey{"Service"}
	ctxKeyAccessKey    = &contextKey{"AccessKey"}
	ctxKeyAccessQuota  = &contextKey{"AccessQuota"}
	ctxKeyProjectID    = &contextKey{"ProjectID"}
	ctxKeyComputeUnits = &contextKey{"ComputeUnits"}
	ctxKeyTime         = &contextKey{"Time"}
	ctxKeySpending     = &contextKey{"Spending"}
)

// WithSessionType adds the access key to the context.
func WithSessionType(ctx context.Context, accessType proto.SessionType) context.Context {
	return context.WithValue(ctx, ctxKeySessionType, accessType)
}

// GetSessionType returns the access key from the context.
func GetSessionType(ctx context.Context) (proto.SessionType, bool) {
	v, ok := ctx.Value(ctxKeySessionType).(proto.SessionType)
	if !ok {
		return proto.SessionType_Public, false
	}
	return v, true
}

// WithAccount adds the account to the context.
func WithAccount(ctx context.Context, account string) context.Context {
	return context.WithValue(ctx, ctxKeyAccount, account)
}

// GetAccount returns the account from the context.
func GetAccount(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyAccount).(string)
	return v, ok
}

// WithUser adds the user to the context.
func WithUser(ctx context.Context, user any) context.Context {
	return context.WithValue(ctx, ctxKeyUser, user)
}

// GetUser returns the user from the context.
func GetUser[T any](ctx context.Context) (T, bool) {
	v, ok := ctx.Value(ctxKeyUser).(T)
	return v, ok
}

// WithService adds the service to the context.
func WithService(ctx context.Context, service string) context.Context {
	return context.WithValue(ctx, ctxKeyService, service)
}

// GetService returns the service from the context.
func GetService(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyService).(string)
	return v, ok
}

// WithAccessKey adds the access key to the context.
func WithAccessKey(ctx context.Context, accessKey string) context.Context {
	return context.WithValue(ctx, ctxKeyAccessKey, accessKey)
}

// GetAccessKey returns the access key from the context.
func GetAccessKey(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyAccessKey).(string)
	return v, ok
}

// withAccessQuota adds the quota to the context.
func withAccessQuota(ctx context.Context, quota *proto.AccessQuota) context.Context {
	return context.WithValue(ctx, ctxKeyAccessQuota, quota)
}

// GetAccessQuota returns the access quota from the context.
func GetAccessQuota(ctx context.Context) (*proto.AccessQuota, bool) {
	v, ok := ctx.Value(ctxKeyAccessQuota).(*proto.AccessQuota)
	return v, ok
}

// withProjectID adds the projectID to the context.
func withProjectID(ctx context.Context, projectID uint64) context.Context {
	return context.WithValue(ctx, ctxKeyProjectID, projectID)
}

// GetProjectID returns the projectID and if its active from the context.
// In case its not set, it will return 0.
func GetProjectID(ctx context.Context) (uint64, bool) {
	if v, ok := getProjectID(ctx); ok {
		return v, true
	}
	if q, ok := GetAccessQuota(ctx); ok {
		return q.GetProjectID(), q.IsActive()
	}
	return 0, false
}

func getProjectID(ctx context.Context) (uint64, bool) {
	v, ok := ctx.Value(ctxKeyProjectID).(uint64)
	return v, ok
}

// WithComputeUnits sets the compute units and rate limit increment to the context.
func WithComputeUnits(ctx context.Context, cu int64) context.Context {
	ctx = httprate.WithIncrement(ctx, int(cu))
	return context.WithValue(ctx, ctxKeyComputeUnits, cu)
}

// getComputeUnits returns the compute units from the context. If the compute units is not set, it returns 1.
func getComputeUnits(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxKeyComputeUnits).(int64)
	return v, ok
}

// AddComputeUnits adds the compute units to the context.
func AddComputeUnits(ctx context.Context, cu int64) context.Context {
	v, _ := ctx.Value(ctxKeyComputeUnits).(int64)
	return WithComputeUnits(ctx, v+cu)
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
