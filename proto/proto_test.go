package proto_test

import (
	"testing"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/assert"
)

func TestServiceGetQuotaKey(t *testing.T) {
	s := proto.Service_Indexer
	now := time.Date(2023, time.April, 1, 0, 0, 0, 0, time.Local)
	assert.Equal(t, s.GetQuotaKey(777, now), "Indexer_777_2023-04")
}

func TestAccessTokenValidateOrigin(t *testing.T) {
	t.Run("no allowed origins", func(t *testing.T) {
		tk := &proto.AccessToken{}
		assert.True(t, tk.ValidateOrigin("http://localhost:8080"))
	})

	t.Run("allowed origins", func(t *testing.T) {
		tk := &proto.AccessToken{
			AllowedOrigins: []string{"http://localhost:8080", "http://localhost:8081"},
		}
		assert.True(t, tk.ValidateOrigin("http://localhost:8080"))
		assert.True(t, tk.ValidateOrigin("http://localhost:8081"))
		assert.False(t, tk.ValidateOrigin("http://localhost:8082"))
	})
}
