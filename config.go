package quotacontrol

import (
	"time"

	"github.com/0xsequence/quotacontrol/middleware"
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
	Enabled       bool                `toml:"enabled"`
	URL           string              `toml:"url"`
	AuthToken     string              `toml:"auth_token"`
	UpdateFreq    Duration            `toml:"update_freq"`
	RateLimiter   middleware.RLConfig `toml:"rate_limiter"`
	Redis         redis.Config        `toml:"redis"`
	DefaultUsage  *int64              `toml:"default_usage"`
	LRUSize       int                 `toml:"lru_size"`
	LRUExpiration Duration            `toml:"lru_expiration"`

	// DangerMode is used for debugging
	DangerMode bool `toml:"danger_mode"`
}
