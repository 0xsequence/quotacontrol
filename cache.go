package quotacontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/redis/go-redis/v9"
)

type QuotaCache interface {
	GetProjectQuota(ctx context.Context, projectID uint64) (*proto.AccessQuota, error)
	SetProjectQuota(ctx context.Context, quota *proto.AccessQuota) error
	DeleteProjectQuota(ctx context.Context, projectID uint64) error
	GetAccessQuota(ctx context.Context, accessKey string) (*proto.AccessQuota, error)
	SetAccessQuota(ctx context.Context, quota *proto.AccessQuota) error
	DeleteAccessQuota(ctx context.Context, accessKey string) error
}

type UsageCache interface {
	SetComputeUnits(ctx context.Context, redisKey string, amount int64) error
	ClearComputeUnits(ctx context.Context, redisKey string) (bool, error)
	PeekComputeUnits(ctx context.Context, redisKey string) (int64, error)
	SpendComputeUnits(ctx context.Context, redisKey string, amount, limit int64) (int64, error)
}

type PermissionCache interface {
	GetUserPermission(ctx context.Context, projectID uint64, userID string) (proto.UserPermission, *proto.ResourceAccess, error)
	SetUserPermission(ctx context.Context, projectID uint64, userID string, userPerm proto.UserPermission, resourceAccess *proto.ResourceAccess) error
	DeleteUserPermission(ctx context.Context, projectID uint64, userID string) error
}

type CacheResponse uint8

var (
	ErrCachePing = errors.New("quotacontrol: cache ping")
	ErrCacheWait = errors.New("quotacontrol: cache wait")
)

const redisKeyPrefix = "quota:"

var _ QuotaCache = (*RedisCache)(nil)
var _ QuotaCache = (*LRU)(nil)
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
	return s.getQuota(ctx, accessKey)
}

func (s *RedisCache) DeleteAccessQuota(ctx context.Context, accessKey string) error {
	return s.deleteQuota(ctx, accessKey)
}

func (s *RedisCache) SetAccessQuota(ctx context.Context, quota *proto.AccessQuota) error {
	return s.setQuota(ctx, quota.AccessKey.AccessKey, quota)
}

func (s *RedisCache) GetProjectQuota(ctx context.Context, projectID uint64) (*proto.AccessQuota, error) {
	return s.getQuota(ctx, getProjectKey(projectID))
}

func (s *RedisCache) DeleteProjectQuota(ctx context.Context, projectID uint64) error {
	return s.deleteQuota(ctx, getProjectKey(projectID))
}

func (s *RedisCache) SetProjectQuota(ctx context.Context, quota *proto.AccessQuota) error {
	return s.setQuota(ctx, getProjectKey(quota.AccessKey.ProjectID), quota)
}

func (s *RedisCache) getQuota(ctx context.Context, key string) (*proto.AccessQuota, error) {
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, key)
	raw, err := s.client.Get(ctx, cacheKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, proto.ErrAccessKeyNotFound
		}
		return nil, fmt.Errorf("get quota: %w", err)
	}
	var quota proto.AccessQuota
	if err := json.Unmarshal(raw, &quota); err != nil {
		return nil, fmt.Errorf("unmarshal quota: %w", err)
	}
	return &quota, nil
}

func (s *RedisCache) deleteQuota(ctx context.Context, key string) error {
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, key)
	return s.client.Del(ctx, cacheKey).Err()
}

func (s *RedisCache) setQuota(ctx context.Context, key string, quota *proto.AccessQuota) error {
	raw, err := json.Marshal(quota)
	if err != nil {
		return fmt.Errorf("marshal quota: %w", err)
	}
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, key)
	if err := s.client.Set(ctx, cacheKey, raw, s.ttl).Err(); err != nil {
		return fmt.Errorf("set quota: %w", err)
	}
	return nil
}

func (s *RedisCache) SetComputeUnits(ctx context.Context, redisKey string, amount int64) error {
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, redisKey)
	return s.client.Set(ctx, cacheKey, amount, s.ttl).Err()
}

func (s *RedisCache) ClearComputeUnits(ctx context.Context, redisKey string) (bool, error) {
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, redisKey)
	count, err := s.client.Del(ctx, cacheKey).Result()
	if err != nil {
		return false, err
	}
	return count != 0, nil
}

func (s *RedisCache) PeekComputeUnits(ctx context.Context, redisKey string) (int64, error) {
	const SpecialValue = -1
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, redisKey)
	v, err := s.client.Get(ctx, cacheKey).Int64()
	if err == nil {
		if v == SpecialValue {
			return 0, ErrCacheWait
		}
		return v, nil
	}
	if !errors.Is(err, redis.Nil) {
		return 0, err
	}
	ok, err := s.client.SetNX(ctx, cacheKey, SpecialValue, time.Second*2).Result()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrCacheWait
	}
	return 0, ErrCachePing
}

