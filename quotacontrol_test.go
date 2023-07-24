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

type mockLimits map[uint64][]*proto.ServiceLimit

func (m mockLimits) GetAccessLimit(ctx context.Context, dappID uint64) ([]*proto.ServiceLimit, error) {
	limit, ok := m[dappID]
	if !ok {
		return nil, proto.ErrTokenNotFound
	}
	return limit, nil
}

type mockTokens map[string]*proto.AccessToken

func (m mockTokens) FindByTokenKey(ctx context.Context, tokenKey string) (*proto.AccessToken, error) {
	token, ok := m[tokenKey]
	if !ok {
		return nil, proto.ErrTokenNotFound
	}
	return token, nil
}

type mockUsage struct {
	tokens map[string]*proto.AccessToken
	usage  map[string]*proto.AccessTokenUsage
}

func (m mockUsage) GetAccountTotalUsage(ctx context.Context, dappID uint64, service proto.Service, min, max time.Time) (proto.AccessTokenUsage, error) {
	usage := proto.AccessTokenUsage{}
	for _, v := range m.tokens {
		if v.DappID == dappID {
			u, ok := m.usage[v.TokenKey]
			if !ok {
				continue
			}
			usage.Add(*u)
		}
	}
	return usage, nil
}

func (m mockUsage) UpdateTokenUsage(ctx context.Context, tokenKey string, service proto.Service, time time.Time, usage proto.AccessTokenUsage) error {
	if _, ok := m.usage[tokenKey]; !ok {
		m.usage[tokenKey] = &usage
		return nil
	}
	m.usage[tokenKey].Add(usage)
	return nil
}

var (
	_Port    = ":8080"
	_DappID  = uint64(777)
	_Service = proto.Service_Indexer
	_Tokens  = []string{"abc", "cde"}
	_Now     = time.Date(2023, time.June, 26, 0, 0, 0, 0, time.Local)
)

func TestMiddlewareUseToken(t *testing.T) {
	s := miniredis.NewMiniRedis()
	s.Start()
	defer s.Close()
	redisClient := redisclient.NewClient(&redisclient.Options{Addr: s.Addr()})
	cache := quotacontrol.NewRedisCache(redisClient, time.Minute)

	limits := mockLimits{}
	tokens := mockTokens{}
	usage := mockUsage{
		usage:  map[string]*proto.AccessTokenUsage{},
		tokens: tokens,
	}
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

	go middlewareClient.Run(context.Background())

	l, err := net.Listen("tcp", _Port)
	require.NoError(t, err)

	go func() {
		serviceServer := quotacontrol.NewQuotaControl(cache, limits, tokens, usage)
		err = http.Serve(l, proto.NewQuotaControlServer(serviceServer))
		require.NoError(t, err)
	}()

	// populate store
	limits[_DappID] = []*proto.ServiceLimit{
		{Service: &_Service, ComputeRateLimit: 100, ComputeMonthlyQuota: 100},
	}
	tokens[_Tokens[0]] = &proto.AccessToken{Active: true, TokenKey: _Tokens[0], DappID: _DappID}
	tokens[_Tokens[1]] = &proto.AccessToken{Active: true, TokenKey: _Tokens[1], DappID: _DappID}
	tokens["mno"] = &proto.AccessToken{Active: true, TokenKey: "mno", DappID: _DappID + 1}
	tokens["xyz"] = &proto.AccessToken{Active: true, TokenKey: "xyz", DappID: _DappID + 1}

	ctx := quotacontrol.WithTime(context.Background(), _Now)
	for i := 0; i < 10; i++ {
		ctx := quotacontrol.WithComputeUnits(ctx, 10)
		ok, err := middlewareClient.UseToken(ctx, _Tokens[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
	}
	ok, err := middlewareClient.UseToken(ctx, _Tokens[0], "")
	assert.ErrorIs(t, err, proto.ErrLimitExceeded)
	assert.False(t, ok)

	// Add Quota and try again, it should fail because of rate limit
	// NOTE/TODO: the limits[_DappID][_Service] assumes limits[_DappID] is a map of service enum
	// but its not, its an array.
	// limits[_DappID][_Service].ComputeMonthlyQuota += 100
	limits[_DappID][0].ComputeMonthlyQuota += 100
	cache.DeleteToken(ctx, _Tokens[0])

	ok, err = middlewareClient.UseToken(ctx, _Tokens[0], "")
	assert.ErrorIs(t, err, proto.ErrLimitExceeded)
	assert.False(t, ok)
}
