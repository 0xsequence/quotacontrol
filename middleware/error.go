package middleware

import (
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
		if werr := proto.NewError(err); werr.HTTPStatus < http.StatusInternalServerError && werr.HTTPStatus != http.StatusNotFound {
			fn(w, err)
			return
		}
		log.Error().Err(err).Str("op", "quota").Msg("-> quotacontrol: unexpected error")
		next.ServeHTTP(w, r)
	}
}
