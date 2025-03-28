package quotacontrol

import (
	"cmp"
	"time"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
)

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
	Errors        ErrorConfig     `toml:"errors"`

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

type ErrorConfig struct {
	MessageQuota      string `toml:"quota_message"`
	MessageRate       string `toml:"ratelimit_message"`
	MessageRatePublic string `toml:"public_message"`
}

// Apply applies the error configuration globally.
func (e ErrorConfig) Apply() {
	proto.ErrQuotaExceeded.Message = cmp.Or(e.MessageQuota, proto.ErrQuotaExceeded.Message)
	proto.ErrQuotaRateLimit.Message = cmp.Or(e.MessageRate, proto.ErrQuotaRateLimit.Message)
	proto.ErrRateLimited.Message = cmp.Or(e.MessageRatePublic, proto.ErrRateLimited.Message)
}
