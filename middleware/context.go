package middleware

import (
	"context"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "quotacontrol context value " + k.name
}

var (
	ctxKeyTokenKey      = &contextKey{"TokenKey"}
	ctxKeyCachedToken   = &contextKey{"CachedToken"}
	ctxKeyComputeUnits  = &contextKey{"ComputeUnits"}
	ctxKeyRateLimitSkip = &contextKey{"RateLimitSkip"}
	ctxKeyTime          = &contextKey{"Time"}
	ctxKeyResult        = &contextKey{"Result"}
)

// WithTokenKey adds the token key to the context.
func WithTokenKey(ctx context.Context, tokenKey string) context.Context {
	return context.WithValue(ctx, ctxKeyTokenKey, tokenKey)
}

// getTokenKey returns the token key from the context.
func getTokenKey(ctx context.Context) string {
	v, ok := ctx.Value(ctxKeyTokenKey).(string)
	if !ok {
		return ""
	}
	return v
}

// withToken adds the token to the context.
func withToken(ctx context.Context, token *proto.CachedToken) context.Context {
	return context.WithValue(ctx, ctxKeyCachedToken, token)
}

// GetToken returns the token from the context.
func GetToken(ctx context.Context) *proto.CachedToken {
	v, ok := ctx.Value(ctxKeyCachedToken).(*proto.CachedToken)
	if !ok {
		return nil
	}
	return v
}

// WithSkipRateLimit adds the skip rate limit flag to the context.
func WithSkipRateLimit(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyRateLimitSkip, struct{}{})
}

// IsSkipRateLimit returns true if the skip rate limit flag is set in the context.
func IsSkipRateLimit(ctx context.Context) bool {
	_, ok := ctx.Value(ctxKeyRateLimitSkip).(struct{})
	return ok
}

// WithComputeUnits sets the compute units to the context.
func WithComputeUnits(ctx context.Context, cu int64) context.Context {
	return context.WithValue(ctx, ctxKeyComputeUnits, cu)
}

// GetComputeUnits returns the compute units from the context. If the compute units is not set, it returns 1.
func GetComputeUnits(ctx context.Context) int64 {
	v, ok := ctx.Value(ctxKeyComputeUnits).(int64)
	if !ok {
		return 1
	}
	return v
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

// getTime returns the time from the context. If the time is not set, it returns the current time.
func getTime(ctx context.Context) time.Time {
	v, ok := ctx.Value(ctxKeyTime).(time.Time)
	if !ok {
		return time.Now().Truncate(time.Hour * 24)
	}
	return v
}

// withResult sets the result of spending in the context.
func withResult(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyResult, struct{}{})
}

// GetResult returns the result of spending from the context.
func GetResult(ctx context.Context) bool {
	_, ok := ctx.Value(ctxKeyResult).(struct{})
	return ok
}
