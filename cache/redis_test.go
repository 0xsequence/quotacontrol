package cache_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/0xsequence/quotacontrol/cache"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRedisTestConfig creates a miniredis instance and returns it with a cache config
func newRedisTestConfig(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	host, port, _ := net.SplitHostPort(mr.Addr())
	var portNum uint16
	_, err = fmt.Sscanf(port, "%d", &portNum)
	require.NoError(t, err)

	return mr, redis.NewClient(&redis.Options{Addr: fmt.Sprintf("%s:%d", host, portNum)})
}

// TestRedis tests Redis cache operations
func TestRedis(t *testing.T) {
	mr, client := newRedisTestConfig(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	backend := cache.NewBackend(client, time.Second*10)
	require.NotNil(t, backend)

	// Test basic Set and Get operations
	obj := cache.RedisCache[Key, string]{Backend: backend}
	require.NotNil(t, obj)

	err := obj.Set(ctx, "key1", "value1")
	assert.NoError(t, err)

	v, ok, err := obj.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "value1", v)

	// Test Set and Get with another key
	err = obj.Set(ctx, "key2", "value2")
	assert.NoError(t, err)

	v, ok, err = obj.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "value2", v)

	// Test Clear operation
	ok, err = obj.Clear(ctx, "key1")
	assert.NoError(t, err)
	assert.True(t, ok)

	// Verify key1 is gone
	_, ok, err = obj.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.False(t, ok)

	// Verify key2 still exists
	v, ok, err = obj.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "value2", v)

	// Test Clear on non-existent key
	ok, err = obj.Clear(ctx, "missing")
	assert.NoError(t, err)
	assert.False(t, ok)

	// Test TTL functionality
	backend = cache.NewBackend(client, 100*time.Millisecond)

	cacheWithTTL := cache.RedisCache[Key, string]{Backend: backend}
	require.NotNil(t, cacheWithTTL)

	err = cacheWithTTL.Set(ctx, "expiring", "value")
	assert.NoError(t, err)

	// Value should exist immediately
	v, ok, err = cacheWithTTL.Get(ctx, "expiring")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "value", v)

	// Fast-forward time in miniredis
	mr.FastForward(200 * time.Millisecond)

	// Value should be gone
	_, ok, err = cacheWithTTL.Get(ctx, "expiring")
	assert.NoError(t, err)
	assert.False(t, ok)

	cacheWithKeyFn := cache.RedisCache[UserKey, string]{Backend: backend}
	require.NotNil(t, cacheWithKeyFn)

	err = cacheWithKeyFn.Set(ctx, 123, "user123data")
	assert.NoError(t, err)

	v, ok, err = cacheWithKeyFn.Get(ctx, 123)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "user123data", v)
}

// TestRedisWithLRU tests Redis cache with LRU layer that has shorter expiration
func TestRedisWithLRU(t *testing.T) {
	mr, client := newRedisTestConfig(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Create Redis backend with longer TTL (10 seconds)
	backend := cache.NewBackend(client, 10*time.Second)

	redisCache := cache.RedisCache[Key, string]{Backend: backend}
	require.NotNil(t, redisCache)

	// Create LRU cache on top with very short TTL (100ms)
	lruCache := cache.NewMemory(redisCache, 10, 100*time.Millisecond)
	require.NotNil(t, lruCache)

	// Set a value through LRU (should be stored in both LRU and Redis)
	err := lruCache.Set(ctx, "testkey", "testvalue")
	assert.NoError(t, err)

	// Get immediately - should come from LRU
	v, ok, err := lruCache.Get(ctx, "testkey")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "testvalue", v)

	// Wait for LRU to expire naturally (150ms to be safe)
	time.Sleep(150 * time.Millisecond)

	// Get again through LRU - should fetch from Redis and repopulate LRU
	v, ok, err = lruCache.Get(ctx, "testkey")
	assert.NoError(t, err)
	assert.True(t, ok, "value should still exist in Redis after LRU expiration")
	assert.Equal(t, "testvalue", v)

	// Verify direct Redis access still works
	v, ok, err = redisCache.Get(ctx, "testkey")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "testvalue", v)

	// Test Clear operation (should clear both LRU and Redis)
	err = lruCache.Set(ctx, "key2", "value2")
	assert.NoError(t, err)

	ok, err = lruCache.Clear(ctx, "key2")
	assert.NoError(t, err)
	assert.True(t, ok)

	// Verify it's gone from both LRU and Redis
	_, ok, err = lruCache.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.False(t, ok)

	_, ok, err = redisCache.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.False(t, ok)
}

