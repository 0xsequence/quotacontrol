package quotacontrol_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

type mockStore struct {
	limits map[uint64]*proto.AccessLimit
	tokens map[string]*proto.AccessToken
	usage  map[string]*proto.AccessTokenUsage
}

func (m *mockStore) RetrieveToken(ctx context.Context, tokenKey string) (*proto.CachedToken, error) {
	token, ok := m.tokens[tokenKey]
	if !ok {
		return nil, quotacontrol.ErrTokenNotFound
	}
	limit, ok := m.limits[token.DappID]
	if !ok {
		return nil, quotacontrol.ErrTokenNotFound
	}
	return &proto.CachedToken{
		AccessToken: token,
		AccessLimit: limit,
	}, nil
}

func (m *mockStore) GetAccountTotalUsage(ctx context.Context, dappID uint64, service proto.Service, min, max time.Time) (proto.AccessTokenUsage, error) {
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

func (m *mockStore) UpdateTokenUsage(ctx context.Context, tokenKey string, time time.Time, usage proto.AccessTokenUsage) error {
	if _, ok := m.usage[tokenKey]; !ok {
		m.usage[tokenKey] = &usage
		return nil
	}
	m.usage[tokenKey].Add(usage)
	return nil
}

var (
	_Address = "localhost:8080"
	_DappID  = uint64(777)
	_Service = proto.Service_Indexer
	_Tokens  = []string{"abc", "cde"}
	_Now     = time.Date(2023, time.June, 26, 0, 0, 0, 0, time.Local)
)

func TestMiddlewareUseToken(t *testing.T) {
	s := miniredis.NewMiniRedis()
	s.Start()
	defer s.Close()

	cache := quotacontrol.NewRedisCache(redis.NewClient(&redis.Options{Addr: s.Addr()}), -1)
	store := &mockStore{
		limits: map[uint64]*proto.AccessLimit{},
		tokens: map[string]*proto.AccessToken{},
		usage:  map[string]*proto.AccessTokenUsage{},
	}
	client := proto.NewQuotaControlClient(`http://`+_Address, http.DefaultClient)
	server := proto.NewQuotaControlServer(quotacontrol.NewQuotaControl(cache, store, store))
	m := quotacontrol.NewMiddleware(zerolog.New(zerolog.Nop()), &_Service, cache, client)
	ctx := quotacontrol.WithTime(context.Background(), _Now)

	go m.Run(ctx, time.Minute)
	go http.ListenAndServe(_Address, server)

	// populate store
	store.limits[_DappID] = &proto.AccessLimit{
		DappID: _DappID,
		Config: map[proto.Service]*proto.ServiceLimit{
			_Service: {ComputeRateLimit: 100, ComputeMonthlyQuota: 100},
		},
		Active: true,
	}
	store.tokens[_Tokens[0]] = &proto.AccessToken{Active: true, TokenKey: _Tokens[0], DappID: _DappID}
	store.tokens[_Tokens[1]] = &proto.AccessToken{Active: true, TokenKey: _Tokens[1], DappID: _DappID}
	store.tokens["mno"] = &proto.AccessToken{Active: true, TokenKey: "mno", DappID: _DappID + 1}
	store.tokens["xyz"] = &proto.AccessToken{Active: true, TokenKey: "xyz", DappID: _DappID + 1}

	time.Sleep(100 * time.Millisecond) // wait http server to start

	for i := 0; i < 10; i++ {
		ctx := quotacontrol.WithComputeUnits(ctx, 10)
		ok, err := m.UseToken(ctx, _Tokens[0], "")
		assert.NoError(t, err)
		assert.True(t, ok)
	}
	ok, err := m.UseToken(ctx, _Tokens[0], "")
	assert.ErrorIs(t, err, quotacontrol.ErrLimitExceeded)
	assert.False(t, ok)

}
