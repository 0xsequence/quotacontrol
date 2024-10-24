package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

const (
	HeaderAccessKey = "X-Access-Key"
	HeaderOrigin    = "Origin"
)

func errHandler(r *http.Request, w http.ResponseWriter, err error) {
	rpcErr, ok := err.(proto.WebRPCError)
	if !ok {
		rpcErr = proto.ErrWebrpcEndpoint.WithCause(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(rpcErr.HTTPStatus)

	respBody, _ := json.Marshal(rpcErr)
	w.Write(respBody)
}

func KeyFromHeader(r *http.Request) string {
	return r.Header.Get(HeaderAccessKey)
}

// Client is the interface that wraps the basic FetchKeyQuota, GetUsage and SpendQuota methods.
type Client interface {
	IsEnabled() bool
	GetDefaultUsage() int64
	FetchProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error)
	FetchKeyQuota(ctx context.Context, accessKey, origin string, now time.Time) (*proto.AccessQuota, error)
	FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error)
	CheckPermission(ctx context.Context, projectID uint64, minPermission proto.UserPermission) (bool, error)
	SpendQuota(ctx context.Context, quota *proto.AccessQuota, cost int64, now time.Time) (bool, int64, error)
}
