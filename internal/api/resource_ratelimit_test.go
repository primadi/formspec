package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestResourceRateLimiter_TokenBucket verifies token-bucket strategy: burst
// allowed immediately, then refill-limited.
func TestResourceRateLimiter_TokenBucket(t *testing.T) {
	rl := NewResourceRateLimiter()
	rs := &spec.RateLimitSpec{Max: 3, Per: "1s", Strategy: "token_bucket"}

	// Burst of 3 allowed.
	for i := 0; i < 3; i++ {
		if !rl.Allow(rs, "k") {
			t.Fatalf("burst request %d should be allowed", i+1)
		}
	}
	// 4th blocked.
	if rl.Allow(rs, "k") {
		t.Fatal("4th request should be blocked (burst exhausted)")
	}
	// Different key unaffected.
	if !rl.Allow(rs, "other") {
		t.Fatal("different key should be allowed")
	}
}

// TestResourceRateLimiter_SlidingWindow verifies sliding-window strategy.
func TestResourceRateLimiter_SlidingWindow(t *testing.T) {
	rl := NewResourceRateLimiter()
	rs := &spec.RateLimitSpec{Max: 2, Per: "1s", Strategy: "sliding_window"}

	if !rl.Allow(rs, "k") || !rl.Allow(rs, "k") {
		t.Fatal("first two should be allowed")
	}
	if rl.Allow(rs, "k") {
		t.Fatal("third should be blocked")
	}
}

// TestResourceRateLimiter_NilSpec verifies nil spec → always allowed.
func TestResourceRateLimiter_NilSpec(t *testing.T) {
	rl := NewResourceRateLimiter()
	if !rl.Allow(nil, "k") {
		t.Fatal("nil spec should always allow")
	}
}

// TestParsePer verifies "per" duration parsing.
func TestParsePer(t *testing.T) {
	cases := map[string]float64{
		"1s": 1,
		"5s": 5,
		"1m": 60,
		"2h": 7200,
		"1d": 86400,
		"":   0,
		"x":  0,
	}
	for in, want := range cases {
		if got := parsePer(in); got != want {
			t.Errorf("parsePer(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestCheckRateLimit_429 verifies the handler-level gate writes 429 when the
// limit is exceeded and passes otherwise.
func TestCheckRateLimit_429(t *testing.T) {
	f := &HandlerFactory{rateLimiter: NewResourceRateLimiter()}
	es := &spec.EntitySpec{
		RateLimit: &spec.RateLimitSpec{Max: 1, Per: "1s", Strategy: "token_bucket"},
	}

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(WithWorkspace(req.Context(), "demo"))

	// First request allowed.
	w1 := httptest.NewRecorder()
	if !f.checkRateLimit(w1, req, es, "list") {
		t.Fatal("first request should be allowed")
	}
	// Second request → 429.
	w2 := httptest.NewRecorder()
	if f.checkRateLimit(w2, req, es, "list") {
		t.Fatal("second request should be blocked")
	}
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w2.Code)
	}
}

// TestCheckRateLimit_PerActionOverride verifies the per-action override wins.
func TestCheckRateLimit_PerActionOverride(t *testing.T) {
	f := &HandlerFactory{rateLimiter: NewResourceRateLimiter()}
	es := &spec.EntitySpec{
		RateLimit: &spec.RateLimitSpec{Max: 1, Per: "1s", Strategy: "token_bucket"},
		Actions: []spec.Action{
			{Name: "export", RateLimit: &spec.RateLimitSpec{Max: 10, Per: "1s", Strategy: "token_bucket"}},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(WithWorkspace(req.Context(), "demo"))

	// Resource-level "list" is exhausted after 1.
	f.checkRateLimit(httptest.NewRecorder(), req, es, "list")
	w := httptest.NewRecorder()
	if f.checkRateLimit(w, req, es, "list") {
		t.Fatal("list should be blocked after 1")
	}
	// Per-action "export" has its own budget (10) — still allowed.
	w2 := httptest.NewRecorder()
	if !f.checkRateLimit(w2, req, es, "export") {
		t.Fatal("export should be allowed (per-action override)")
	}
}
