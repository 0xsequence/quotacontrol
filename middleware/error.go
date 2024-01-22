package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/logger"
)

var _DefaultErrorHandler = FailOnUnexpectedError(proto.RespondWithError)

func FailOnUnexpectedError(fn func(w http.ResponseWriter, err error)) func(w http.ResponseWriter, _ *http.Request, _ http.Handler, err error) {
	return func(w http.ResponseWriter, _ *http.Request, _ http.Handler, err error) { fn(w, err) }
}

func ContinueOnUnexpectedError(log logger.Logger, fn func(w http.ResponseWriter, err error)) func(w http.ResponseWriter, _ *http.Request, next http.Handler, err error) {
	return func(w http.ResponseWriter, r *http.Request, next http.Handler, err error) {
		if !errors.Is(err, proto.ErrAccessKeyNotFound) {
			log.With("err", err, "op", "quota").Warn("-> quotacontrol: unexpected error")
		}

		if !shouldErrorContinue(log, err) {
			// trigger error response fn
			fn(w, err)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithSkipRateLimit(r.Context())))
	}
}

func ContinueOnAnyError(log logger.Logger, fn func(w http.ResponseWriter, err error)) func(w http.ResponseWriter, _ *http.Request, next http.Handler, err error) {
	return func(w http.ResponseWriter, r *http.Request, next http.Handler, err error) {
		// all errors are okay, except for ErrLimitExceeded
		if errors.Is(err, proto.ErrLimitExceeded) {
			// trigger error response fn
			fn(w, err)
			return
		}

		log.With("err", err, "op", "quota").Warn("-> quotacontrol: unexpected error -- continuing anyways")
		next.ServeHTTP(w, r.WithContext(WithSkipRateLimit(r.Context())))
	}
}

func shouldErrorContinue(log logger.Logger, err error) bool {
	w := proto.WebRPCError{}

	// Unexpected error
	if !errors.As(err, &w) {
		// Sample log of unexpected errors (every 10 seconds)
		if time.Now().Second()%10 == 0 {
			log.With("err", err).Error("quotacontrol: unexpected error, allowing all traffic")
		}
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

	// QuotaControl timed out
	if errors.Is(w, proto.ErrTimeout) {
		return true
	}

	return false
}
