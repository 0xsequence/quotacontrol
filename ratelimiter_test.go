package quotacontrol

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"github.com/go-chi/httprate"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	const _CustomErrorMessage = "Custom error message"
	rl := NewHTTPRateLimiter(Config{
		RateLimiter: RateLimiterConfig{
			Enabled:                 true,
			PublicRequestsPerMinute: 10,
			ErrorMessage:            _CustomErrorMessage,
		},
	}, nil)
	handler := rl(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	buf := make([]byte, 4)
	for i := 0; i < 10; i++ {
		ip := rand.Uint32()
		binary.LittleEndian.PutUint32(buf, ip)
	}
	ipAddress := net.IP(buf).String()
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
		err := proto.WebRPCError{}
		assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &err))
		assert.Equal(t, err.Message, _CustomErrorMessage)
	}
}

func TestOverridePublicRateLimiting(t *testing.T) {
	rl := NewHTTPRateLimiter(Config{
		RateLimiter: RateLimiterConfig{
			Enabled:                 true,
			PublicRequestsPerMinute: 10,
			ErrorMessage:            "Custom error",
		},
	}, nil)
	handler := rl(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	buf := make([]byte, 4)
	for i := 11; i < 20; i++ {
		ip := rand.Uint32()
		binary.LittleEndian.PutUint32(buf, ip)
	}
	ipAddress := net.IP(buf).String()
	srv := httptest.NewServer(handler)
	for i := 0; i < 5; i++ {
		ctx := httprate.WithRequestLimit(context.Background(), 2)
		req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
		req.RemoteAddr = ipAddress
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		code := http.StatusOK
		if i >= 2 {
			code = http.StatusTooManyRequests
		}
		assert.Equal(t, code, w.Code, "call #%d", i)
	}
}
