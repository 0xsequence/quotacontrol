package quotacontrol

import (
	"net/http"
	"strings"
	"time"

	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/go-chi/jwtauth/v5"
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
	Enabled      bool         `toml:"enabled"`
	ErrorMessage string       `toml:"error_message"`
	DefaultRates DefaultRates `toml:"default_rates"`
}

type DefaultRates struct {
	Public  int `toml:"public"`
	User    int `toml:"user"`
	Service int `toml:"service"`
}

// DetectRate retuns key and limit for rate limiter when access key si not available.
func (d DefaultRates) DetectRate(r *http.Request) (key string, limit int) {
	_, claims, err := jwtauth.FromContext(r.Context())
	if err != nil || claims == nil {
		return "", d.Public
	}

	if svc, _ := claims["service"].(string); svc != "" {
		return "service:" + svc, d.Service
	}

	if account, _ := claims["account"].(string); account != "" {
		return "user:" + strings.ToLower(account), d.User
	}

	return "", d.Public
}
