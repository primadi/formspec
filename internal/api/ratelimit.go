package api

import (
	"sync"
	"time"
)

// rateLimiter is a simple in-memory token-bucket rate limiter (todo 6.6.3).
// It is used to rate-limit auth endpoints per method/key (e.g. per IP or per
// username) to slow down brute-force attempts. Not distributed — single-server
// only; a shared limiter belongs in the Control Plane / Redis layer.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens refilled per second
	burst   int     // max tokens (burst size)
}

type bucket struct {
	tokens float64
	last   time.Time
}

// newRateLimiter creates a token-bucket limiter that allows `burst` requests
// immediately, then refills at `rate` tokens/second.
func newRateLimiter(rate float64, burst int) *rateLimiter {
	return &rateLimiter{
		buckets: map[string]*bucket{},
		rate:    rate,
		burst:   burst,
	}
}

// Allow reports whether a request for key is permitted now, consuming one
// token if so.
func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(rl.burst), last: now}
		rl.buckets[key] = b
	}

	// Refill based on elapsed time.
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// ResetAuthRateLimiters resets the global auth rate limiters. Used by tests
// to isolate rate-limit state between cases (the limiters are process-global
// and keyed by client IP).
func ResetAuthRateLimiters() {
	loginLimiter = newRateLimiter(0.5, 5)
	refreshLimiter = newRateLimiter(1, 10)
}
