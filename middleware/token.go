package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

const (
	HeaderSequenceTokenKey = "X-Sequence-Token-Key" // TODO: should we use this header or "Authorization" ? lets discuss
	HeaderOrigin           = "Origin"
)

// Client is the interface that wraps the basic FetchToken, GetUsage and SpendToken methods.
type Client interface {
	FetchToken(ctx context.Context, tokenKey, origin string) (*proto.CachedToken, error)
	GetUsage(ctx context.Context, token *proto.CachedToken, now time.Time) (int64, error)
	SpendToken(ctx context.Context, token *proto.CachedToken, computeUnits int64, now time.Time) (bool, error)
}

// ErrorHandler is a function that handles errors.
type ErrorHandler func(w http.ResponseWriter, err error)

// VerifyToken is a middleware that verifies the token and adds it to the request context.
func VerifyToken(client Client, eh ErrorHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = DefaultErrorHandler
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// skip with no token key
			tokenKey := r.Header.Get(HeaderSequenceTokenKey)
			if tokenKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			token, err := client.FetchToken(ctx, tokenKey, r.Header.Get(HeaderOrigin))
			if err != nil {
				eh(w, err)
				return
			}

			// set token in context
			ctx = WithToken(ctx, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// EnsureUsage is a middleware that checks if the token has enough usage left.
func EnsureUsage(client Client, eh ErrorHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = DefaultErrorHandler
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			token := GetToken(ctx)
			if token == nil {
				next.ServeHTTP(w, r)
				return
			}

			cu := GetComputeUnits(ctx)
			if cu == 0 {
				next.ServeHTTP(w, r)
				return
			}

			usage, err := client.GetUsage(ctx, token, GetTime(ctx))
			if err != nil {
				eh(w, err)
				return
			}
			if usage+cu > token.Limit.HardQuota {
				eh(w, proto.ErrLimitExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SpendUsage spends the usage before calling next handler and sets the result in the context.
func SpendUsage(client Client, eh ErrorHandler) func(next http.Handler) http.Handler {
	if eh == nil {
		eh = DefaultErrorHandler
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			token := GetToken(ctx)
			if token == nil {
				next.ServeHTTP(w, r)
				return
			}

			ok, err := client.SpendToken(ctx, token, GetComputeUnits(ctx), GetTime(ctx))
			if err != nil {
				eh(w, err)
				return
			}

			if !ok {
				eh(w, proto.ErrLimitExceeded)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithResult(ctx)))
		})
	}
}
