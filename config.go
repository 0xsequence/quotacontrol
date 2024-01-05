package quotacontrol

import (
	"time"

	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/goware/cachestore/redis"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
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

func (cfg *Config) SetAccessToken(alg jwa.SignatureAlgorithm, secret, service string) error {
	token := jwt.New()
	token.Set("service", service)
	token.Set("iat", time.Now().Unix())
	payload, err := jwt.Sign(token, jwt.WithKey(alg, []byte(secret)))
	if err != nil {
		return err
	}
	cfg.AuthToken = string(payload)
	return nil
}

type RateLimiterConfig struct {
	Enabled                  bool   `toml:"enabled"`
	PublicRequestsPerMinute  int    `toml:"public_requests_per_minute"`
	UserRequestsPerMinute    int    `toml:"user_requests_per_minute"`
	ServiceRequestsPerMinute int    `toml:"service_requests_per_minute"`
	ErrorMessage             string `toml:"error_message"`
}
