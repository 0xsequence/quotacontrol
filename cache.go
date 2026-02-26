package quotacontrol

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/0xsequence/go-libs/xlog"
	"github.com/0xsequence/quotacontrol/cache"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	AccessKeys  cache.Simple[KeyAccessKey, *proto.AccessQuota]
	Projects    cache.Simple[KeyProject, *proto.AccessQuota]
	Permissions cache.Simple[KeyPermission, UserPermission]
	Usage       cache.Usage[KeyUsage]
}

// NewCache creates a Cache backed by Redis using the new generic cache package.
func NewCache(client *redis.Client, ttl time.Duration, lruSize int, lruExpiration time.Duration) Cache {
	backend := cache.NewBackend(client, ttl)
	c := Cache{
		AccessKeys:  cache.RedisCache[KeyAccessKey, *proto.AccessQuota]{Backend: backend},
		Projects:    cache.RedisCache[KeyProject, *proto.AccessQuota]{Backend: backend},
		Permissions: cache.RedisCache[KeyPermission, UserPermission]{Backend: backend},
		Usage:       cache.NewUsageCache[KeyUsage](backend),
	}
	if lruSize > 0 {
		c.AccessKeys = cache.NewMemory(c.AccessKeys, lruSize, lruExpiration)
		c.Projects = cache.NewMemory(c.Projects, lruSize, lruExpiration)
		c.Permissions = cache.NewMemory(c.Permissions, lruSize, lruExpiration)
	}
	return c
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
				logger.Error("redis counter error", xlog.Error(err))
			}
		},
		OnFallbackChange: func(fallback bool) {
			if logger != nil {
				logger.Warn("redis counter fallback", slog.Bool("fallback", fallback))
			}
		},
	})
}
