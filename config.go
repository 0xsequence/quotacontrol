package quotacontrol

import (
	"time"

	"github.com/0xsequence/quotacontrol/middleware"
)

var Version = "0.23.0"

type Config struct {
	Enabled       bool            `toml:"enabled"`
	URL           string          `toml:"url"`
	AuthToken     string          `toml:"auth_token"`
	UpdateFreq    time.Duration   `toml:"update_freq"`
	RateLimiter   RateLimitConfig `toml:"rate_limiter"`
	Redis         RedisConfig     `toml:"redis"`
	DefaultUsage  *int64          `toml:"default_usage"`
	LRUSize       int             `toml:"lru_size"`
	LRUExpiration time.Duration   `toml:"lru_expiration"`

	// DangerMode is used for debugging
	DangerMode bool `toml:"danger_mode"`
}

type RateLimitConfig = middleware.RateLimitConfig

type RedisConfig struct {
	Enabled   bool          `toml:"enabled"`
	Host      string        `toml:"host"`
	Port      uint16        `toml:"port"`
	DBIndex   int           `toml:"db_index"`   // default 0
	MaxIdle   int           `toml:"max_idle"`   // default 4
	MaxActive int           `toml:"max_active"` // default 8
	KeyTTL    time.Duration `toml:"key_ttl"`    // default 1 day
}
