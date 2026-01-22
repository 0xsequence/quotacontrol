package quotacontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
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
	SetUsage(ctx context.Context, key string, amount int64) error
	ClearUsage(ctx context.Context, key string) (bool, error)
	PeekUsage(ctx context.Context, key string) (int64, error)
	SpendUsage(ctx context.Context, key string, amount, limit int64) (int64, error)
}

type PermissionCache interface {
	GetUserPermission(ctx context.Context, projectID uint64, userID string) (proto.UserPermission, *proto.ResourceAccess, error)
	SetUserPermission(ctx context.Context, projectID uint64, userID string, userPerm proto.UserPermission, resourceAccess *proto.ResourceAccess) error
	DeleteUserPermission(ctx context.Context, projectID uint64, userID string) error
}

type CacheResponse uint8

var (
	errCacheReady = errors.New("quotacontrol: cache ready for initialization")
	errCacheWait  = errors.New("quotacontrol: cache wait")
)

const (
	redisRLPrefix = "rl:"
)

var (
	_ QuotaCache = (*RedisCache)(nil)
	_ QuotaCache = (*LRU)(nil)
	_ UsageCache = (*RedisCache)(nil)
)

func NewLimitCounter(svc proto.Service, cfg RedisConfig, logger *slog.Logger) httprate.LimitCounter {
	if !cfg.Enabled {
		return nil
	}

	prefix := redisRLPrefix
	if s := svc.String(); s != "" {
		prefix = fmt.Sprintf("%s%s:", redisRLPrefix, s)
	}

	return httprateredis.NewCounter(&httprateredis.Config{
		Host:      cfg.Host,
		Port:      cfg.Port,
		MaxIdle:   cfg.MaxIdle,
		MaxActive: cfg.MaxActive,
		DBIndex:   cfg.DBIndex,
		PrefixKey: prefix,
		OnError: func(err error) {
			if logger != nil {
				logger.Error("redis counter error", slog.Any("error", err))
			}
		},
		OnFallbackChange: func(fallback bool) {
			if logger != nil {
				logger.Warn("redis counter fallback", slog.Bool("fallback", fallback))
			}
		},
	})
}

const (
	defaultExpRedis = time.Hour
	defaultExpLRU   = time.Minute
	cacheVersion    = "v2"
)

// usageKey returns the redis key for storing usage amount.
// It does not include version because usage is just a number, and it's safe to share across versions.
func usageKey(key string) string {
	return fmt.Sprintf("usage:%s", key)
}

// quotaKey returns the redis key for storing AccessQuota.
// It includes version to avoid conflicts when the structure changes.
func quotaKey(key string) string {
	return fmt.Sprintf("quota:%s:%s", cacheVersion, key)
}

// permissionKey returns the redis key for storing user permission for a project.
// It includes version to avoid conflicts when the structure changes.
func permissionKey(projectID uint64, userID string) string {
	return fmt.Sprintf("perm:%s:project:%d:user:%s", cacheVersion, projectID, userID)
}

func NewRedisCache(redisClient *redis.Client, ttl time.Duration) *RedisCache {
	if ttl <= 0 {
		ttl = defaultExpRedis
	}
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
	raw, err := s.client.Get(ctx, quotaKey(key)).Bytes()
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
	if err := s.client.Del(ctx, quotaKey(key)).Err(); err != nil {
		return fmt.Errorf("delete quota: %w", err)
	}
	return nil
}

func (s *RedisCache) setQuota(ctx context.Context, key string, quota *proto.AccessQuota) error {
	raw, err := json.Marshal(quota)
	if err != nil {
		return fmt.Errorf("marshal quota: %w", err)
	}
	if err := s.client.Set(ctx, quotaKey(key), raw, s.ttl).Err(); err != nil {
		return fmt.Errorf("set quota: %w", err)
	}
	return nil
}

func (s *RedisCache) SetUsage(ctx context.Context, key string, amount int64) error {
	if err := s.client.Set(ctx, usageKey(key), amount, s.ttl).Err(); err != nil {
		return fmt.Errorf("set usage: %w", err)
	}
	return nil
}

func (s *RedisCache) ClearUsage(ctx context.Context, key string) (bool, error) {
	count, err := s.client.Del(ctx, usageKey(key)).Result()
	if err != nil {
		return false, fmt.Errorf("clear usage: %w", err)
	}
	return count != 0, nil
}

func (s *RedisCache) PeekUsage(ctx context.Context, key string) (int64, error) {
	const SpecialValue = -1
	cacheKey := usageKey(key)

	v, err := s.client.Get(ctx, cacheKey).Int64()
	if err == nil {
		if v == SpecialValue {
			return 0, errCacheWait
		}
		return v, nil
	}
	if !errors.Is(err, redis.Nil) {
		return 0, fmt.Errorf("peek usage - get: %w", err)
	}
	ok, err := s.client.SetNX(ctx, cacheKey, SpecialValue, time.Second*2).Result()
	if err != nil {
		return 0, fmt.Errorf("peek usage - setnx: %w", err)
	}
	if !ok {
		return 0, errCacheWait
	}
	return 0, errCacheReady
}

func (s *RedisCache) SpendUsage(ctx context.Context, key string, amount, limit int64) (int64, error) {
	// NOTE: don't use usageKey yet, PeekUsage is doing that
	v, err := s.PeekUsage(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("spend usage - peek: %w", err)
	}
	if v >= limit {
		return v, proto.ErrQuotaExceeded
	}

	value, err := s.client.IncrBy(ctx, usageKey(key), amount).Result()
	if err != nil {
		return v, fmt.Errorf("spend usage - incrby: %w", err)
	}
	return value, nil
}

type cacheUserPermission struct {
	UserPermission proto.UserPermission  `json:"userPerm"`
	ResourceAccess *proto.ResourceAccess `json:"resourceAccess"`
}

func (s *RedisCache) GetUserPermission(ctx context.Context, projectID uint64, userID string) (proto.UserPermission, *proto.ResourceAccess, error) {
	raw, err := s.client.Get(ctx, permissionKey(projectID, userID)).Bytes()
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
	return s.client.Set(ctx, permissionKey(projectID, userID), raw, 10*time.Second).Err()
}

func (s *RedisCache) DeleteUserPermission(ctx context.Context, projectID uint64, userID string) error {
	return s.client.Del(ctx, permissionKey(projectID, userID)).Err()
}

type LRU struct {
	// mem is in-memory lru cache layer
	mem *expirable.LRU[string, *proto.AccessQuota]

	// backend is pluggable QuotaCache layer, which usually is redis
	backend QuotaCache
}

func NewLRU(cacheBackend QuotaCache, size int, ttl time.Duration) *LRU {
	if ttl <= 0 {
		ttl = defaultExpLRU
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
	if quota, ok := s.mem.Get(key); ok {
		return quota, nil
	}

	quota, err := s.backend.GetAccessQuota(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get quota: %w", err)
	}
	s.mem.Add(key, quota)
	return quota, nil
}

func (s *LRU) setQuota(ctx context.Context, key string, quota *proto.AccessQuota) error {
	s.mem.Add(key, quota)
	return s.backend.SetAccessQuota(ctx, quota)
}

func (s *LRU) deleteQuota(ctx context.Context, key string) error {
	s.mem.Remove(key)
	return s.backend.DeleteAccessQuota(ctx, key)
}

func getProjectKey(projectID uint64) string {
	return fmt.Sprintf("project:%d", projectID)
}
