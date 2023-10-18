package quotacontrol

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/redis/go-redis/v9"
)

type QuotaCache interface {
	GetAccessQuota(ctx context.Context, accessKey string) (*proto.AccessQuota, error)
	SetAccessQuota(ctx context.Context, quota *proto.AccessQuota) error
	DeleteAccessKey(ctx context.Context, accessKey string) error
}

type UsageCache interface {
	SetComputeUnits(ctx context.Context, redisKey string, amount int64) error
	PeekComputeUnits(ctx context.Context, redisKey string) (int64, error)
	SpendComputeUnits(ctx context.Context, redisKey string, amount, limit int64) (int64, error)
}

type CacheResponse uint8

var (
	ErrCachePing = errors.New("quotacontrol: cache ping")
	ErrCacheWait = errors.New("quotacontrol: cache wait")
)

var _ QuotaCache = (*RedisCache)(nil)
var _ UsageCache = (*RedisCache)(nil)

func NewRedisCache(redisClient *redis.Client, ttl time.Duration) *RedisCache {
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

type LRU struct {
	// mem is in-memory lru cache layer
	mem *lru.TwoQueueCache[string, *proto.AccessQuota]

	// backend is pluggable cache layer, which usually is redis
	backend QuotaCache
}

func NewLRU(cacheBackend QuotaCache, size int) (*LRU, error) {
	lruCache, err := lru.New2Q[string, *proto.AccessQuota](size)
	if err != nil {
		return nil, err
	}
	return &LRU{
		mem:     lruCache,
		backend: cacheBackend,
	}, nil
}

func (s *LRU) GetAccessQuota(ctx context.Context, accessKey string) (*proto.AccessQuota, error) {
	if aq, ok := s.mem.Get(accessKey); ok {
		return aq, nil
	}
	aq, err := s.backend.GetAccessQuota(ctx, accessKey)
	if err != nil {
		return nil, err
	}
	s.mem.Add(accessKey, aq)
	return aq, nil
}

func (s *LRU) SetAccessQuota(ctx context.Context, aq *proto.AccessQuota) error {
	s.mem.Add(aq.AccessKey.AccessKey, aq)
	return s.backend.SetAccessQuota(ctx, aq)
}

func (s *LRU) DeleteAccessKey(ctx context.Context, accessKey string) error {
	s.mem.Remove(accessKey)
	return s.backend.DeleteAccessKey(ctx, accessKey)
}
