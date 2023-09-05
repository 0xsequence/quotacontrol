package middleware

import (
	"context"
	"net/http"

	"github.com/0xsequence/quotacontrol/proto"
)

const (
	HeaderSequenceTokenKey = "X-Sequence-Token-Key" // TODO: should we use this header or "Authorization" ? lets discuss
	HeaderOrigin           = "Origin"
)

type QuotaClient interface {
	FetchToken(ctx context.Context, tokenKey, origin string) (*proto.CachedToken, error)
	UseToken(ctx context.Context, tokenKey, origin string) (bool, error)
}

type ContextFunc func(context.Context) context.Context

func NoAction(ctx context.Context) context.Context { return ctx }

type Middleware func(http.Handler) http.Handler

func UseToken(client QuotaClient, noToken Middleware, onSuccess ContextFunc) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenKey := r.Header.Get(HeaderSequenceTokenKey)
			if tokenKey == "" {
				handler := next
				if noToken != nil {
					handler = noToken(handler)
				}
				handler.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			ok, err := client.UseToken(ctx, tokenKey, r.Header.Get(HeaderOrigin))
			if err != nil {
				proto.RespondWithError(w, err)
				return
			}
			if !ok {
				proto.RespondWithError(w, proto.ErrLimitExceeded)
				return
			}
			next.ServeHTTP(w, r.WithContext(onSuccess(ctx)))
		})
	}
}

func CheckToken(client QuotaClient) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenKey := r.Header.Get(HeaderSequenceTokenKey)
			if tokenKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			if _, err := client.FetchToken(r.Context(), tokenKey, r.Header.Get(HeaderOrigin)); err != nil {
				proto.RespondWithError(w, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
