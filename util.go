package quotacontrol

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/jxskiss/base62"
)

type UserPermission = proto.UserPermission

const (
	UserPermission_UNAUTHORIZED = proto.UserPermission_UNAUTHORIZED
	UserPermission_READ         = proto.UserPermission_READ
	UserPermission_READ_WRITE   = proto.UserPermission_READ_WRITE
)

func DefaultAccessKey(projectID uint64) string {
	buf := make([]byte, 24)
	binary.BigEndian.PutUint64(buf, projectID)
	rand.Read(buf[8:])
	return base62.EncodeToString(buf)
}

func GetProjectID(accessKey string) (uint64, error) {
	buf, err := base62.DecodeString(accessKey)
	if err != nil || len(buf) < 8 {
		return 0, proto.ErrAccessKeyNotFound
	}
	return binary.BigEndian.Uint64(buf[:8]), nil
}
