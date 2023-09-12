package quotacontrol_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/alicebob/miniredis/v2"

	"github.com/go-chi/chi/v5"
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

	cfg = Config{
		Enabled:    true,
		URL:        `http://localhost` + _Port,
		UpdateFreq: Duration{time.Minute},
		RateLimiter: RateLimiterConfig{
			Enabled:                 true,
			PublicRequestsPerMinute: 10,
		},
	}
)

func TestMiddlewareUseToken(t *testing.T) {
	limit := proto.Limit{FreeCU: 5, RateLimit: 100, SoftQuota: 7, HardQuota: 10}
	token := proto.AccessToken{Active: true, TokenKey: _Tokens[0], ProjectID: _ProjectID}
	expectedCounter := proto.AccessTokenUsage{}

	s := miniredis.NewMiniRedis()
	s.Start()
	t.Cleanup(s.Close)

	cfg.Redis.Host = s.Host()
	cfg.Redis.Port = uint16(s.Server().Addr().Port)

	redisClient := redisclient.NewClient(&redisclient.Options{Addr: s.Addr()})
	cache := NewRedisCache(redisClient, time.Minute)
	notifier := make(notifier)
	store := NewMemoryStore()
	// populate store
	ctx := context.Background()
	store.SetAccessLimit(ctx, _ProjectID, &limit)
	store.InsertToken(ctx, &token)
	client := NewClient(zerolog.Nop(), proto.Service_Indexer, notifier, cfg)

	server := http.Server{
		Addr:    _Port,
		Handler: proto.NewQuotaControlServer(NewQuotaControl(cache, store, store, store)),
	}
	go func() { require.ErrorIs(t, server.ListenAndServe(), http.ErrServerClosed) }()
	defer server.Close()

	router := chi.NewRouter()
	// we set the compute units to 2, then in another handler we remove 1 before spending
	router.Use(middleware.ChangeContext(func(ctx context.Context) context.Context {
		return middleware.WithComputeUnits(ctx, 2)
	}))
	router.Use(middleware.VerifyToken(client, nil))
	router.Use(NewPublicRateLimiter(cfg))
	router.Use(middleware.ChangeContext(func(ctx context.Context) context.Context {
		return middleware.AddComputeUnits(ctx, -1)
	}))
	router.Use(middleware.SpendUsage(client, nil))

	var counter int64
	router.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		// up the counter only if quota control run
		if middleware.GetResult(r.Context()) {
			atomic.AddInt64(&counter, 1)
		}
		w.WriteHeader(http.StatusOK)
	})

	t.Run("WithToken", func(t *testing.T) {
		go client.Run(context.Background())

		ctx := middleware.WithTime(context.Background(), _Now)
		for i := 0; i < 15; i++ {
			ok, err := executeRequest(ctx, router, _Tokens[0], "")
			if i >= int(limit.HardQuota) {
				assert.ErrorIs(t, err, proto.ErrLimitExceeded)
				assert.False(t, ok)
				continue
			}
			assert.NoError(t, err)
			assert.True(t, ok)

			_, ok = notifier[_Tokens[0]]
			assert.Equal(t, i >= int(limit.SoftQuota), ok, i)
		}

		client.Stop(context.Background())
		usage, err := store.GetAccountTotalUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		expectedCounter.Add(proto.AccessTokenUsage{ValidCompute: 5, OverCompute: 5, LimitedCompute: 5})
		assert.Equal(t, int64(expectedCounter.ValidCompute+expectedCounter.OverCompute), atomic.LoadInt64(&counter))
		assert.Equal(t, &expectedCounter, &usage)
	})

	// change limits
	store.SetAccessLimit(ctx, _ProjectID, &proto.Limit{RateLimit: 100, SoftQuota: 5, HardQuota: 110})
	cache.DeleteToken(ctx, _Tokens[0])

	t.Run("ChangeLimits", func(t *testing.T) {
		go client.Run(context.Background())
		ctx := middleware.WithTime(context.Background(), _Now)

		ok, err := executeRequest(ctx, router, _Tokens[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)

		client.Stop(context.Background())
		usage, err := store.GetAccountTotalUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		expectedCounter.Add(proto.AccessTokenUsage{ValidCompute: 0, OverCompute: 1, LimitedCompute: 0})
		assert.Equal(t, int64(expectedCounter.ValidCompute+expectedCounter.OverCompute), atomic.LoadInt64(&counter))
		assert.Equal(t, &expectedCounter, &usage)
	})

	t.Run("PublicRateLimit", func(t *testing.T) {
		go client.Run(context.Background())
		ctx := middleware.WithTime(context.Background(), _Now)

		for i, max := 0, cfg.RateLimiter.PublicRequestsPerMinute*2; i < max; i++ {
			ok, err := executeRequest(ctx, router, "", "")
			if i < cfg.RateLimiter.PublicRequestsPerMinute {
				assert.NoError(t, err, i)
				assert.True(t, ok, i)
			} else {
				assert.ErrorIs(t, err, proto.ErrLimitExceeded, i)
				assert.False(t, ok, i)
			}
		}

		client.Stop(context.Background())
		usage, err := store.GetAccountTotalUsage(ctx, _ProjectID, proto.Ptr(proto.Service_Indexer), _Now.Add(-time.Hour), _Now.Add(time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, int64(expectedCounter.ValidCompute+expectedCounter.OverCompute), atomic.LoadInt64(&counter))
		assert.Equal(t, &expectedCounter, &usage)
	})

}

func executeRequest(ctx context.Context, handler http.Handler, token, origin string) (bool, error) {
	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Real-IP", "127.0.0.1")
	if token != "" {
		req.Header.Set(middleware.HeaderSequenceTokenKey, token)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	status := rr.Result().StatusCode
	if status < http.StatusOK || status >= http.StatusBadRequest {
		return false, proto.ErrLimitExceeded
	}
	return true, nil
}

type notifier map[string]struct{}

func (n notifier) Notify(token *proto.AccessToken) error {
	n[token.TokenKey] = struct{}{}
	return nil
}
