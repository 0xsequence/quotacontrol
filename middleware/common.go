package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

const (
	HeaderAccessKey      = "X-Access-Key"
	HeaderOrigin         = "Origin"
	HeaderQuotaLimit     = "Quota-Limit"
	HeaderQuotaRemaining = "Quota-Remaining"
	HeaderQuotaOverage   = "Quota-Overage"
)

// Client is the interface that wraps the basic FetchKeyQuota, GetUsage and SpendQuota methods.
type Client interface {
	IsEnabled() bool
	GetDefaultUsage() int64
	FetchProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error)
	FetchKeyQuota(ctx context.Context, accessKey, origin string, now time.Time) (*proto.AccessQuota, error)
	FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error)
	FetchPermission(ctx context.Context, projectID uint64, userID string, useCache bool) (proto.UserPermission, *proto.ResourceAccess, error)
	SpendQuota(ctx context.Context, quota *proto.AccessQuota, computeUnits int64, now time.Time) (bool, int64, error)
}

type Claims map[string]any

func (c Claims) String() string {
	s := strings.Builder{}
	s.WriteString("{")
	for k, v := range c {
		if s.Len() > 1 {
			s.WriteString(", ")
		}
		fmt.Fprintf(&s, "%s:%v", k, v)
	}
	s.WriteString("}")
	return s.String()
}

type ServiceConfig[T any] map[string]map[string]T

type (
	ACL  = ServiceConfig[proto.SessionType]
	Cost = ServiceConfig[int64]
)

func (s ServiceConfig[T]) GetConfig(r *rcpRequest) (v T, ok bool) {
	if r.Package != "rpc" || s == nil {
		return v, false
	}

	serviceACL, ok := s[r.Service]
	if !ok {
		return v, false
	}

	// get method's ACL
	cfg, ok := serviceACL[r.Method]
	if !ok {
		return v, false
	}

	return cfg, true
}

type rcpRequest struct {
	Package string
	Service string
	Method  string
}

func newRequest(path string) *rcpRequest {
	parts := strings.Split(path, "/")
	if len(parts) != 4 {
		return nil
	}
	if parts[0] != "" {
		return nil
	}
	t := rcpRequest{
		Package: parts[1],
		Service: parts[2],
		Method:  parts[3],
	}
	if t.Package == "" || t.Service == "" || t.Method == "" {
		return nil
	}
	return &t
}

func ProjectRateKey(projectID uint64) string {
	return fmt.Sprintf("rl:project:%d", projectID)
}

func AccountRateKey(account string) string {
	return fmt.Sprintf("rl:account:%s", account)
}

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "quotacontrol context value " + k.name
}

var (
	ctxKeySessionType  = &contextKey{"SessionType"}
	ctxKeyAccount      = &contextKey{"Account"}
	ctxKeyService      = &contextKey{"Service"}
	ctxKeyAccessKey    = &contextKey{"AccessKey"}
	ctxKeyAccessQuota  = &contextKey{"AccessQuota"}
	ctxKeyProjectID    = &contextKey{"ProjectID"}
	ctxKeyComputeUnits = &contextKey{"ComputeUnits"}
	ctxKeyTime         = &contextKey{"Time"}
	ctxKeySpending     = &contextKey{"Spending"}
)

// withSessionType adds the access key to the context.
func withSessionType(ctx context.Context, accessType proto.SessionType) context.Context {
	return context.WithValue(ctx, ctxKeySessionType, accessType)
}

// GetSessionType returns the access key from the context.
func GetSessionType(ctx context.Context) proto.SessionType {
	v, ok := ctx.Value(ctxKeySessionType).(proto.SessionType)
	if !ok {
		return proto.SessionType_Public
	}
	return v
}

// WithAccount adds the account to the context.
func withAccount(ctx context.Context, account string) context.Context {
	return context.WithValue(ctx, ctxKeyAccount, account)
}

// GetAccount returns the account from the context.
func GetAccount(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyAccount).(string)
	return v, ok
}

// withService adds the service to the context.
func withService(ctx context.Context, service string) context.Context {
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

// WithComputeUnits sets the compute units.
func WithComputeUnits(ctx context.Context, cu int64) context.Context {
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
