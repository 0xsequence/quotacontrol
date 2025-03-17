package encoding

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/goware/base64"
	"github.com/jxskiss/base62"
)

var (
	ErrInvalidKeyLength = errors.New("invalid access key length")
)

type Encoding interface {
	Version() byte
	Encode(ctx context.Context, projectID uint64) string
	Decode(accessKey string) (projectID uint64, err error)
}

const (
	sizeV0 = 24
	sizeV1 = 26
	sizeV2 = 32
)

// V0: base62 encoded, 24-byte fixed length. 8 bytes for project ID, rest random.
// Uses custom base62, limiting cross-language compatibility.
type V0 struct{}

func (V0) Version() byte { return 0 }

func (V0) Encode(_ context.Context, projectID uint64) string {
	buf := make([]byte, sizeV0)
	binary.BigEndian.PutUint64(buf, projectID)
	rand.Read(buf[8:])
	return base62.EncodeToString(buf)
}

func (V0) Decode(accessKey string) (projectID uint64, err error) {
	buf, err := base62.DecodeString(accessKey)
	if err != nil {
		return 0, fmt.Errorf("base62 decode: %w", err)
	}
	if len(buf) != sizeV0 {
		return 0, ErrInvalidKeyLength
	}
	return binary.BigEndian.Uint64(buf[:8]), nil
}

// V1: base64 encoded, 26-byte fixed length. 1 byte for version, 8 bytes for project ID, rest random.
// Uses standard base64url encoding. Compatible with other systems.
type V1 struct{}

func (V1) Version() byte { return 1 }

func (v V1) Encode(_ context.Context, projectID uint64) string {
	buf := make([]byte, sizeV1)
	buf[0] = v.Version()
	binary.BigEndian.PutUint64(buf[1:], projectID)
	rand.Read(buf[9:])
	return base64.Base64UrlEncode(buf)
}

func (V1) Decode(accessKey string) (projectID uint64, err error) {
	buf, err := base64.Base64UrlDecode(accessKey)
	if err != nil {
		return 0, fmt.Errorf("base64 decode: %w", err)
	}
	if len(buf) != sizeV1 {
		return 0, ErrInvalidKeyLength
	}
	return binary.BigEndian.Uint64(buf[1:9]), nil
}

// V2: base64 encoded, 32-byte fixed length. 1 byte for version, 8 bytes for project ID, rest random.
// Uses ":" as separator between prefix and base64 encoded data.
type V2 struct{}

const (
	Separator     = ":"
	DefaultPrefix = "seq"
)

func (V2) Version() byte { return 2 }

func (v V2) Encode(ctx context.Context, projectID uint64) string {
	buf := make([]byte, sizeV2)
	buf[0] = v.Version()
	binary.BigEndian.PutUint64(buf[1:], projectID)
	rand.Read(buf[9:])
	return getPrefix(ctx) + Separator + base64.Base64UrlEncode(buf)
}

func (V2) Decode(accessKey string) (projectID uint64, err error) {
	parts := strings.Split(accessKey, Separator)
	if len(parts) < 2 {
		return 0, ErrInvalidKeyLength
	}
	accessKey = parts[len(parts)-1]

	buf, err := base64.Base64UrlDecode(accessKey)
	if err != nil {
		return 0, fmt.Errorf("base64 decode: %w", err)
	}
	if len(buf) != sizeV2 {
		return 0, ErrInvalidKeyLength
	}
	return binary.BigEndian.Uint64(buf[1:9]), nil
}
