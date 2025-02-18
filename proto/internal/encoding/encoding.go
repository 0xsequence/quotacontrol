package encoding

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/goware/base64"
	"github.com/jxskiss/base62"
)

var (
	ErrInvalidKeyLength = fmt.Errorf("invalid access key length")
)

type Encoding interface {
	Version() int
	Encode(projectID uint64, ecosystemID uint64) string
	Decode(accessKey string) (projectID uint64, ecosystemID uint64, err error)
}

const (
	sizeV0 = 24
	sizeV1 = 26
	sizeV2 = 32
)

// V0 is the v0 encoding format for project access keys
type V0 struct{}

func (V0) Version() int { return 0 }

func (V0) Encode(projectID uint64, ecosystemID uint64) string {
	buf := make([]byte, sizeV0)
	binary.BigEndian.PutUint64(buf, projectID)
	rand.Read(buf[8:])
	return base62.EncodeToString(buf)
}

func (V0) Decode(accessKey string) (projectID uint64, ecosystemID uint64, err error) {
	buf, err := base62.DecodeString(accessKey)
	if err != nil {
		return 0, 0, fmt.Errorf("base62 decode: %w", err)
	}
	if len(buf) != sizeV0 {
		return 0, 0, ErrInvalidKeyLength
	}
	return binary.BigEndian.Uint64(buf[:8]), 0, nil
}

type V1 struct{}

func (V1) Version() int { return 1 }

func (V1) Encode(projectID uint64, ecosystemID uint64) string {
	buf := make([]byte, sizeV1)
	buf[0] = byte(1)
	binary.BigEndian.PutUint64(buf[1:], projectID)
	rand.Read(buf[9:])
	return base64.Base64UrlEncode(buf)
}

func (V1) Decode(accessKey string) (projectID uint64, ecosystemID uint64, err error) {
	buf, err := base64.Base64UrlDecode(accessKey)
	if err != nil {
		return 0, 0, fmt.Errorf("base64 decode: %w", err)
	}
	if len(buf) != sizeV1 {
		return 0, 0, ErrInvalidKeyLength
	}
	return binary.BigEndian.Uint64(buf[1:9]), 0, nil
}

type V2 struct{}

func (V2) Version() int { return 2 }

func (V2) Encode(projectID uint64, ecosystemID uint64) string {
	buf := make([]byte, sizeV2)
	buf[0] = byte(2)
	binary.BigEndian.PutUint64(buf[1:], projectID)
	binary.BigEndian.PutUint64(buf[9:], ecosystemID)
	rand.Read(buf[17:])
	return base64.Base64UrlEncode(buf)
}

func (V2) Decode(accessKey string) (projectID uint64, ecosystemID uint64, err error) {
	buf, err := base64.Base64UrlDecode(accessKey)
	if err != nil {
		return 0, 0, fmt.Errorf("base64 decode: %w", err)
	}
	if len(buf) != sizeV2 {
		return 0, 0, ErrInvalidKeyLength
	}
	return binary.BigEndian.Uint64(buf[1:9]), binary.BigEndian.Uint64(buf[9:17]), nil
}
