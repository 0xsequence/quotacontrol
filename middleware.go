package quotacontrol

import (
	"context"
	"net/http"

	"github.com/0xsequence/quotacontrol/proto"
)

const (
	HeaderSequenceTokenKey = "X-Sequence-Token-Key" // TODO: should we use this header or "Authorization" ? lets discuss
	HeaderOrigin           = "Origin"
)

type ContextFunc func(context.Context) context.Context

func NoAction(ctx context.Context) context.Context { return ctx }

type Middleware func(http.Handler) http.Handler

func NewMiddleware(client *Client, noToken Middleware, onSuccess ContextFunc) Middleware {
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
