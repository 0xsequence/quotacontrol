package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewBackend(rdb *redis.Client, ttl time.Duration) *Backend {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &Backend{client: rdb, ttl: ttl}
}

type Backend struct {
	client *redis.Client
	ttl    time.Duration
}

func (r *Backend) Get(ctx context.Context, key Key, dst any) (bool, error) {
	data, err := r.client.Get(ctx, key.String()).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("get: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return false, fmt.Errorf("get unmarshal: %w", err)
	}
	return true, nil
}

func (r *Backend) Set(ctx context.Context, key Key, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("set marshal: %w", err)
	}
	if err := r.client.Set(ctx, key.String(), data, r.ttl).Err(); err != nil {
		return fmt.Errorf("set: %w", err)
	}
	return nil
}

func (r *Backend) Clear(ctx context.Context, key Key) (bool, error) {
	count, err := r.client.Del(ctx, key.String()).Result()
	if err != nil {
		return false, fmt.Errorf("clear: %w", err)
	}
	return count != 0, nil
}

type RedisCache[K Key, T any] struct {
	*Backend
}

func (r RedisCache[K, T]) Get(ctx context.Context, key K) (v T, ok bool, err error) {
	ok, err = r.Backend.Get(ctx, key, &v)
	if err != nil {
		return v, false, fmt.Errorf("redis cache get: %w", err)
	}
	return v, ok, nil
}

func (r RedisCache[K, T]) Set(ctx context.Context, key K, value T) (err error) {
	if err = r.Backend.Set(ctx, key, value); err != nil {
		return fmt.Errorf("redis cache set: %w", err)
	}
	return nil
}

func (r RedisCache[K, T]) Clear(ctx context.Context, key K) (ok bool, err error) {
	ok, err = r.Backend.Clear(ctx, key)
	if err != nil {
		return false, fmt.Errorf("redis cache clear: %w", err)
	}
	return ok, nil
}

type redisUsage[K Key] struct {
	RedisCache[K, int64]
}

// NewUsageCache creates a new usage cache for tracking usage counters
func NewUsageCache[K Key](backend *Backend) Usage[K] {
	return &redisUsage[K]{RedisCache: RedisCache[K, int64]{Backend: backend}}
}

var (
	ErrCacheReady   = errors.New("cache: ready for initialization")
	ErrCacheWait    = errors.New("cache: wait")
	ErrCacheTimeout = errors.New("cache: timeout")
)

func (r *redisUsage[K]) Ensure(ctx context.Context, fetcher Fetcher[K], key K) (int64, error) {
	for i := range 3 {
		usage, err := r.peek(ctx, key)
		if err != nil {
			// Some other client is updating the cache, wait and retry.
			if errors.Is(err, ErrCacheWait) {
				time.Sleep(time.Millisecond * 100 * time.Duration(i+1))
				continue
			}
			// PeekUsage found nil and set the cache to -1, expecting the client to set the usage.
			if errors.Is(err, ErrCacheReady) {
				usage, err := fetcher(ctx, key)
				if err != nil {
					return 0, fmt.Errorf("get account usage: %w", err)
				}

				if err := r.Set(ctx, key, usage); err != nil {
					return 0, fmt.Errorf("set usage cache: %w", err)
				}
				return usage, nil
			}

			return 0, fmt.Errorf("peek usage cache: %w", err)
		}
		return usage, nil
	}
	return 0, ErrCacheTimeout
}

func (r *redisUsage[K]) peek(ctx context.Context, key K) (int64, error) {
	const SpecialValue = -1

	v, err := r.client.Get(ctx, key.String()).Int64()
	if err == nil {
		if v == SpecialValue {
			return 0, ErrCacheWait
		}
		return v, nil
	}
	if !errors.Is(err, redis.Nil) {
		var numErr *strconv.NumError
		if !errors.As(err, &numErr) {
			return 0, fmt.Errorf("peek usage - get: %w", err)
		}
		counter, err := r.repair(ctx, key, numErr.Num)
		if err != nil {
			return 0, fmt.Errorf("peek usage - repair: %w", err)
		}
		return counter, nil
	}
	ok, err := r.client.SetNX(ctx, key.String(), SpecialValue, time.Second*2).Result()
	if err != nil {
		return 0, fmt.Errorf("peek usage - setnx: %w", err)
	}
	if !ok {
		return 0, ErrCacheWait
	}
	return 0, ErrCacheReady
}

// repairScript rewrites the counter only while it still holds the exact value the caller
// read, so a client slow to notice the bad value cannot undo a repair another one has
// already made, and the sentinel keeps its meaning as the initialization lock.
var repairScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	redis.call("SET", KEYS[1], ARGV[2], "KEEPTTL")
	return 1
end
return 0
`)

// repair recovers a counter that redis wrote in scientific notation. Redis 7.2.0 to 7.2.4
// formats any multiple of 1e8 that way when a script hands it a lua number
// (redis/redis#13113), which leaves the amount intact but unreadable by ParseInt. Reading it
// back beats reseeding from the store, which would drop everything spent since the last sync.
func (r *redisUsage[K]) repair(ctx context.Context, key K, raw string) (int64, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return 0, fmt.Errorf("counter %q is not an integer", raw)
	}
	counter := int64(f)
	if err := repairScript.Run(ctx, r.client, []string{key.String()}, raw, counter).Err(); err != nil {
		return 0, err
	}
	return counter, nil
}

// spendScript is a Lua script that atomically increments the usage counter by a given amount, but not exceeding the limit.
// It returns the new counter value and the actual amount spent (which may be less than the requested amount if it hits the limit).
//
// Both writes pass their value through as a string and let redis do the arithmetic. Handing
// redis a lua number instead makes it format a double, which renders any multiple of 1e8 as
// "1e+8" on redis 7.2.0 to 7.2.4 (redis/redis#13113) and poisons the counter for good.
var spendScript = redis.NewScript(`
local limit = tonumber(ARGV[2]) or 0
local current = tonumber(redis.call("GET", KEYS[1]) or 0)
if current >= limit then
	return {current, 0}
end
local newValue = redis.call("INCRBY", KEYS[1], ARGV[1])
if newValue > limit then
	redis.call("SET", KEYS[1], ARGV[2], "KEEPTTL")
	newValue = limit
end
return {newValue, newValue - current}
`)

func (r *redisUsage[K]) Spend(ctx context.Context, fetcher Fetcher[K], key K, amount, limit int64) (counter int64, spent int64, err error) {
	v, err := r.Ensure(ctx, fetcher, key)
	if err != nil {
		return 0, 0, fmt.Errorf("spend - ensure: %w", err)
	}
	if v >= limit {
		return v, 0, nil
	}

	res, err := spendScript.Run(ctx, r.client, []string{key.String()}, amount, limit).Result()
	if err != nil {
		return v, 0, fmt.Errorf("spend - script: %w", err)
	}

	values, ok := res.([]any)
	if !ok || len(values) != 2 {
		return 0, 0, fmt.Errorf("spend - script: unexpected result type")
	}

	if counter, ok = values[0].(int64); !ok {
		return 0, 0, fmt.Errorf("spend - script: unexpected result type")
	}
	if spent, ok = values[1].(int64); !ok {
		return 0, 0, fmt.Errorf("spend - script: unexpected result type")
	}

	return counter, spent, nil
}
