package cache

import (
	"context"
)

type Provider string

const (
	ProviderMemory Provider = "memory"
	ProviderRedis  Provider = "redis"
)

// Cache is a generic cache for objects.
type Cache[K Key, T any] interface {
	// Get returns the value.
	Get(ctx context.Context, key K) (v T, ok bool, err error)
	// Set sets the value
	Set(ctx context.Context, key K, value T) (err error)
	// Clear removes the value.
	Clear(ctx context.Context, key K) (ok bool, err error)
}

type Fetcher[K Key] func(ctx context.Context, key K) (counter int64, err error)

// UsageCache is a cache for usage, that allows peek and spend operations.
type UsageCache[K Key] interface {
	Cache[K, int64]
	Ensure(ctx context.Context, fetcher Fetcher[K], key K) (counter int64, err error)
	Spend(ctx context.Context, fetcher Fetcher[K], key K, amount, limit int64) (counter int64, spent int64, err error)
}
