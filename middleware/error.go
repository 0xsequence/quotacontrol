package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/0xsequence/quotacontrol/proto"
)

// DefaultErrorHandler is the default function that handles errors. It ignores next.
func DefaultErrorHandler(w http.ResponseWriter, _ *http.Request, _ http.Handler, err error) {
	proto.RespondWithError(w, err)
}

// LegacyErrorHandler is a function that handles errors for older versions of WebRPC.
func LegacyErrorHandler(w http.ResponseWriter, _ *http.Request, _ http.Handler, err error) {
	legacyError := proto.NewLegacyError(err)
	w.Header().Set("Content-Type", "application/json")
	respBody, _ := json.Marshal(legacyError)
	w.WriteHeader(legacyError.Status)
	w.Write(respBody)
}
