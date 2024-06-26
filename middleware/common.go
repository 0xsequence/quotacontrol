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
