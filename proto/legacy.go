package proto

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type legacyError struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Cause  string `json:"cause,omitempty"`
	Msg    string `json:"msg"`
	Error  string `json:"error"`
}

func (w WebRPCError) getLegacyPayload() legacyError {
	code, ok := _StatusCode[w.HTTPStatus]
	if !ok {
		code = "invalid"
	}
	return legacyError{
		Status: w.HTTPStatus,
		Code:   code,
		Cause:  w.Cause,
		Msg:    w.Message,
		Error:  fmt.Sprintf("webrpc %s error: %s", code, w.Message),
	}
}

var _StatusCode = map[int]string{
	422: "fail",
	400: "invalid argument",
	408: "deadline exceeded",
	404: "not found",
	403: "permission denied",
	401: "unauthenticated",
	412: "failed precondition",
	409: "aborted",
	501: "unimplemented",
	500: "internal",
	503: "unavailable",
	200: "",
}

func RespondWithLegacyError(w http.ResponseWriter, err error) {
	rpcErr, ok := err.(WebRPCError)
	if !ok {
		rpcErr = ErrorWithCause(ErrWebrpcEndpoint, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(rpcErr.HTTPStatus)

	respBody, _ := json.Marshal(rpcErr.getLegacyPayload())
	w.Write(respBody)
}
