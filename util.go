package quotacontrol

import (
	"context"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

type UserPermission = proto.UserPermission

const (
	UserPermission_UNAUTHORIZED = proto.UserPermission_UNAUTHORIZED
	UserPermission_READ         = proto.UserPermission_READ
	UserPermission_READ_WRITE   = proto.UserPermission_READ_WRITE
	UserPermission_ADMIN        = proto.UserPermission_ADMIN
)

type (
	ResourceAccess = proto.ResourceAccess
	Subscription   = proto.Subscription
	Minter         = proto.Minter
)

type DefaultCycleStore struct{}

func (s DefaultCycleStore) GetAccessCycle(ctx context.Context, projectID uint64, now time.Time) (*proto.Cycle, error) {
	return &proto.Cycle{
		Start: now.AddDate(0, 0, 1-now.Day()),
		End:   now.AddDate(0, 1, 1-now.Day()),
	}, nil
}
