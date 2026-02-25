package quotacontrol

import (
	"fmt"
	"log/slog"
	"time"

	cachelib "github.com/0xsequence/quotacontrol/cache"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/redis/go-redis/v9"
)

type UserPermission struct {
	UserPermission proto.UserPermission  `json:"userPerm"`
	ResourceAccess *proto.ResourceAccess `json:"resourceAccess"`
}

const (
	redisRLPrefix = "rl:"
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

// NewCache creates a Cache backed by Redis using the new generic cache package.
func NewCache(client *redis.Client, ttl time.Duration) Cache {
	backend := cachelib.NewBackend(client, ttl)
	return Cache{
		AccessKeys:      cachelib.NewRedisCache[cachelib.KeyAccessKey, *proto.AccessQuota](backend),
		Projects:        cachelib.NewRedisCache[cachelib.KeyProject, *proto.AccessQuota](backend),
		UsageCache:      cachelib.NewUsageCache[cachelib.KeyAccessKey](backend),
		PermissionCache: cachelib.NewRedisCache[cachelib.KeyPermission, UserPermission](backend),
	}
}
