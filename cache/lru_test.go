package cache_test

import (
	"context"
	"testing"

	"github.com/0xsequence/quotacontrol/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLRU tests LRU cache operations
func TestLRU(t *testing.T) {
	memCache := cache.NewMemory[Key, string](nil, 0, 0)
	require.NotNil(t, memCache)

	ctx := context.Background()

	// Test Set operation
	err := memCache.Set(ctx, "key1", "value1")
	assert.NoError(t, err)

	// Test Get operation - should retrieve from cache
	v, ok, err := memCache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "value1", v)

	// Test Get again - should still work
	v, ok, err = memCache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "value1", v)

	// Test Set with another key
	err = memCache.Set(ctx, "key2", "value2")
	assert.NoError(t, err)

	// Verify we can get the second key back
	v, ok, err = memCache.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "value2", v)

	// Test Clear operation on first key
	ok, err = memCache.Clear(ctx, "key1")
	assert.NoError(t, err)
	assert.True(t, ok)

	// Verify key1 is removed from cache
	_, ok, err = memCache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.False(t, ok)

	// Verify key2 still exists
	v, ok, err = memCache.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "value2", v)

	// Test Clear on non-existent key
	ok, err = memCache.Clear(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.False(t, ok)
}
