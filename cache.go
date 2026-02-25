package quotacontrol

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/0xsequence/quotacontrol/cache"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/redis/go-redis/v9"
)

const Version = "v2"

// KeyUsage is a cache key for usage amounts.
// It does not include version because usage is just a number, and it's safe to share across versions.
type KeyUsage struct {
	ProjectID uint64
	Service   *proto.Service
	Start     time.Time
	End       time.Time
}

func (k KeyUsage) String() string {
	if k.Service == nil {
		return fmt.Sprintf("usage:%d:%s-%s", k.ProjectID, k.Start.Format("2006-01-02"), k.End.Format("2006-01-02"))
	}
	return fmt.Sprintf("usage:%s:%d:%s-%s", k.Service.String(), k.ProjectID, k.Start.Format("2006-01-02"), k.End.Format("2006-01-02"))
}

// KeyAccessKey is a cache key for AccessQuota indexed by access key string.
// It includes version to avoid conflicts when the structure changes.
type KeyAccessKey struct {
	AccessKey string
}

func (k KeyAccessKey) String() string {
	return fmt.Sprintf("quota:%s:%s", Version, k.AccessKey)
}

// KeyProject is a cache key for AccessQuota indexed by project ID.
// It includes version to avoid conflicts when the structure changes.
type KeyProject struct {
	ProjectID uint64
}

func (k KeyProject) String() string {
	return fmt.Sprintf("project:%s:%d", Version, k.ProjectID)
}

// KeyPermission is a cache key for user permission indexed by project ID and user ID.
// It includes version to avoid conflicts when the structure changes.
type KeyPermission struct {
	ProjectID uint64
	UserID    string
}

func (k KeyPermission) String() string {
	return fmt.Sprintf("perm:%s:%d:%s", Version, k.ProjectID, k.UserID)
}

type UserPermission struct {
	UserPermission proto.UserPermission  `json:"userPerm"`
	ResourceAccess *proto.ResourceAccess `json:"resourceAccess"`
}

const (
	redisRLPrefix = "rl:"
)

func NewLimitCounter(svc proto.Service, cfg RedisConfig, logger *slog.Logger) httprate.LimitCounter {
	return httprateredis.NewCounter(&httprateredis.Config{
		Host:      cfg.Host,
		Port:      cfg.Port,
		MaxIdle:   cfg.MaxIdle,
		MaxActive: cfg.MaxActive,
		DBIndex:   cfg.DBIndex,
		PrefixKey: fmt.Sprintf("%s%s:", redisRLPrefix, svc),
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

// NewCache creates a Cache backed by Redis using the new generic cache package.
func NewCache(client *redis.Client, ttl time.Duration) Cache {
	backend := cache.NewBackend(client, ttl)
	return Cache{
		AccessKeys:      cache.NewRedisCache[KeyAccessKey, *proto.AccessQuota](backend),
		Projects:        cache.NewRedisCache[KeyProject, *proto.AccessQuota](backend),
		Permissions: cache.NewRedisCache[KeyPermission, UserPermission](backend),
		Usage:      cache.NewUsageCache[KeyAccessKey](backend),
	}
}
