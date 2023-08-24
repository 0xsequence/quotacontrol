package quotacontrol_test

import (
	"context"
	"net"
	"net/http"
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
	_Service   = proto.Service_Indexer
	_Tokens    = []string{"abc", "cde"}
	_Now       = time.Date(2023, time.June, 26, 0, 0, 0, 0, time.Local)
)

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
	}
	middlewareClient, err := quotacontrol.NewClient(zerolog.New(zerolog.Nop()), &_Service, cfg)
	require.NoError(t, err)

	ctx := context.Background()

	go middlewareClient.Run(ctx)

	l, err := net.Listen("tcp", _Port)
	require.NoError(t, err)

	go func() {
		serviceServer := quotacontrol.NewQuotaControl(cache, store, store, store)
		err = http.Serve(l, proto.NewQuotaControlServer(serviceServer))
		require.NoError(t, err)
	}()

	// populate store
	store.InsertAccessLimit(ctx, _ProjectID, &proto.ServiceLimit{
		Service:                 &_Service,
		ComputeRateLimit:        100,
		ComputeMonthlyQuota:     5,
		ComputeMonthlyHardQuota: 10,
	})
	store.InsertToken(ctx, &proto.AccessToken{Active: true, TokenKey: _Tokens[0], ProjectID: _ProjectID})
	store.InsertToken(ctx, &proto.AccessToken{Active: true, TokenKey: _Tokens[1], ProjectID: _ProjectID})
	store.InsertToken(ctx, &proto.AccessToken{Active: true, TokenKey: "mno", ProjectID: _ProjectID + 1})
	store.InsertToken(ctx, &proto.AccessToken{Active: true, TokenKey: "xyz", ProjectID: _ProjectID + 1})

	ctx = quotacontrol.WithTime(ctx, _Now)
	for i := 0; i < 10; i++ {
		ok, err := middlewareClient.UseToken(ctx, _Tokens[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
	}
	for i := 0; i < 5; i++ {
		ok, err := middlewareClient.UseToken(ctx, _Tokens[0], "")
		assert.ErrorIs(t, err, proto.ErrLimitExceeded)
		assert.False(t, ok)
	}

	// Add Quota and try again, it should fail because of rate limit
	store.InsertAccessLimit(ctx, _ProjectID, &proto.ServiceLimit{
		Service:                 &_Service,
		ComputeRateLimit:        100,
		ComputeMonthlyQuota:     5,
		ComputeMonthlyHardQuota: 110,
	})
	cache.DeleteToken(ctx, _Tokens[0])

	ok, err := middlewareClient.UseToken(ctx, _Tokens[0], "")
	assert.NoError(t, err)
	assert.True(t, ok)

	middlewareClient.Stop(ctx)
	usage, _ := store.GetAccountTotalUsage(ctx, _ProjectID, _Service, _Now.Add(-time.Hour), _Now.Add(time.Hour))
	assert.Equal(t, &proto.AccessTokenUsage{ValidCompute: 5, OverCompute: 6, LimitedCompute: 5}, &usage)
}
