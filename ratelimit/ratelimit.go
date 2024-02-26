package ratelimit

import (
	"errors"

	"github.com/go-chi/httprate"
)

var (
	ErrAlreadyRegistered = errors.New("ratelimit: already registered")
)

type Type string

type Settings struct {
	counter       httprate.LimitCounter
	commonOptions []httprate.Option
	typeSettings  map[Type]rateLimit
}

type rateLimit struct {
	Limit   int64
	Options []httprate.Option
}

func NewSettings(counter httprate.LimitCounter, commonOptions ...httprate.Option) Settings {
	return Settings{
		counter:       counter,
		commonOptions: commonOptions,
		typeSettings:  make(map[Type]rateLimit),
	}
}

func (s *Settings) RegisterRateLimit(rateType Type, limit int64, options ...httprate.Option) error {
	if _, ok := s.typeSettings[rateType]; ok {
		return ErrAlreadyRegistered
	}

	s.typeSettings[rateType] = rateLimit{
		Limit:   limit,
		Options: options,
	}
	return nil
}

func (s *Settings) GetRateLimit(rateType Type) (int64, []httprate.Option, bool) {
	limiter, ok := s.typeSettings[rateType]
	if !ok {
		return 0, nil, false
	}

	options := make([]httprate.Option, 1, len(s.commonOptions)+len(limiter.Options))
	options[0] = httprate.WithLimitCounter(s.counter)
	options = append(options, s.commonOptions...)
	options = append(options, limiter.Options...)

	return limiter.Limit, options, true
}
