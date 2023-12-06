package quotacontrol

import (
	"time"

	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/goware/cachestore/redis"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type Config struct {
	Enabled       bool              `toml:"enabled"`
	URL           string            `toml:"url"`
	AccessKey     string            `toml:"access_key"`
	UpdateFreq    Duration          `toml:"update_freq"`
	RateLimiter   RateLimiterConfig `toml:"rate_limiter"`
	Redis         redis.Config      `toml:"redis"`
	LRUSize       int               `toml:"lru_size"`
	LRUExpiration Duration          `toml:"lru_expiration"`
}

func (cfg Config) RedisRateLimitConfig() *httprateredis.Config {
	return &httprateredis.Config{
		Host:      cfg.Redis.Host,
		Port:      cfg.Redis.Port,
		MaxIdle:   cfg.Redis.MaxIdle,
		MaxActive: cfg.Redis.MaxActive,
		DBIndex:   cfg.Redis.DBIndex,
	}
}

type RateLimiterConfig struct {
	Enabled                  bool   `toml:"enabled"`
	PublicRequestsPerMinute  int    `toml:"public_requests_per_minute"`
	UserRequestsPerMinute    int    `toml:"user_requests_per_minute"`
	ServiceRequestsPerMinute int    `toml:"service_requests_per_minute"`
	ErrorMessage             string `toml:"error_message"`
}
