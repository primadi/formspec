package observability

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// TestHealth_Vocabulary proves the status derivation and controlled reason
// vocabulary (spec §5).
func TestHealth_Vocabulary(t *testing.T) {
	h := NewHealth()

	// No probes → healthy with empty reasons.
	rep := h.Check()
	if rep.Status != StatusHealthy || len(rep.Reasons) != 0 {
		t.Fatalf("empty registry: %+v, want healthy/[]", rep)
	}
	if rep.CheckedAt == "" {
		t.Fatal("checked_at empty")
	}

	// Degraded probe → degraded.
	h.Register(ReasonOutboxBacklog, func() (string, bool) { return ReasonOutboxBacklog, false })
	rep = h.Check()
	if rep.Status != StatusDegraded {
		t.Fatalf("status = %q, want degraded", rep.Status)
	}
	if len(rep.Reasons) != 1 || rep.Reasons[0] != ReasonOutboxBacklog {
		t.Fatalf("reasons = %v, want [outbox_backlog]", rep.Reasons)
	}

	// Hard-failing probe → unhealthy, wins over degraded.
	h.Register(ReasonDatastoreUnreachable, func() (string, bool) { return ReasonDatastoreUnreachable, true })
	rep = h.Check()
	if rep.Status != StatusUnhealthy {
		t.Fatalf("status = %q, want unhealthy", rep.Status)
	}
	if len(rep.Reasons) != 2 {
		t.Fatalf("reasons = %v, want 2 entries", rep.Reasons)
	}

	// Recovering the hard failure returns to degraded.
	h.Register(ReasonDatastoreUnreachable, func() (string, bool) { return "", false })
	if rep := h.Check(); rep.Status != StatusDegraded {
		t.Fatalf("status = %q, want degraded after recovery", rep.Status)
	}
}

// TestHealth_Handler proves the /health response shape
// {status, reasons, checked_at} and the HTTP contract: 200 for
// healthy/degraded, 503 for unhealthy (readiness gate).
func TestHealth_Handler(t *testing.T) {
	h := NewHealth()
	h.Register(ReasonDatastoreUnreachable, func() (string, bool) { return ReasonDatastoreUnreachable, true })

	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != 503 {
		t.Errorf("unhealthy status = %d, want 503", rec.Code)
	}
	var body Report
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the contract shape: %v", err)
	}
	if body.Status != StatusUnhealthy {
		t.Errorf("status = %q, want unhealthy", body.Status)
	}

	// Healthy → 200.
	h.Register(ReasonDatastoreUnreachable, func() (string, bool) { return "", false })
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != 200 {
		t.Errorf("healthy status = %d, want 200", rec.Code)
	}
}

// TestRequestID proves context round-trip and generation.
func TestRequestID(t *testing.T) {
	ctx := WithRequestID(t.Context(), "req-abc")
	if got := RequestIDFromContext(ctx); got != "req-abc" {
		t.Errorf("RequestIDFromContext = %q, want req-abc", got)
	}
	if got := RequestIDFromContext(t.Context()); got != "" {
		t.Errorf("empty context request id = %q, want \"\"", got)
	}
	id := NewRequestID()
	if len(id) != 16 {
		t.Errorf("NewRequestID len = %d, want 16 hex chars", len(id))
	}
}
