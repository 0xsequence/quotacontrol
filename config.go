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
	AuthToken     string            `toml:"auth_token"`
	UpdateFreq    Duration          `toml:"update_freq"`
	RateLimiter   RateLimiterConfig `toml:"rate_limiter"`
	Redis         redis.Config      `toml:"redis"`
	DefaultUsage  *int64            `toml:"default_usage"`
	LRUSize       int               `toml:"lru_size"`
	LRUExpiration Duration          `toml:"lru_expiration"`

	// DangerMode is used for debugging
	DangerMode bool `toml:"danger_mode"`
}

func (cfg Config) RateLimitCfg() *httprateredis.Config {
	return &httprateredis.Config{
		Host:      cfg.Redis.Host,
		Port:      cfg.Redis.Port,
		MaxIdle:   cfg.Redis.MaxIdle,
		MaxActive: cfg.Redis.MaxActive,
		DBIndex:   cfg.Redis.DBIndex,
	}
}

type RateLimiterConfig struct {
	Enabled    bool   `toml:"enabled"`
	PublicRPM  int    `toml:"public_rpm"`
	AccountRPM int    `toml:"account_rpm"`
	ErrorMsg   string `toml:"error_message"`
}