// TestRedisBackendBasic tests basic Redis backend functionality
func TestRedisBackendBasic(t *testing.T) {
	mr, client := newRedisTestConfig(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	backend := cache.NewBackend(client, 10*time.Second)

	redisCache := cache.RedisCache[Key, string]{Backend: backend}

	// Test basic set/get
	err := redisCache.Set(ctx, "key1", "value1")
	require.NoError(t, err)

	v, ok, err := redisCache.Get(ctx, "key1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "value1", v)

	// Test LRU wrapping Redis with longer Redis TTL and shorter LRU TTL
	lru := cache.NewMemory(redisCache, 10, 100*time.Millisecond)

	err = lru.Set(ctx, "key2", "value2")
	require.NoError(t, err)

	// Get from LRU immediately - should hit LRU cache
	v, ok, err = lru.Get(ctx, "key2")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "value2", v)

	// Verify it's in Redis
	v, ok, err = redisCache.Get(ctx, "key2")
	require.NoError(t, err)
	require.True(t, ok, "should be in Redis backend")
	require.Equal(t, "value2", v)

	// Wait for LRU to expire
	time.Sleep(150 * time.Millisecond)

	// Check Redis still has it
	v, ok, err = redisCache.Get(ctx, "key2")
	require.NoError(t, err)
	require.True(t, ok, "should still be in Redis after LRU expires")
	require.Equal(t, "value2", v)

	// Get through LRU should fetch from Redis
	v, ok, err = lru.Get(ctx, "key2")
	require.NoError(t, err)
	require.True(t, ok, "LRU should fetch from Redis backend")
	require.Equal(t, "value2", v)
}

// TestUsageConcurrency tests parallel execution of Peek and Spend to ensure no race conditions
func TestUsageConcurrency(t *testing.T) {
	mr, client := newRedisTestConfig(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	backend := cache.NewBackend(client, 10*time.Second)

	// Simulate fetching usage from a database or external service
	fetcher := func(ctx context.Context, key UsageKey) (int64, error) {
		return 0, nil
	}

	usage := cache.NewUsageCache[UsageKey](backend)
	require.NotNil(t, usage)

	const (
		numGoroutines = 50
		spendAmount   = int64(1)
		limit         = int64(1000)
		key           = 123
	)

	// Test 1: Concurrent Peek operations
	t.Run("concurrent peek", func(t *testing.T) {
		// Clear the key first
		_, err := usage.Clear(ctx, key)
		assert.NoError(t, err)

		var wg sync.WaitGroup
		errs := make(chan error, numGoroutines)

		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				counter, err := usage.Ensure(ctx, fetcher, key)
				if err != nil && !errors.Is(err, cache.ErrCacheReady) && !errors.Is(err, cache.ErrCacheWait) {
					errs <- err
					return
				}
				// First peek should return 0 or ErrCacheReady/ErrCacheWait
				if err == nil && counter < 0 {
					errs <- fmt.Errorf("unexpected negative counter: %d", counter)
				}
			}()
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			t.Errorf("Peek error: %v", err)
		}
	})

	// Test 2: Concurrent Spend operations
	t.Run("concurrent spend", func(t *testing.T) {
		// Reset by clearing the key
		_, err := usage.Clear(ctx, key)
		assert.NoError(t, err)

		var wg sync.WaitGroup
		successCount := int64(0)
		var mu sync.Mutex
		errs := make(chan error, numGoroutines)

		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				counter, spent, err := usage.Spend(ctx, fetcher, key, spendAmount, limit)
				if err != nil {
					errs <- err
					return
				}
				if counter > limit {
					errs <- fmt.Errorf("counter exceeded limit: %d > %d", counter, limit)
				}
				if spent == spendAmount {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			t.Errorf("Spend error: %v", err)
		}

		// Verify final counter value
		finalCounter, ok, err := usage.Get(ctx, key)
		require.NoError(t, err)
		require.True(t, ok)

		// Final counter should equal successful spends * amount
		expectedCounter := successCount * spendAmount
		assert.Equal(t, expectedCounter, finalCounter,
			"final counter should match successful spend operations")

		// Verify counter didn't exceed limit
		assert.LessOrEqual(t, finalCounter, limit,
			"final counter should not exceed limit")
	})

	// Test 3: Mixed Peek and Spend operations
	t.Run("mixed peek and spend", func(t *testing.T) {
		// Reset by clearing the key
		_, err := usage.Clear(ctx, key)
		assert.NoError(t, err)

		var wg sync.WaitGroup
		errs := make(chan error, numGoroutines*2)

		// Launch Spend goroutines
		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				counter, spent, err := usage.Spend(ctx, fetcher, key, spendAmount, limit)
				if err != nil {
					errs <- err
					return
				}
				if counter > limit {
					errs <- fmt.Errorf("counter exceeded limit: %d > %d", counter, limit)
				}
				// Spend should return non-negative spent amount
				if spent < 0 {
					errs <- fmt.Errorf("spend returned negative amount: %d", spent)
				}
				// Spend should not exceed the requested amount
				if spent > spendAmount {
					errs <- fmt.Errorf("spend returned more than requested: %d", spent)
				}
				// If spent is 0, it means limit was reached
				if spent == 0 {
					// Verify that the counter is at or above the limit
					counter, ok, err := usage.Get(ctx, key)
					if err != nil {
						errs <- fmt.Errorf("get after spend: %w", err)
						return
					}
					if ok && counter < limit {
						errs <- fmt.Errorf("spend returned 0 but counter %d is below limit %d", counter, limit)
					}
				}
			}()
		}

		// Launch Peek goroutines interleaved
		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				counter, err := usage.Ensure(ctx, fetcher, key)
				if err != nil && err.Error() != "cache ready for initialization" &&
					err.Error() != "cache wait" {
					errs <- err
					return
				}
				// Peek should return non-negative values
				if err == nil && counter < 0 {
					errs <- fmt.Errorf("peek returned negative counter: %d", counter)
				}
			}()
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			t.Errorf("Mixed operation error: %v", err)
		}

		// Verify final state is consistent
		finalCounter, ok, err := usage.Get(ctx, key)
		require.NoError(t, err)
		require.True(t, ok)

		// Counter should be a multiple of spendAmount
		assert.Equal(t, int64(0), finalCounter%spendAmount,
			"final counter should be a multiple of spend amount")

		// Counter should not exceed limit
		assert.LessOrEqual(t, finalCounter, limit,
			"final counter should not exceed limit")
	})

	// Test 4: Spend with limit enforcement
	t.Run("spend limit enforcement", func(t *testing.T) {
		// Reset and pre-populate with a value close to limit
		limit := int64(100)
		preloadValue := int64(85)

		err := usage.Set(ctx, key, preloadValue)
		assert.NoError(t, err)

		var wg sync.WaitGroup
		successCount := int64(0)
		failureCount := int64(0)
		var mu sync.Mutex
		errs := make(chan error, numGoroutines)

		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				counter, spent, err := usage.Spend(ctx, fetcher, key, spendAmount, limit)
				if err != nil {
					errs <- err
					return
				}
				if counter > limit {
					errs <- fmt.Errorf("counter exceeded limit: %d > %d", counter, limit)
				}

				mu.Lock()
				if spent == spendAmount {
					successCount++
				} else {
					failureCount++
				}
				mu.Unlock()
			}()
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			t.Errorf("Limit enforcement error: %v", err)
		}

		// Verify some spends succeeded and some failed
		assert.Greater(t, successCount, int64(0), "some spends should succeed")
		assert.Greater(t, failureCount, int64(0), "some spends should fail due to limit")

		// Verify final counter
		finalCounter, ok, err := usage.Get(ctx, key)
		require.NoError(t, err)
		require.True(t, ok)

		// Final counter should not exceed limit
		assert.LessOrEqual(t, finalCounter, limit, "final counter must not exceed limit")
	})
}
