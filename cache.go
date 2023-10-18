package quotacontrol

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/redis/go-redis/v9"
)

type Cache interface {
	GetAccessQuota(ctx context.Context, accessKey string) (*proto.AccessQuota, error)
	SetAccessQuota(ctx context.Context, quota *proto.AccessQuota) error
	DeleteAccessKey(ctx context.Context, accessKey string) error
	SetComputeUnits(ctx context.Context, redisKey string, amount int64) error
	PeekComputeUnits(ctx context.Context, redisKey string) (int64, error)
	SpendComputeUnits(ctx context.Context, redisKey string, amount, limit int64) (int64, error)
}

type CacheResponse uint8

var (
	ErrCachePing = errors.New("quotacontrol: cache ping")
	ErrCacheWait = errors.New("quotacontrol: cache wait")
)

var _ Cache = (*RedisCache)(nil)

func NewRedisCache(redisClient *redis.Client, ttl time.Duration) Cache {
	return &RedisCache{
		client: redisClient,
		ttl:    ttl,
	}
}

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

func (s *RedisCache) GetAccessQuota(ctx context.Context, accessKey string) (*proto.AccessQuota, error) {
	raw, err := s.client.Get(ctx, accessKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, proto.ErrAccessKeyNotFound
		}
		return nil, err
	}
	var quota proto.AccessQuota
	if err := json.Unmarshal(raw, &quota); err != nil {
		return nil, err
	}
	return &quota, nil
}

func (s *RedisCache) DeleteAccessKey(ctx context.Context, accessKey string) error {
	return s.client.Del(ctx, accessKey).Err()
}

func (s *RedisCache) SetAccessQuota(ctx context.Context, quota *proto.AccessQuota) error {
	raw, err := json.Marshal(quota)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, quota.AccessKey.AccessKey, raw, s.ttl).Err()
}

func (s *RedisCache) SetComputeUnits(ctx context.Context, redisKey string, amount int64) error {
	return s.client.Set(ctx, redisKey, amount, s.ttl).Err()
}

func (s *RedisCache) PeekComputeUnits(ctx context.Context, redisKey string) (int64, error) {
	const SpecialValue = -1
	v, err := s.client.Get(ctx, redisKey).Int64()
	if err == nil {
		if v == SpecialValue {
			return 0, ErrCacheWait
		}
		return v, nil
	}
	if !errors.Is(err, redis.Nil) {
		return 0, err
	}
	ok, err := s.client.SetNX(ctx, redisKey, SpecialValue, time.Second*2).Result()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrCacheWait
	}
	return 0, ErrCachePing
}

func (s *RedisCache) SpendComputeUnits(ctx context.Context, redisKey string, amount, limit int64) (int64, error) {
	v, err := s.PeekComputeUnits(ctx, redisKey)
	if err != nil {
		return 0, err
	}
	if v > limit {
		return v, proto.ErrLimitExceeded
	}
	value, err := s.client.IncrBy(ctx, redisKey, amount).Result()
	if err != nil {
		return 0, err
	}
	return value, nil
}
