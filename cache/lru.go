package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

func NewMemory[K Key, T any](backend Cache[K, T], size int, ttl time.Duration) *Memory[K, T] {
	if ttl <= 0 {
		ttl = time.Minute
	}
	if size <= 0 {
		size = 1000
	}
	return &Memory[K, T]{
		mem:     expirable.NewLRU[string, T](size, nil, ttl),
		backend: backend,
	}
}

type Memory[K Key, T any] struct {
	// mem is in-memory lru cache layer
	mem *expirable.LRU[string, T]
	// backend is pluggable layer, which usually is redis
	backend Cache[K, T]
}

func (s *Memory[K, T]) Get(ctx context.Context, key K) (v T, ok bool, err error) {
	if v, ok = s.mem.Get(key.String()); ok {
		return v, true, nil
	}

	if s.backend != nil {
		v, ok, err = s.backend.Get(ctx, key)
		if err != nil {
			return v, false, fmt.Errorf("lru get backend: %w", err)
		}
	}

	if s.backend == nil || ok {
		s.mem.Add(key.String(), v) // Add to LRU cache, but don't overwrite ok
	}
	return v, ok, nil
}

func (s *Memory[K, T]) Set(ctx context.Context, key K, value T) (err error) {
	if s.backend != nil {
		if err = s.backend.Set(ctx, key, value); err != nil {
			return fmt.Errorf("lru set backend: %w", err)
		}
	}

	s.mem.Add(key.String(), value)

	return nil
}

func (s *Memory[K, T]) Clear(ctx context.Context, key K) (ok bool, err error) {
	if s.backend != nil {
		if ok, err = s.backend.Clear(ctx, key); err != nil {
			return false, fmt.Errorf("lru clear backend: %w", err)
		}
	}
	if s.backend == nil || ok {
		ok = s.mem.Remove(key.String())
	}
	return ok, nil
}
