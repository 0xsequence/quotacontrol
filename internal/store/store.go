package store

import (
	"context"
	"time"

	"github.com/0xsequence/quotacontrol/encoding"
	"github.com/0xsequence/quotacontrol/proto"
)

type Cycle struct{}

func (s Cycle) GetAccessCycle(_ context.Context, _ uint64, now time.Time) (*proto.Cycle, error) {
	return &proto.Cycle{
		Start: now.AddDate(0, 0, 1-now.Day()),
		End:   now.AddDate(0, 1, 1-now.Day()),
	}, nil
}

type Prefix struct {
	Env string
}

func (s Prefix) GetPrefix(_ context.Context, _ uint64) (string, string, error) {
	return encoding.DefaultPrefix, s.Env, nil
}
