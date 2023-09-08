package middleware_test

import (
	"context"
	"testing"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/assert"
)

func TestToken(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, middleware.GetToken(ctx))

	ctx = middleware.WithToken(ctx, &proto.CachedToken{})
	assert.NotNil(t, middleware.GetToken(ctx))
}

func TestSkipRateLimit(t *testing.T) {
	ctx := context.Background()
	assert.False(t, middleware.IsSkipRateLimit(ctx))

	ctx = middleware.WithSkipRateLimit(ctx)
	assert.True(t, middleware.IsSkipRateLimit(ctx))
}

func TestComputeUnits(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, int64(1), middleware.GetComputeUnits(ctx))

	ctx = middleware.AddComputeUnits(ctx, 10)
	assert.Equal(t, int64(10), middleware.GetComputeUnits(ctx))

	ctx = middleware.WithComputeUnits(ctx, 10)
	assert.Equal(t, int64(10), middleware.GetComputeUnits(ctx))

	ctx = middleware.AddComputeUnits(ctx, 10)
	assert.Equal(t, int64(20), middleware.GetComputeUnits(ctx))
}
