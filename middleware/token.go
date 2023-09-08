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

type Client interface {
	FetchToken(ctx context.Context, tokenKey, origin string) (*proto.CachedToken, error)
	GetUsage(ctx context.Context, token *proto.CachedToken, now time.Time) (int64, error)
	SpendToken(ctx context.Context, token *proto.CachedToken, computeUnits int64, now time.Time) (bool, error)
}

// VerifyToken is a middleware that verifies the token and adds it to the request context.
func VerifyToken(client Client) func(next http.Handler) http.Handler {
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
				proto.RespondWithError(w, err)
				return
			}

			// set token in context
			ctx = WithToken(ctx, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// EnsureUsage is a middleware that checks if the token has enough usage left.
func EnsureUsage(client Client) func(next http.Handler) http.Handler {
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
				proto.RespondWithError(w, err)
				return
			}
			if usage+cu > token.Limit.HardQuota {
				proto.RespondWithError(w, proto.ErrLimitExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SpendUsage spends the usage before calling next handler and sets the result in the context.
func SpendUsage(client Client) func(next http.Handler) http.Handler {
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
				proto.RespondWithError(w, err)
				return
			}

			if !ok {
				proto.RespondWithError(w, proto.ErrLimitExceeded)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithResult(ctx)))
		})
	}
}
