package quotacontrol_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/alicebob/miniredis/v2"
	"github.com/goware/cachestore/redis"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_Port      = ":8080"
	_ProjectID = uint64(777)
	_Tokens    = []string{"abc", "cde"}
	_Now       = time.Date(2023, time.June, 26, 0, 0, 0, 0, time.Local)
)

type notifier map[string]struct{}

func (n notifier) Notify(token *proto.AccessToken) error {
	n[token.TokenKey] = struct{}{}
	return nil
}

func TestMiddlewareUseToken(t *testing.T) {
	s := miniredis.NewMiniRedis()
	s.Start()
	defer s.Close()
	redisClient := redisclient.NewClient(&redisclient.Options{Addr: s.Addr()})
	cache := quotacontrol.NewRedisCache(redisClient, time.Minute)

	store := quotacontrol.NewMemoryStore()
	cfg := quotacontrol.Config{
		Enabled:    true,
		URL:        `http://localhost` + _Port,
		UpdateFreq: quotacontrol.Duration{time.Minute},
		Redis: redis.Config{
			Host: s.Host(),
			Port: uint16(s.Server().Addr().Port),
		},
		RateLimiter: quotacontrol.RateLimiterConfig{
			Enabled:                 true,
			PublicRequestsPerMinute: 10,
		},
	}

	rateLimiter := quotacontrol.NewPublicRateLimiter(cfg)
	notifier := notifier{}
	quotaClient, err := quotacontrol.NewClient(zerolog.New(zerolog.Nop()), proto.Ptr(proto.Service_Indexer), notifier, cfg)
	require.NoError(t, err)

	ctx := context.Background()

	go quotaClient.Run(ctx)

	l, err := net.Listen("tcp", _Port)
	require.NoError(t, err)

	go func() {
		serviceServer := quotacontrol.NewQuotaControl(cache, store, store, store)
		err = http.Serve(l, proto.NewQuotaControlServer(serviceServer))
		require.NoError(t, err)
	}()

	// populate store
	limit := proto.Limit{
		FreeCU:    5,
		RateLimit: 100,
		SoftQuota: 7,
		HardQuota: 10,
	}
	store.InsertAccessLimit(ctx, _ProjectID, &limit)
	store.InsertToken(ctx, &proto.AccessToken{Active: true, TokenKey: _Tokens[0], ProjectID: _ProjectID})
	store.InsertToken(ctx, &proto.AccessToken{Active: true, TokenKey: _Tokens[1], ProjectID: _ProjectID})
	store.InsertToken(ctx, &proto.AccessToken{Active: true, TokenKey: "mno", ProjectID: _ProjectID + 1})
	store.InsertToken(ctx, &proto.AccessToken{Active: true, TokenKey: "xyz", ProjectID: _ProjectID + 1})

	middleware := quotacontrol.NewMiddleware(quotaClient, rateLimiter, quotacontrol.NoAction)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	ctx = quotacontrol.WithTime(ctx, _Now)
	for i := 0; i < 10; i++ {
		ok, err := executeRequest(ctx, handler, _Tokens[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)

		_, ok = notifier[_Tokens[0]]
		assert.Equal(t, i >= int(limit.SoftQuota), ok, i)
	}
	for i := 0; i < 5; i++ {
		ok, err := executeRequest(ctx, handler, _Tokens[0], "")
		assert.ErrorIs(t, err, proto.ErrLimitExceeded)
		assert.False(t, ok)
	}

	// Add Quota and try again, it should fail because of rate limit
	store.InsertAccessLimit(ctx, _ProjectID, &proto.Limit{
		RateLimit: 100,
		SoftQuota: 5,
		HardQuota: 110,
	})
	cache.DeleteToken(ctx, _Tokens[0])

	ok, err := executeRequest(ctx, handler, _Tokens[0], "")
	assert.NoError(t, err)
	assert.True(t, ok)

	for i, max := 0, cfg.RateLimiter.PublicRequestsPerMinute*2; i < max; i++ {
		ok, err := executeRequest(ctx, handler, "", "")
		if i < cfg.RateLimiter.PublicRequestsPerMinute {
			assert.NoError(t, err)
			assert.True(t, ok)
		} else {
			assert.ErrorIs(t, err, proto.ErrLimitExceeded)
			assert.False(t, ok)
		}
	}

	quotaClient.Stop(ctx)
	usage, err := store.GetAccountTotalUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, &proto.AccessTokenUsage{ValidCompute: 5, OverCompute: 6, LimitedCompute: 5}, &usage)

}

func executeRequest(ctx context.Context, handler http.Handler, token, origin string) (bool, error) {
	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Real-IP", "127.0.0.1")
	if token != "" {
		req.Header.Set(quotacontrol.HeaderSequenceTokenKey, token)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	status := rr.Result().StatusCode
	if status < http.StatusOK || status >= http.StatusBadRequest {
		return false, proto.ErrLimitExceeded
	}
	return true, nil
}
