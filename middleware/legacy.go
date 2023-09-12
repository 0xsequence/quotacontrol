package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/0xsequence/quotacontrol/proto"
)

// DefaultErrorHandler is the default function that handles errors.
func DefaultErrorHandler(w http.ResponseWriter, err error) {
	proto.RespondWithError(w, err)
}

// LegacyErrorHandler is a function that handles errors for older versions of WebRPC.
func LegacyErrorHandler(w http.ResponseWriter, err error) {
	type payload struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
		Cause  string `json:"cause,omitempty"`
		Msg    string `json:"msg"`
		Error  string `json:"error"`
	}
	rpcErr := proto.WebRPCError{}
	if !errors.As(err, &rpcErr) {
		rpcErr = proto.WebRPCError{
			HTTPStatus: http.StatusInternalServerError,
			Message:    err.Error(),
			Cause:      codeFromHTTPStatus(rpcErr.HTTPStatus),
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(rpcErr.HTTPStatus)
	code := codeFromHTTPStatus(rpcErr.HTTPStatus)
	respBody, _ := json.Marshal(payload{
		Status: rpcErr.HTTPStatus,
		Code:   code,
		Cause:  rpcErr.Cause,
		Msg:    rpcErr.Message,
		Error:  fmt.Sprintf("webrpc %s error: %s", code, rpcErr.Message),
	})
	w.Write(respBody)
}

func codeFromHTTPStatus(code int) string {
	switch code {
	case 422:
		return "fail"
	case 400:
		return "invalid argument"
	case 408:
		return "deadline exceeded"
	case 404:
		return "not found"
	case 403:
		return "permission denied"
	case 401:
		return "unauthenticated"
	case 412:
		return "failed precondition"
	case 409:
		return "aborted"
	case 501:
		return "unimplemented"
	case 500:
		return "internal"
	case 503:
		return "unavailable"
	case 200:
		return ""
	default:
		return "invalid"
	}
}
