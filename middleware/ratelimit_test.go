package middleware_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit(t *testing.T) {
	const (
		RateLimitTypePublic1 middleware.RateType = "public1"
		RateLimitTypePublic2 middleware.RateType = "public2"
	)

	rl := middleware.NewRateLimitSettings(
		&localCounter{
			counters:     make(map[uint64]*count),
			windowLength: time.Minute,
		},
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			proto.RespondWithError(w, proto.ErrLimitExceeded)
		}),
	)
	rl.RegisterRateLimit(RateLimitTypePublic1, 10, httprate.WithKeyFuncs(httprate.KeyByRealIP))
	rl.RegisterRateLimit(RateLimitTypePublic2, 20, httprate.WithKeyFuncs(httprate.KeyByRealIP))

	handler := middleware.NewRateLimit(rl, 1*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	ip := net.IPv4(192, 168, 0, 1)

	for i := 0; i < 40; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip.String()
		// req 1-20 are type 1, req 21-40 are type 2
		rlType := RateLimitTypePublic1
		if i >= 20 {
			rlType = RateLimitTypePublic2
		}
		req = req.WithContext(middleware.WithRateLimitType(req.Context(), rlType))

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		expected := http.StatusOK
		// req 10-20 and 30-40 should be limited
		if (i >= 10 && i < 20) || i >= 30 {
			expected = http.StatusTooManyRequests
		}
		assert.Equal(t, expected, w.Code, "i=%d", i)
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
