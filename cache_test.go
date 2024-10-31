package quotacontrol_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/assert"
)

type mockCache struct {
	count int32
}

func (s *mockCache) GetAccessQuota(ctx context.Context, accessKey string) (*proto.AccessQuota, error) {
	atomic.AddInt32(&s.count, 1)
	return &proto.AccessQuota{AccessKey: &proto.AccessKey{AccessKey: accessKey}}, nil
}

func (s *mockCache) SetAccessQuota(context.Context, *proto.AccessQuota) error {
	return nil
}

func (s *mockCache) DeleteAccessQuota(context.Context, string) error {
	return nil
}

func (s *mockCache) GetProjectQuota(context.Context, uint64) (*proto.AccessQuota, error) {
	return nil, nil
}

func (s *mockCache) SetProjectQuota(context.Context, *proto.AccessQuota) error {
	return nil
}

func (s *mockCache) DeleteProjectQuota(context.Context, uint64) error {
	return nil
}

func TestLRU(t *testing.T) {
	baseCache := mockCache{}

	lru := quotacontrol.NewLRU(&baseCache, 2, 0)

	ctx := context.Background()

	_, err := lru.GetAccessQuota(ctx, "a")
	assert.NoError(t, err)

	_, err = lru.GetAccessQuota(ctx, "a")
	assert.NoError(t, err)

	assert.Equal(t, int32(1), baseCache.count)
}