func (s *RedisCache) SpendComputeUnits(ctx context.Context, redisKey string, amount, limit int64) (int64, error) {
	// NOTE: skip redisKeyPrefix as it's already in PeekComputeUnits
	v, err := s.PeekComputeUnits(ctx, redisKey)
	if err != nil {
		return 0, err
	}
	if v >= limit {
		return v, proto.ErrLimitExceeded
	}
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, redisKey)
	value, err := s.client.IncrBy(ctx, cacheKey, amount).Result()
	if err != nil {
		return 0, err
	}
	return value, nil
}

type cacheUserPermission struct {
	UserPermission proto.UserPermission  `json:"userPerm"`
	ResourceAccess *proto.ResourceAccess `json:"resourceAccess"`
}

func (s *RedisCache) GetUserPermission(ctx context.Context, projectID uint64, userID string) (proto.UserPermission, *proto.ResourceAccess, error) {
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, getUserPermKey(projectID, userID))
	raw, err := s.client.Get(ctx, cacheKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return proto.UserPermission_UNAUTHORIZED, nil, nil // not found, without error
		}
		return proto.UserPermission_UNAUTHORIZED, nil, err
	}
	var v cacheUserPermission
	if err := json.Unmarshal(raw, &v); err != nil {
		return proto.UserPermission_UNAUTHORIZED, nil, err
	}
	return v.UserPermission, v.ResourceAccess, nil
}

func (s *RedisCache) SetUserPermission(ctx context.Context, projectID uint64, userID string, userPerm proto.UserPermission, resourceAccess *proto.ResourceAccess) error {
	v := cacheUserPermission{
		UserPermission: userPerm,
		ResourceAccess: resourceAccess,
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// cache userPermissions for 10 seconds
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, getUserPermKey(projectID, userID))
	return s.client.Set(ctx, cacheKey, raw, 10*time.Second).Err()
}

func (s *RedisCache) DeleteUserPermission(ctx context.Context, projectID uint64, userID string) error {
	cacheKey := fmt.Sprintf("%s%s", redisKeyPrefix, getUserPermKey(projectID, userID))
	return s.client.Del(ctx, cacheKey).Err()
}

type LRU struct {
	// mem is in-memory lru cache layer
	mem *expirable.LRU[string, *proto.AccessQuota]

	// backend is pluggable QuotaCache layer, which usually is redis
	backend QuotaCache
}

func NewLRU(cacheBackend QuotaCache, size int, ttl time.Duration) *LRU {
	if ttl == 0 {
		ttl = time.Minute * 5
	}
	lruCache := expirable.NewLRU[string, *proto.AccessQuota](size, nil, ttl)
	return &LRU{
		mem:     lruCache,
		backend: cacheBackend,
	}
}

func (s *LRU) GetAccessQuota(ctx context.Context, accessKey string) (*proto.AccessQuota, error) {
	return s.getQuota(ctx, accessKey)
}

func (s *LRU) SetAccessQuota(ctx context.Context, quota *proto.AccessQuota) error {
	return s.setQuota(ctx, quota.AccessKey.AccessKey, quota)
}

func (s *LRU) DeleteAccessQuota(ctx context.Context, accessKey string) error {
	return s.deleteQuota(ctx, accessKey)
}

func (s *LRU) GetProjectQuota(ctx context.Context, projectID uint64) (*proto.AccessQuota, error) {
	return s.getQuota(ctx, getProjectKey(projectID))
}

func (s *LRU) SetProjectQuota(ctx context.Context, quota *proto.AccessQuota) error {
	return s.setQuota(ctx, getProjectKey(quota.AccessKey.ProjectID), quota)
}

func (s *LRU) DeleteProjectQuota(ctx context.Context, projectID uint64) error {
	return s.deleteQuota(ctx, getProjectKey(projectID))
}

func (s *LRU) getQuota(ctx context.Context, key string) (*proto.AccessQuota, error) {
	if aq, ok := s.mem.Get(key); ok {
		return aq, nil
	}
	aq, err := s.backend.GetAccessQuota(ctx, key)
	if err != nil {
		return nil, err
	}
	s.mem.Add(key, aq)
	return aq, nil
}

func (s *LRU) setQuota(ctx context.Context, key string, aq *proto.AccessQuota) error {
	s.mem.Add(key, aq)
	return s.backend.SetAccessQuota(ctx, aq)
}

func (s *LRU) deleteQuota(ctx context.Context, key string) error {
	s.mem.Remove(key)
	return s.backend.DeleteAccessQuota(ctx, key)
}

func getProjectKey(projectID uint64) string {
	return fmt.Sprintf("project:%d", projectID)
}

func getUserPermKey(projectID uint64, userID string) string {
	return fmt.Sprintf("project:%d:userPerm:%s", projectID, userID)
}
