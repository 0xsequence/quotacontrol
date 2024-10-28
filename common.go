package quotacontrol

import (
	"github.com/0xsequence/quotacontrol/proto"
)

const (
	UserPermission_UNAUTHORIZED = proto.UserPermission_UNAUTHORIZED
	UserPermission_READ         = proto.UserPermission_READ
	UserPermission_READ_WRITE   = proto.UserPermission_READ_WRITE
	UserPermission_ADMIN        = proto.UserPermission_ADMIN
)

type (
	ResourceAccess = proto.ResourceAccess
	UserPermission = proto.UserPermission

	Subscription = proto.Subscription
	Minter       = proto.Minter
)
