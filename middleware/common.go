package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

type ErrHandler func(w http.ResponseWriter, err error)

// Client is the interface that wraps the basic FetchKeyQuota, GetUsage and SpendQuota methods.
type Client interface {
	IsEnabled() bool
	GetDefaultUsage() int64
	FetchProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error)
	FetchKeyQuota(ctx context.Context, accessKey, origin string, now time.Time) (*proto.AccessQuota, error)
	FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error)
	CheckPermission(ctx context.Context, projectID uint64, minPermission proto.UserPermission) (bool, error)
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

func (s ServiceConfig[T]) GetConfig(r *rcpRequest) (v T, ok bool) {
	if s == nil || r.Package != "rpc" {
		return v, false
	}
	serviceCfg, ok := s[r.Service]
	if !ok {
		return v, false
	}
	methodCfg, ok := serviceCfg[r.Method]
	if !ok {
		return v, false
	}
	return methodCfg, true
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

// swapHeader swaps the header from one key to another.
func swapHeader(h http.Header, from, to string) {
	if v := h.Get(from); v != "" {
		h.Set(to, v)
		h.Del(from)
	}
}

// ACL is a list of session types, encoded as a bitfield.
// SessionType(n) is represented by n=-the bit.
type ACL uint64

func NewACL(t ...proto.SessionType) ACL {
	var types ACL
	for _, v := range t {
		types = types.And(v)
	}
	return types
}

func (t ACL) And(types ...proto.SessionType) ACL {
	for _, v := range types {
		t |= 1 << v
	}
	return t
}

func (t ACL) Includes(session proto.SessionType) bool {
	return t|ACL(1<<session) == t
}
