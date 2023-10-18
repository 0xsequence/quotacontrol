package quotacontrol

import (
	"time"

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
	Enabled     bool              `toml:"enabled"`
	URL         string            `toml:"url"`
	AccessKey   string            `toml:"access_key"`
	UpdateFreq  Duration          `toml:"update_freq"`
	RateLimiter RateLimiterConfig `toml:"rate_limiter"`
	Redis       redis.Config      `toml:"redis"`
}

type RateLimiterConfig struct {
	Enabled                   bool          `toml:"enabled"`
	PublicRequestsPerMinute   int           `toml:"public_requests_per_minute"`
	AccessKeyUsageFlushPeriod time.Duration `toml:"access_key_usage_flush_period"`
}
