package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := newRateLimiter(1, 2) // burst 2, refill 1/s
	if !rl.Allow("k") || !rl.Allow("k") {
		t.Fatal("expected first 2 requests allowed")
	}
	if rl.Allow("k") {
		t.Fatal("expected 3rd request blocked (burst exhausted)")
	}
	// Different key is unaffected.
	if !rl.Allow("other") {
		t.Fatal("expected other key allowed")
	}
}

func TestRateLimiter_Refills(t *testing.T) {
	rl := newRateLimiter(1000, 1) // fast refill, burst 1
	if !rl.Allow("k") {
		t.Fatal("expected first allowed")
	}
	if rl.Allow("k") {
		t.Fatal("expected second blocked immediately")
	}
	// After a short wait the bucket refills (rate 1000/s → ~10 tokens in 10ms).
	time.Sleep(10 * time.Millisecond)
	if !rl.Allow("k") {
		t.Fatal("expected allowed after refill")
	}
}

func TestLogin_RecordsAudit(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	// Successful login.
	body := bytes.NewBufferString(`{"username":"admin","password":"admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Failed login.
	body2 := bytes.NewBufferString(`{"username":"admin","password":"wrong"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/login", body2)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr2.Code)
	}

	// Audit log should contain both a success and a failure entry.
	entries := authAuditLog.recent(10)
	var sawSuccess, sawFailure bool
	for _, e := range entries {
		if e.Method == "login" && e.Result == "success" {
			sawSuccess = true
		}
		if e.Method == "login" && e.Result == "failure" {
			sawFailure = true
		}
	}
	if !sawSuccess {
		t.Error("expected a login success audit entry")
	}
	if !sawFailure {
		t.Error("expected a login failure audit entry")
	}
}
