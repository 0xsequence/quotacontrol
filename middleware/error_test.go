package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/logger"
)

func TestContinueOnUnexpectedError(t *testing.T) {
	t.Run("ShouldCallNext", func(t *testing.T) {
		fn := func(w http.ResponseWriter, err error) {
			t.Error("Unexpected call")
		}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		for _, err := range []error{
			proto.ErrWebrpcRequestFailed, proto.ErrWebrpcBadRoute, errors.New("unexpected error"),
		} {
			req, _ := http.NewRequest("GET", "/", nil)
			middleware.ContinueOnUnexpectedError(logger.Nop(), fn)(httptest.NewRecorder(), req, next, err)
		}
	})
	t.Run("ShouldError", func(t *testing.T) {
		fn := func(w http.ResponseWriter, err error) {}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Unexpected call")
		})
		for _, status := range []int{403, 408, 429} {
			middleware.ContinueOnUnexpectedError(logger.Nop(), fn)(httptest.NewRecorder(), nil, next, proto.WebRPCError{
				HTTPStatus: status,
			})
		}
	})
}
