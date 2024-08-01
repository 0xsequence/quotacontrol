package quotacontrol

import (
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	const _CustomErrorMessage = "Custom error message"
	rl := NewHTTPRateLimiter(Config{
		RateLimiter: middleware.RateLimiterConfig{
			Enabled:                 true,
			PublicRequestsPerMinute: 10,
			ErrorMessage:            _CustomErrorMessage,
		},
	}, nil, nil)
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
