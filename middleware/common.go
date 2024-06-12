package middleware

import (
	"context"
	"slices"
	"strings"
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
	GetDefaultUsage() int64
	FetchProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error)
	FetchKeyQuota(ctx context.Context, accessKey, origin string, now time.Time) (*proto.AccessQuota, error)
	FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error)
	FetchPermission(ctx context.Context, projectID uint64, userID string, useCache bool) (proto.UserPermission, *proto.ResourceAccess, error)
	SpendQuota(ctx context.Context, quota *proto.AccessQuota, computeUnits int64, now time.Time) (bool, error)
}

type ACL map[string]map[string][]proto.SessionType

func (acl ACL) authorize(r *rcpRequest, sessionType proto.SessionType) error {
	if r.Package != "rpc" {
		return proto.ErrUnauthorized
	}

	serviceACL, ok := acl[r.Service]
	if !ok {
		return proto.ErrUnauthorized
	}

	// get method's ACL
	perms, ok := serviceACL[r.Method]
	if !ok {
		// unable to find method in rules list. deny.
		return proto.ErrUnauthorized
	}

	// authorize using methods's ACL
	if !slices.Contains(perms, sessionType) {
		return proto.ErrUnauthorized
	}

	return nil
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
