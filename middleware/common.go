package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/logger"
)

const (
	HeaderOrigin = "Origin"
)

type ChainIDsFunc func(*http.Request) []uint64

// StaticChainIDs returns a ChainIDsFunc that always returns the given chainIDs.
func StaticChainIDs(chainIDs ...uint64) ChainIDsFunc {
	return func(*http.Request) []uint64 {
		return chainIDs
	}
}

// ChainFromPath returns a ChainIDsFunc that extracts the chain ID from the first part of the URL path.
func ChainFromPath(r *http.Request) []uint64 {
	chainID, err := strconv.ParseUint(strings.Split(r.URL.Path, "/")[0], 10, 64)
	if err != nil {
		return nil
	}
	return []uint64{chainID}
}

type Options struct {
	Logger          logger.Logger
	BaseRequestCost int
	ChainIDsFunc    ChainIDsFunc
	ErrHandler      func(r *http.Request, w http.ResponseWriter, err error)
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
	proto.RespondWithError(w, err)
}

// Client is the interface that wraps the basic FetchKeyQuota, GetUsage and SpendQuota methods.
type Client interface {
	IsEnabled() bool
	GetDefaultUsage() int64
	FetchProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error)
	FetchKeyQuota(ctx context.Context, accessKey, origin string, chainIDs []uint64, now time.Time) (*proto.AccessQuota, error)
	FetchUsage(ctx context.Context, quota *proto.AccessQuota, now time.Time) (int64, error)
	CheckPermission(ctx context.Context, projectID uint64, minPermission proto.UserPermission) (bool, error)
	SpendQuota(ctx context.Context, quota *proto.AccessQuota, cost int64, now time.Time) (bool, int64, error)
}
