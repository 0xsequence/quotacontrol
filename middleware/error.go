package middleware

import (
	"errors"
	"net/http"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/rs/zerolog"
)

var _DefaultErrorHandler = FailOnUnexpectedError(proto.RespondWithError)

func FailOnUnexpectedError(fn func(w http.ResponseWriter, err error)) func(w http.ResponseWriter, _ *http.Request, _ http.Handler, err error) {
	return func(w http.ResponseWriter, _ *http.Request, _ http.Handler, err error) { fn(w, err) }
}

func ContinueOnUnexpectedError(log zerolog.Logger, fn func(w http.ResponseWriter, err error)) func(w http.ResponseWriter, _ *http.Request, next http.Handler, err error) {
	return func(w http.ResponseWriter, r *http.Request, next http.Handler, err error) {
		if !shouldErrorContinue(err) {
			fn(w, err)
			return
		}
		log.Error().Err(err).Str("op", "quota").Msg("-> quotacontrol: unexpected error")
		next.ServeHTTP(w, r.WithContext(WithSkipRateLimit(r.Context())))
	}
}

func shouldErrorContinue(err error) bool {
	w := proto.NewError(err)
	// Unexpected error
	if !errors.As(err, &w) {
		return true
	}
	// QuotaControl server down
	if errors.Is(w, proto.ErrWebrpcRequestFailed) {
		return true
	}
	// QuotaControl method not found
	if errors.Is(w, proto.ErrWebrpcBadRoute) {
		return true
	}
	return false
}
