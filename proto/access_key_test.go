package proto_test

import (
	"testing"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/require"
)

func TestAccessKeyV1Encoding(t *testing.T) {
	projectID := uint64(12345)
	accessKey := proto.GenerateAccessKey(projectID)
	// fmt.Println("=> k", accessKey)

	outID, err := proto.DecodeProjectID(accessKey)
	require.NoError(t, err)
	require.Equal(t, projectID, outID)

	outID, err = proto.GetProjectID(accessKey)
	require.NoError(t, err)
	require.Equal(t, projectID, outID)
}

func TestAccessKeyLegacyEncoding(t *testing.T) {
	projectID := uint64(12345)
	accessKey := proto.LegacyGenerateAccessKey(projectID)
	// fmt.Println("=> k", accessKey)

	outID, err := proto.LegacyDecodeProjectID(accessKey)
	require.NoError(t, err)
	require.Equal(t, projectID, outID)

	outID, err = proto.GetProjectID(accessKey)
	require.NoError(t, err)
	require.Equal(t, projectID, outID)
}
