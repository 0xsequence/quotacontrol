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
	Enabled    bool         `toml:"enabled"`
	URL        string       `toml:"url"`
	Token      string       `toml:"token"`
	UpdateFreq Duration     `toml:"update_freq"`
	Redis      redis.Config `toml:"redis"`
}
