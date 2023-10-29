package quotacontrol

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/jxskiss/base62"
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
