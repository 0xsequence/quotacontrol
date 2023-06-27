package quotacontrol

import (
	"context"
	"encoding/json"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/redis/go-redis/v9"
)

type CacheStorage interface {
	GetToken(ctx context.Context, tokenKey string) (*proto.CachedToken, error)
	SetToken(ctx context.Context, token *proto.CachedToken) error
	DeleteToken(ctx context.Context, tokenKey string) error
	SetComputeUnits(ctx context.Context, redisKey string, amount int64) error
	SpendComputeUnits(ctx context.Context, redisKey string, amount, limit int64) (*RedisResponse, error)
}

type RedisResponse uint8

// Redis
const (
	PING_BUILDER RedisResponse = iota
	WAIT_AND_RETRY
	ALLOWED
	LIMITED
)

var _ CacheStorage = (*RedisCache)(nil)

func NewRedisCache(redisClient *redis.Client, ttl time.Duration) CacheStorage {
	return &RedisCache{
		client: redisClient,
		ttl:    ttl,
	}
}

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

func (s *RedisCache) GetToken(ctx context.Context, tokenKey string) (*proto.CachedToken, error) {
	raw, err := s.client.Get(ctx, tokenKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	var token proto.CachedToken
	if err := json.Unmarshal(raw, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *RedisCache) DeleteToken(ctx context.Context, tokenKey string) error {
	return s.client.Del(ctx, tokenKey).Err()
}

func (s *RedisCache) SetToken(ctx context.Context, token *proto.CachedToken) error {
	raw, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, token.AccessToken.TokenKey, raw, s.ttl).Err()
}

func (s *RedisCache) SetComputeUnits(ctx context.Context, redisKey string, amount int64) error {
	return s.client.Set(ctx, redisKey, amount, s.ttl).Err()
}

func (s *RedisCache) SpendComputeUnits(ctx context.Context, redisKey string, amount, limit int64) (*RedisResponse, error) {
	const SpecialValue = -1
	v, err := s.client.Get(ctx, redisKey).Int()
	if err != nil {
		if err != redis.Nil {
			return nil, err
		}
		ok, err := s.client.SetNX(ctx, redisKey, SpecialValue, time.Second*2).Result()
		if err != nil {
			return nil, err
		}
		if !ok {
			return proto.Ptr(WAIT_AND_RETRY), nil
		}
		return proto.Ptr(PING_BUILDER), nil
	}

	if v == SpecialValue {
		return proto.Ptr(WAIT_AND_RETRY), nil
	}
	value, err := s.client.IncrBy(ctx, redisKey, int64(amount)).Result()
	if err != nil {
		return nil, err
	}
	if value > int64(limit) {
		return proto.Ptr(LIMITED), nil
	}
	return proto.Ptr(ALLOWED), nil
}
