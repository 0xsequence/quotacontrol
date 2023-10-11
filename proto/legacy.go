package proto

import (
	"errors"
	"fmt"
)

type LegacyError struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Cause  string `json:"cause,omitempty"`
	Msg    string `json:"msg"`
	Error  string `json:"error"`
}

func NewLegacyError(err error) LegacyError {
	w := WebRPCError{}
	if !errors.As(err, &w) {
		w = WebRPCError{HTTPStatus: 500, Message: err.Error(), Cause: "internal"}
	}
	return w.GetLegacyPayload()
}

func (w WebRPCError) GetLegacyPayload() LegacyError {
	code := codeFromHTTPStatus(w.HTTPStatus)
	return LegacyError{
		Status: w.HTTPStatus,
		Code:   code,
		Cause:  w.Cause,
		Msg:    w.Message,
		Error:  fmt.Sprintf("webrpc %s error: %s", code, w.Message),
	}
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
