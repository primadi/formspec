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
	// Seconds multiplier for the unit suffix; a bare number means seconds.
	var mult float64
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
	b := rl.ensureBucket(key, rs, now)

	switch rs.Strategy {
	case "sliding_window":
		return rl.allowSlidingWindow(b, rs, perSec, now)
	default: // token_bucket (default)
		return rl.allowTokenBucket(b, rs, perSec, now)
	}
}

// Observe records one hit for key and returns the current utilization — the
// fraction of the budget in use (0 empty, 1 exhausted). It never denies.
//
// It exists for the intake gate's pressure signal (plan
// docs_internal/plan/intake-challenge-pow.md): the signal must count every
// anonymous request, including those the gate is not yet challenging, because a
// signal that only counted while the gate was already on could never turn it on.
// The key is dedicated to that signal, so consuming a token here is the point.
func (rl *ResourceRateLimiter) Observe(rs *spec.RateLimitSpec, key string) float64 {
	if rs == nil || rs.Max <= 0 {
		return 0
	}
	perSec := parsePer(rs.Per)
	if perSec <= 0 {
		return 0
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b := rl.ensureBucket(key, rs, now)

	var u float64
	if rs.Strategy == "sliding_window" {
		u = rl.observeSlidingWindow(b, rs, perSec, now)
	} else {
		u = rl.observeTokenBucket(b, rs, perSec, now)
	}
	if u < 0 {
		u = 0
	}
	if u > 1 {
		u = 1
	}
	return u
}

// ensureBucket returns the bucket for key, creating a full one when absent.
// Callers hold rl.mu.
func (rl *ResourceRateLimiter) ensureBucket(key string, rs *spec.RateLimitSpec, now time.Time) *resourceBucket {
	b, ok := rl.buckets[key]
	if !ok {
		b = &resourceBucket{
			tokens:      float64(rs.Max),
			last:        now,
			windowStart: now,
		}
		rl.buckets[key] = b
	}
	return b
}

// refillBucket adds the tokens accrued since the last call, capped at Max.
// Callers hold rl.mu.
func (rl *ResourceRateLimiter) refillBucket(b *resourceBucket, rs *spec.RateLimitSpec, perSec float64, now time.Time) {
	elapsed := now.Sub(b.last).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	b.tokens += elapsed * (float64(rs.Max) / perSec)
	if b.tokens > float64(rs.Max) {
		b.tokens = float64(rs.Max)
	}
	b.last = now
}

func (rl *ResourceRateLimiter) allowTokenBucket(b *resourceBucket, rs *spec.RateLimitSpec, perSec float64, now time.Time) bool {
	rl.refillBucket(b, rs, perSec, now)
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// observeTokenBucket refills, consumes the hit, and reports utilization.
func (rl *ResourceRateLimiter) observeTokenBucket(b *resourceBucket, rs *spec.RateLimitSpec, perSec float64, now time.Time) float64 {
	rl.refillBucket(b, rs, perSec, now)
	if b.tokens >= 1 {
		b.tokens--
	} else {
		b.tokens = 0
	}
	return 1 - b.tokens/float64(rs.Max)
}

// slidingWindowEstimate advances the window(s) and returns the weighted
// estimate (previous window scaled by the elapsed fraction + current window).
// Callers hold rl.mu.
func slidingWindowEstimate(b *resourceBucket, perSec float64, now time.Time) (float64, time.Duration) {
	window := time.Duration(perSec * float64(time.Second))
	for !now.Before(b.windowStart.Add(window)) {
		b.prevCount = b.windowCount
		b.windowCount = 0
		b.windowStart = b.windowStart.Add(window)
	}
	elapsedFrac := now.Sub(b.windowStart).Seconds() / window.Seconds()
	return float64(b.prevCount)*(1-elapsedFrac) + float64(b.windowCount), window
}

func (rl *ResourceRateLimiter) allowSlidingWindow(b *resourceBucket, rs *spec.RateLimitSpec, perSec float64, now time.Time) bool {
	estimate, _ := slidingWindowEstimate(b, perSec, now)
	if estimate >= float64(rs.Max) {
		return false
	}
	b.windowCount++
	return true
}

// observeSlidingWindow counts the hit and reports utilization.
func (rl *ResourceRateLimiter) observeSlidingWindow(b *resourceBucket, rs *spec.RateLimitSpec, perSec float64, now time.Time) float64 {
	estimate, _ := slidingWindowEstimate(b, perSec, now)
	b.windowCount++
	return (estimate + 1) / float64(rs.Max)
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
	return f.checkRateLimitAction(w, r, es, nil, actionName)
}

// checkRateLimitAction is checkRateLimit for a caller that ALREADY holds the
// action's spec, resolved through the entity registry's union (`GetActionSpec`:
// declared `actions:` ∪ transition `via`).
//
// Why the override exists: `resolveAction` reads `es.Actions` only. A
// `rate_limit` declared on a transition `via` therefore never reached this
// check on the custom-action route, even though the handler was holding the
// very spec that carries it — the contract was declared and silently not
// enforced (kafe 10.60a). Passing the resolved action in closes that without
// touching `resolveAction`, which is shared with the create/update paths and
// deliberately still reads `actions:` alone (plan
// docs_internal/plan/via-sebagai-action-penuh.md §Further Considerations #1).
//
// Behaviour is unchanged for every pre-existing caller: when the override is
// nil — or carries no `rate_limit` of its own — resolution falls back to
// `resolveAction` and then to the resource default, exactly as before.
func (f *HandlerFactory) checkRateLimitAction(w http.ResponseWriter, r *http.Request, es *spec.EntitySpec, action *spec.Action, actionName string) bool {
	if f.rateLimiter == nil {
		return true
	}
	if es == nil {
		return true
	}

	// Per-action override wins over the resource-level default.
	rs := es.RateLimit
	switch {
	case action != nil && action.RateLimit != nil:
		rs = action.RateLimit
	default:
		if a := resolveAction(es, actionName); a != nil && a.RateLimit != nil {
			rs = a.RateLimit
		}
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
