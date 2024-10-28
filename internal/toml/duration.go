package toml

import "time"

func NewDuration(d time.Duration) Duration {
	return Duration{Duration: d}
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}
