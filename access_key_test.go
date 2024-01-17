package quotacontrol_test

import (
	"fmt"
	"testing"

	"github.com/0xsequence/quotacontrol"
	"github.com/stretchr/testify/require"
)

func TestAccessKeyV1(t *testing.T) {
	projectID := uint64(12345)
	accessKey := quotacontrol.GenerateAccessKey(projectID)
	fmt.Println("=> k", accessKey)

	outID, err := quotacontrol.DecodeProjectID(accessKey)
	require.NoError(t, err)
	require.Equal(t, projectID, outID)
}
