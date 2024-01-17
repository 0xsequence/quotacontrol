package quotacontrol

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/base64"
	"github.com/jxskiss/base62"
)

func GetProjectID(accessKey string) (uint64, error) {
	// try v1 format first
	id, err1 := DecodeProjectID(accessKey)
	if err1 == nil {
		return id, nil
	}

	// try v0/legacy format second
	id, err2 := LegacyDecodeProjectID(accessKey)
	if err2 == nil {
		return id, nil
	}

	return 0, err1
}

// GenerateAccessKey uses v1 encoding format to generate a
// new access key with the encoded projectID.
func GenerateAccessKey(projectID uint64) string {
	buf := make([]byte, 26)
	buf[0] = 1 // version 1
	binary.BigEndian.PutUint64(buf[1:], projectID)
	rand.Read(buf[9:])
	return base64.Base64UrlEncode(buf)
}

// DecodeProjectID uses v1 encoding format to decode project id
// from base64-url-encoded access key
func DecodeProjectID(accessKey string) (uint64, error) {
	buf, err := base64.Base64UrlDecode(accessKey)
	if err != nil {
		return 0, proto.ErrorWithCause(proto.ErrAccessKeyNotFound, err)
	}
	if len(buf) != 26 {
		err := fmt.Errorf("invalid v1 project access key")
		return 0, proto.ErrorWithCause(proto.ErrAccessKeyNotFound, err)
	}
	return binary.BigEndian.Uint64(buf[1:9]), nil
}

// LegacyGenerateAccessKey generates base62 / v0 legacy access key format
func LegacyGenerateAccessKey(projectID uint64) string {
	buf := make([]byte, 24)
	binary.BigEndian.PutUint64(buf, projectID)
	rand.Read(buf[8:])
	return base62.EncodeToString(buf)
}

// LegacyDecodeProjectID decodes base 62 / v0 legacy access keys to
// return the project id.
func LegacyDecodeProjectID(accessKey string) (uint64, error) {
	buf, err := base62.DecodeString(accessKey)
	if err != nil {
		return 0, proto.ErrorWithCause(proto.ErrAccessKeyNotFound, err)
	}
	if len(buf) != 24 {
		err := fmt.Errorf("invalid v0/legacy project access key")
		return 0, proto.ErrorWithCause(proto.ErrAccessKeyNotFound, err)
	}
	return binary.BigEndian.Uint64(buf[:8]), nil
}
