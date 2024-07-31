package quotacontrol

import (
	"time"

	"github.com/0xsequence/quotacontrol/middleware"
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
	Enabled       bool                         `toml:"enabled"`
	URL           string                       `toml:"url"`
	AuthToken     string                       `toml:"auth_token"`
	UpdateFreq    Duration                     `toml:"update_freq"`
	RateLimiter   middleware.RateLimiterConfig `toml:"rate_limiter"`
	Redis         redis.Config                 `toml:"redis"`
	LRUSize       int                          `toml:"lru_size"`
	LRUExpiration Duration                     `toml:"lru_expiration"`

	// DangerMode is used for debugging
	DangerMode bool `toml:"danger_mode"`
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
