package api

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// ─── Per-resource / per-action rate limiter (todo 7.12) ───
//
// Enforces EntitySpec.RateLimit (resource-level) and Action.RateLimit
// (per-action override) from 02-core-extended.md §17. Single-server
// in-memory only — a shared limiter belongs in the Control Plane / Redis
// layer (the same caveat as the auth rate limiter).
//
// Strategies:
//   - token_bucket: burst `max` tokens, refilled at `max / per` per second.
//   - sliding_window: a fixed window of `per` with a sliding counter — the
//     effective count is the previous window's count scaled by elapsed time
//     plus the current window's count.
//
// Scope determines the key: tenant | user | ip | global.

type resourceBucket struct {
	// token_bucket state
	tokens float64
	last   time.Time
	// sliding_window state
	windowStart time.Time
	windowCount int
	prevCount   int
}

// ResourceRateLimiter is a process-global, keyed rate limiter for resource
// and action rate limits.
type ResourceRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*resourceBucket
}

// NewResourceRateLimiter creates an empty resource rate limiter.
func NewResourceRateLimiter() *ResourceRateLimiter {
	return &ResourceRateLimiter{buckets: map[string]*resourceBucket{}}
}

// parsePer converts a "per" duration string ("1s", "1m", "1h", "1d") to
// seconds. Returns 0 for an unparseable value (caller treats as no limit).
func parsePer(per string) float64 {
	per = strings.TrimSpace(per)
	if per == "" {
		return 0
	}
	mult := 1.0
	switch per[len(per)-1] {
	case 's':
		mult = 1
	case 'm':
		mult = 60
	case 'h':
		mult = 3600
	case 'd':
		mult = 86400
	default:
		// bare number → seconds
		mult = 1
	}
	n, err := strconv.ParseFloat(strings.TrimRight(per, "smhd"), 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n * mult
}

// Allow reports whether a request for key is permitted under the given spec,
// consuming one unit if so. A nil spec or unparseable "per" → always allowed.
func (rl *ResourceRateLimiter) Allow(rs *spec.RateLimitSpec, key string) bool {
	if rs == nil || rs.Max <= 0 {
		return true
	}
	perSec := parsePer(rs.Per)
	if perSec <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &resourceBucket{
			tokens:      float64(rs.Max),
			last:        now,
			windowStart: now,
			windowCount: 0,
			prevCount:   0,
		}
		rl.buckets[key] = b
	}

	switch rs.Strategy {
	case "sliding_window":
		return rl.allowSlidingWindow(b, rs, perSec, now)
	default: // token_bucket (default)
		return rl.allowTokenBucket(b, rs, perSec, now)
	}
}

func (rl *ResourceRateLimiter) allowTokenBucket(b *resourceBucket, rs *spec.RateLimitSpec, perSec float64, now time.Time) bool {
	elapsed := now.Sub(b.last).Seconds()
	refillRate := float64(rs.Max) / perSec
	b.tokens += elapsed * refillRate
	if b.tokens > float64(rs.Max) {
		b.tokens = float64(rs.Max)
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func (rl *ResourceRateLimiter) allowSlidingWindow(b *resourceBucket, rs *spec.RateLimitSpec, perSec float64, now time.Time) bool {
	window := time.Duration(perSec * float64(time.Second))
	// Advance windows.
	for !now.Before(b.windowStart.Add(window)) {
		b.prevCount = b.windowCount
		b.windowCount = 0
		b.windowStart = b.windowStart.Add(window)
	}
	// Weighted estimate: previous window scaled by elapsed fraction + current.
	elapsedFrac := now.Sub(b.windowStart).Seconds() / window.Seconds()
	estimate := float64(b.prevCount)*(1-elapsedFrac) + float64(b.windowCount)
	if estimate >= float64(rs.Max) {
		return false
	}
	b.windowCount++
	return true
}

// rateLimitKey derives the limiter key for a request under a scope.
func rateLimitKey(scope string, r *http.Request) string {
	switch scope {
	case "user":
		if id := IdentityFromContext(r.Context()); id != nil && id.UserID != "" {
			return "user:" + id.UserID
		}
		return "user:anonymous"
	case "ip":
		return "ip:" + clientIP(r)
	case "global":
		return "global"
	default: // tenant
		return "tenant:" + workspaceFromContext(r.Context())
	}
}

// checkRateLimit enforces the resource-level and per-action rate limits for
// an entity action. Returns true when the request is allowed; when false,
// it has already written a 429 response.
func (f *HandlerFactory) checkRateLimit(w http.ResponseWriter, r *http.Request, es *spec.EntitySpec, actionName string) bool {
	if f.rateLimiter == nil {
		return true
	}
	if es == nil {
		return true
	}

	// Per-action override wins over the resource-level default.
	rs := es.RateLimit
	if a := resolveAction(es, actionName); a != nil && a.RateLimit != nil {
		rs = a.RateLimit
	}
	if rs == nil {
		return true
	}

	scope := rs.Scope
	if scope == "" {
		scope = "tenant"
	}
	// Key includes the action so each action's budget is independent (a
	// per-action override replaces the resource default for that action).
	key := rateLimitKey(scope, r) + ":" + actionName
	if !f.rateLimiter.Allow(rs, key) {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED",
			"rate limit exceeded for "+actionName)
		return false
	}
	return true
}
