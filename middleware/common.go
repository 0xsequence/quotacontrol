package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"

	"github.com/0xsequence/authcontrol"
)

const (
	HeaderOrigin = "Origin"
)

type Options struct {
	Logger          *slog.Logger
	BaseRequestCost int
	ErrHandler      authcontrol.ErrHandler
}

func (o *Options) ApplyDefaults() {
	// Set default error handler if not provided
	if o.ErrHandler == nil {
		o.ErrHandler = errHandler
	}
	if o.BaseRequestCost < 1 {
		o.BaseRequestCost = 1
	}
}

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
