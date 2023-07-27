package quotacontrol_test

import (
	"context"
	"fmt"
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

type mockLimits map[uint64]map[proto.Service]*proto.ServiceLimit

func (m mockLimits) GetAccessLimit(ctx context.Context, projectID uint64) ([]*proto.ServiceLimit, error) {
	limit, ok := m[projectID]
	if !ok {
		return nil, proto.ErrTokenNotFound
	}
	var result []*proto.ServiceLimit
	for _, v := range limit {
		result = append(result, v)
	}
	return result, nil
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

func (m mockUsage) GetAccountTotalUsage(ctx context.Context, projectID uint64, service proto.Service, min, max time.Time) (proto.AccessTokenUsage, error) {
	usage := proto.AccessTokenUsage{}
	for _, v := range m.tokens {
		if v.ProjectID == projectID {
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
	fmt.Println("update usage", tokenKey, time, usage)
	m.usage[tokenKey].Add(usage)
	return nil
}

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
	limits[_ProjectID] = map[proto.Service]*proto.ServiceLimit{
		_Service: {Service: &_Service, ComputeRateLimit: 100, ComputeMonthlyQuota: 50, ComputeMonthlyHardQuota: 100},
	}
	tokens[_Tokens[0]] = &proto.AccessToken{Active: true, TokenKey: _Tokens[0], ProjectID: _ProjectID}
	tokens[_Tokens[1]] = &proto.AccessToken{Active: true, TokenKey: _Tokens[1], ProjectID: _ProjectID}
	tokens["mno"] = &proto.AccessToken{Active: true, TokenKey: "mno", ProjectID: _ProjectID + 1}
	tokens["xyz"] = &proto.AccessToken{Active: true, TokenKey: "xyz", ProjectID: _ProjectID + 1}

	ctx := quotacontrol.WithComputeUnits(quotacontrol.WithTime(context.Background(), _Now), 10)
	for i := 0; i < 10; i++ {
		ok, err := middlewareClient.UseToken(ctx, _Tokens[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
	}
	ok, err := middlewareClient.UseToken(ctx, _Tokens[0], "")
	assert.ErrorIs(t, err, proto.ErrLimitExceeded)
	assert.False(t, ok)

	// Add Quota and try again, it should fail because of rate limit
	limits[_ProjectID][_Service].ComputeMonthlyHardQuota += 100
	cache.DeleteToken(ctx, _Tokens[0])

	ok, err = middlewareClient.UseToken(ctx, _Tokens[0], "")
	assert.ErrorIs(t, err, proto.ErrLimitExceeded)
	assert.False(t, ok)

	ok, err = middlewareClient.UseToken(quotacontrol.SkipRateLimit(ctx), _Tokens[0], "")
	assert.NoError(t, err)
	assert.True(t, ok)

	middlewareClient.Stop(ctx)
	assert.Equal(t, &proto.AccessTokenUsage{ValidCompute: 50, OverCompute: 60, LimitedCompute: 0}, usage.usage[_Tokens[0]])
}
