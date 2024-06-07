package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
)

const _RateLimitWindow = 1 * time.Minute

func RateLimit(limitCounter httprate.LimitCounter, defaultRPM int, keyFn httprate.KeyFunc, errLimit error) func(next http.Handler) http.Handler {
	if keyFn == nil {
		keyFn = httprate.KeyByRealIP
	}

	if errLimit == nil {
		errLimit = proto.ErrLimitExceeded
	}

	if limitCounter == nil {
		limitCounter = &localCounter{
			counters:     make(map[uint64]*count),
			windowLength: _RateLimitWindow,
		}
	}

	options := []httprate.Option{
		httprate.WithLimitCounter(limitCounter),
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			if q := GetAccessQuota(r.Context()); q != nil {
				return ProjectRateKey(q.GetProjectID()), nil
			}
			return keyFn(r)
		}),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			proto.RespondWithError(w, errLimit)
		}),
	}

	// The rate limiter middleware
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			rpm, ok := getRateLimit(ctx)
			if !ok {
				rpm = defaultRPM
			}

			// if rate limit is set to 0 skip
			if rpm < 1 {
				next.ServeHTTP(w, r)
				return
			}

			httprate.Limit(rpm, _RateLimitWindow, options...)(next).ServeHTTP(w, r)
		})
	}
}

type localCounter struct {
	counters     map[uint64]*count
	windowLength time.Duration
	lastEvict    time.Time
	mu           sync.Mutex
}

var _ httprate.LimitCounter = &localCounter{}

type count struct {
	value     int
	updatedAt time.Time
}

func (c *localCounter) Config(requestLimit int, windowLength time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.windowLength = windowLength
}

func (c *localCounter) Increment(key string, currentWindow time.Time) error {
	return c.IncrementBy(key, currentWindow, 1)
}

func (c *localCounter) IncrementBy(key string, currentWindow time.Time, amount int) error {
	c.evict()

	c.mu.Lock()
	defer c.mu.Unlock()

	hkey := httprate.LimitCounterKey(key, currentWindow)

	v, ok := c.counters[hkey]
	if !ok {
		v = &count{}
		c.counters[hkey] = v
	}
	v.value += amount
	v.updatedAt = time.Now()

	return nil
}

func (c *localCounter) Get(key string, currentWindow, previousWindow time.Time) (int, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	curr, ok := c.counters[httprate.LimitCounterKey(key, currentWindow)]
	if !ok {
		curr = &count{value: 0, updatedAt: time.Now()}
	}
	prev, ok := c.counters[httprate.LimitCounterKey(key, previousWindow)]
	if !ok {
		prev = &count{value: 0, updatedAt: time.Now()}
	}

	return curr.value, prev.value, nil
}

func (c *localCounter) evict() {
	c.mu.Lock()
	defer c.mu.Unlock()

	d := c.windowLength * 3

	if time.Since(c.lastEvict) < d {
		return
	}
	c.lastEvict = time.Now()

	for k, v := range c.counters {
		if time.Since(v.updatedAt) >= d {
			delete(c.counters, k)
		}
	}
}
