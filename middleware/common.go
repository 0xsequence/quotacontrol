package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

const (
	HeaderOrigin = "Origin"
)

// ChainFunc is a function that returns the chain IDs for a given request.
type ChainFunc func(*http.Request) []uint64

// StaticChainIDs always returns the given chainIDs.
func StaticChainIDs(chainIDs ...uint64) ChainFunc {
	return func(*http.Request) []uint64 {
		return chainIDs
	}
}

// ChainFinder is an interface that can find a chain ID by its name or id.
type ChainFinder[T any] interface {
	FindChain(chainID string) (uint64, T, error)
}

// ChainFromPath extracts the chain from the first path segment of the request URL.
func ChainFromPath[T any](finder ChainFinder[T]) func(r *http.Request) []uint64 {
	return func(r *http.Request) []uint64 {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 2 {
			return nil
		}
		chainID, _, err := finder.FindChain(parts[1])
		if err != nil {
			return nil
		}
		return []uint64{chainID}
	}
}

// Options is the configuration for the quota middleware.
type Options struct {
	// BaseRequestCost is the cost of a single request.
	BaseRequestCost int
	// ChainFunc is the function that returns the chain IDs for a given request.
	ChainFunc ChainFunc
	// ErrHandler is the error handler to use when an error occurs.
	ErrHandler func(r *http.Request, w http.ResponseWriter, err error)
}

func (o *Options) ApplyDefaults() {
	// Set default error handler if not provided
	if o.ErrHandler == nil {
		o.ErrHandler = func(r *http.Request, w http.ResponseWriter, err error) {
			proto.RespondWithError(w, err)
		}
	}
	if o.BaseRequestCost < 1 {
		o.BaseRequestCost = 1
	}
}

// Client is the interface that wraps the basic FetchKeyQuota, GetUsage and SpendQuota methods.
type Client interface {
	IsEnabled() bool
	GetDefaultUsage() int64
	GetService() proto.Service
	FetchProjectQuota(ctx context.Context, projectID uint64, chainIDs []uint64, now time.Time) (*proto.AccessQuota, error)
	FetchKeyQuota(ctx context.Context, accessKey, origin string, chainIDs []uint64, now time.Time) (*proto.AccessQuota, error)
	FetchUsage(ctx context.Context, quota *proto.AccessQuota, service *proto.Service, now time.Time) (int64, error)
	CheckPermission(ctx context.Context, projectID uint64, minPermission proto.UserPermission) (bool, error)
	SpendQuota(ctx context.Context, quota *proto.AccessQuota, svc *proto.Service, cost int64, now time.Time) (bool, int64, error)
}

func VerifyChains(ctx context.Context, chainIDs ...uint64) error {
	if len(chainIDs) == 0 {
		return nil
	}
	quota, ok := GetAccessQuota(ctx)
	if !ok {
		return nil
	}
	if err := quota.AccessKey.ValidateChains(chainIDs); err != nil {
		return proto.ErrInvalidChain.WithCause(err)
	}
	return nil
}
