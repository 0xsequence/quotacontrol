package quotacontrol_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/jwtauth/v5"
	"github.com/goware/logger"

	"github.com/goware/cachestore/redis"
	"github.com/stretchr/testify/require"
)

func newConfig() quotacontrol.Config {
	return quotacontrol.Config{
		Enabled:    true,
		UpdateFreq: quotacontrol.Duration{time.Minute},
		Redis: redis.Config{
			Enabled: true,
		},
		RateLimiter: middleware.RLConfig{
			Enabled: true,
		},
	}
}

func newQuotaClient(cfg quotacontrol.Config, service proto.Service) *quotacontrol.Client {
	logger := logger.NewLogger(logger.LogLevel_DEBUG).With(slog.String("client", "client"))
	return quotacontrol.NewClient(logger, service, cfg, nil)
}

type hitCounter int64

func (c *hitCounter) GetValue() int64 {
	return atomic.LoadInt64((*int64)(c))
}

func (c *hitCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64((*int64)(c), 1)
	w.WriteHeader(http.StatusOK)
}

type spendingCounter int64

func (c *spendingCounter) GetValue() int64 {
	return atomic.LoadInt64((*int64)(c))
}

func (c *spendingCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// up the counter only if quota control run
	if middleware.HasSpending(r.Context()) {
		atomic.AddInt64((*int64)(c), 1)
	}
	w.WriteHeader(http.StatusOK)
}

func mustJWT(t *testing.T, auth *jwtauth.JWTAuth, claims map[string]any) string {
	t.Helper()
	if claims == nil {
		return ""
	}
	_, token, err := auth.Encode(claims)
	require.NoError(t, err)
	return token
}

func executeRequest(ctx context.Context, handler http.Handler, path, accessKey, jwt string) (bool, http.Header, error) {
	req, err := http.NewRequest("POST", path, nil)
	if err != nil {
		return false, nil, err
	}
	req.Header.Set("X-Real-IP", "127.0.0.1")
	if accessKey != "" {
		req.Header.Set(middleware.HeaderAccessKey, accessKey)
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Result().StatusCode; status < http.StatusOK || status >= http.StatusBadRequest {
		w := proto.WebRPCError{}
		json.Unmarshal(rr.Body.Bytes(), &w)
		return false, rr.Header(), w
	}

	return true, rr.Header(), nil
}

type addCost int64

func (i addCost) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(middleware.AddCost(r.Context(), int64(i))))
	})
}
