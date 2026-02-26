package cache

import (
	"context"
	"fmt"
)

// Key is a type that can be used as a key in the cache. It must implement the fmt.Stringer interface.
type Key fmt.Stringer

// Simple is a generic cache for objects.
type Simple[K Key, T any] interface {
	// Get returns the value.
	Get(ctx context.Context, key K) (v T, ok bool, err error)
	// Set sets the value
	Set(ctx context.Context, key K, value T) (err error)
	// Clear removes the value.
	Clear(ctx context.Context, key K) (ok bool, err error)
}

type Fetcher[K Key] func(ctx context.Context, key K) (counter int64, err error)

// Usage is a cache for usage, that allows peek and spend operations.
type Usage[K Key] interface {
	Simple[K, int64]
	Ensure(ctx context.Context, fetcher Fetcher[K], key K) (counter int64, err error)
	Spend(ctx context.Context, fetcher Fetcher[K], key K, amount, limit int64) (counter int64, spent int64, err error)
}
