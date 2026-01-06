package middleware_test

import (
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/assert"
)

func makeIP(ipv6 bool) net.IP {
	size := 4
	if ipv6 {
		size = 16
	}
	buf := make([]byte, size)
	for i := 0; i < size/4; i++ {
		binary.LittleEndian.PutUint32(buf[i*4:], rand.Uint32())
	}
	return net.IP(buf)
}

func TestRateLimiter(t *testing.T) {
	const (
		_TestHeader      = "X-Test-Header"
		_TestHeaderValue = "test"
	)
	eh := func(r *http.Request, w http.ResponseWriter, err error) {
		w.Header().Set(_TestHeader, _TestHeaderValue)
		proto.RespondWithError(w, err)
	}

	client := quotacontrol.NewClient(slog.Default(), proto.Service_API, quotacontrol.Config{}, nil)

	rl := middleware.RateLimit(client, middleware.RateLimitConfig{
		Enabled:   true,
		PublicRPM: 10,
	}, nil, middleware.Options{ErrHandler: eh})
	handler := rl(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for i := 0; i < 10; i++ {
		ipAddress := makeIP(false).String()
		for i := 0; i < 20; i++ {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = ipAddress
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if i < 10 {
				assert.Equal(t, http.StatusOK, w.Code)
				continue
			}
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
			assert.Equal(t, _TestHeaderValue, w.Header().Get(_TestHeader))
			err := proto.WebRPCError{}
			assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &err))
		}
	}

	for i := 0; i < 80; i++ {
		ipAddress := makeIP(true).String()
		for i := 0; i < 20; i++ {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = ipAddress
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if i < 10 {
				assert.Equal(t, http.StatusOK, w.Code)
				continue
			}
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
			assert.Equal(t, _TestHeaderValue, w.Header().Get(_TestHeader))
			err := proto.WebRPCError{}
			assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &err))
		}
	}

}
