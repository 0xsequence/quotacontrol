package encoding

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/goware/base64"
	"github.com/jxskiss/base62"
)

var (
	ErrInvalidKeyLength = fmt.Errorf("invalid access key length")
	ErrVersionMismatch  = fmt.Errorf("version mismatch")
)

type Encoding interface {
	Version() byte
	Encode(projectID uint64, ecosystemID uint64) string
	Decode(accessKey string) (projectID uint64, ecosystemID uint64, err error)
}

const (
	sizeV0 = 24
	sizeV1 = 26
	sizeV2 = 32
	sizeV3 = 32
)

// V0 is the v0 encoding format for project access keys
type V0 struct{}

func (V0) Version() byte { return 0 }

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

func (V1) Version() byte { return 1 }

func (v V1) Encode(projectID uint64, ecosystemID uint64) string {
	buf := make([]byte, sizeV1)
	buf[0] = v.Version()
	binary.BigEndian.PutUint64(buf[1:], projectID)
	rand.Read(buf[9:])
	return base64.Base64UrlEncode(buf)
}

func (v V1) Decode(accessKey string) (projectID uint64, ecosystemID uint64, err error) {
	buf, err := base64.Base64UrlDecode(accessKey)
	if err != nil {
		return 0, 0, fmt.Errorf("base64 decode: %w", err)
	}
	if len(buf) != sizeV1 {
		return 0, 0, ErrInvalidKeyLength
	}
	if buf[0] != v.Version() {
		return 0, 0, ErrVersionMismatch
	}
	return binary.BigEndian.Uint64(buf[1:9]), 0, nil
}

type V2 struct{}

func (V2) Version() byte { return 2 }

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

type V3 struct{}

func (V3) Version() byte { return 3 }

func (v V3) Encode(projectID uint64, ecosystemID uint64) string {
	buf := make([]byte, sizeV3)
	buf[0] = v.Version()

	encodedProjectID := encodeUint64(projectID)
	encodedEcosystemID := encodeUint64(ecosystemID)
	buf[1] = byte(len(encodedProjectID)) + (byte(len(encodedEcosystemID) << 4))
	copy(buf[2:], encodedProjectID)
	copy(buf[2+len(encodedProjectID):], encodedEcosystemID)

	rand.Read(buf[2+len(encodedProjectID)+len(encodedEcosystemID):])

	return base64.Base64UrlEncode(buf)
}

func (v V3) Decode(accessKey string) (projectID uint64, ecosystemID uint64, err error) {
	buf, err := base64.Base64UrlDecode(accessKey)
	if err != nil {
		return 0, 0, fmt.Errorf("base64 decode: %w", err)
	}
	if len(buf) != sizeV3 {
		return 0, 0, ErrInvalidKeyLength
	}
	if buf[0] != v.Version() {
		return 0, 0, fmt.Errorf("version mismatch")
	}

	projectLength := buf[1] & 0x0f
	ecosystemLength := buf[1] >> 4

	if projectID, err = decodeUint64(buf[2 : 2+projectLength]); err != nil {
		return 0, 0, fmt.Errorf("decode projectID: %w", err)
	}

	if ecosystemID, err = decodeUint64(buf[2+projectLength : 2+projectLength+ecosystemLength]); err != nil {
		return 0, 0, fmt.Errorf("decode ecosystemID: %w", err)
	}

	return projectID, ecosystemID, nil
}

func encodeUint64(n uint64) []byte {
	switch {
	case n <= math.MaxUint8:
		return []byte{byte(n)}
	case n <= math.MaxUint16:
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(n))
		return buf
	case n <= math.MaxUint32:
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(n))
		return buf
	default:
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(n))
		return buf
	}
}

func decodeUint64(buf []byte) (uint64, error) {
	switch len(buf) {
	case 1:
		return uint64(buf[0]), nil
	case 2:
		return uint64(binary.BigEndian.Uint16(buf)), nil
	case 4:
		return uint64(binary.BigEndian.Uint32(buf)), nil
	case 8:
		return uint64(binary.BigEndian.Uint64(buf)), nil
	default:
		return 0, fmt.Errorf("invalid uint64 length")
	}
}
