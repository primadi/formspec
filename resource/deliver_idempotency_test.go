package formspec

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// idempotencyStoreForTest opens a temp SQLite database and returns a store over
// it. The consequence path uses the real store (not a fake) because the whole
// point of 7.7.5 is that the SAME table serves both the HTTP and the delivery
// path, and a fake would not prove the two keys cannot collide.
func idempotencyStoreForTest(t *testing.T) *db.IdempotencyStore {
	t.Helper()
	d, err := db.OpenSQLite(filepath.Join(t.TempDir(), "deliver.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := db.NewMigrationRunner(d, db.DriverSQLite).EnsureSystemTables(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTables: %v", err)
	}
	return db.NewIdempotencyStore(d, db.DriverSQLite)
}

func deliveryTargetForTest() *spec.DeliveryTarget {
	return &spec.DeliveryTarget{Resource: "gl.gl-balance", Action: "update"}
}

// TestRunTargetOnce_RetryAfterSuccessDoesNotReRun is the 7.7.5 regression,
// stated as the measured failure: re-queuing the same event (exactly what a
// retry does) ran the consequence twice, accumulating `debit_movement`
// 143750 → 287500. With the key enforced, the second delivery is a no-op.
func TestRunTargetOnce_RetryAfterSuccessDoesNotReRun(t *testing.T) {
	store := idempotencyStoreForTest(t)
	ctx := context.Background()
	payload := map[string]any{"id": "jrn-1", "number": "JV-001"}

	calls := 0
	run := func() error { calls++; return nil }

	for attempt := 1; attempt <= 3; attempt++ {
		if err := runTargetOnce(ctx, store, "ws-1", deliveryTargetForTest(), "balance.{id}", payload, run); err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
	}
	if calls != 1 {
		t.Fatalf("consequence ran %d times, want exactly 1 — a retry must not re-apply the movement", calls)
	}
}

// TestRunTargetOnce_FailureIsRetryable keeps at-least-once delivery intact: a
// failed attempt must NOT be remembered as "done", or a transient target error
// would drop the consequence forever.
func TestRunTargetOnce_FailureIsRetryable(t *testing.T) {
	store := idempotencyStoreForTest(t)
	ctx := context.Background()
	payload := map[string]any{"id": "jrn-2"}

	boom := func() error { return errTestTarget }
	if err := runTargetOnce(ctx, store, "ws-1", deliveryTargetForTest(), "balance.{id}", payload, boom); err == nil {
		t.Fatal("a failing consequence must return its error so the outbox retries")
	}

	// The retry succeeds and IS allowed to run — the previous attempt wrote
	// nothing, so re-running is the recovery, not a duplicate.
	calls := 0
	if err := runTargetOnce(ctx, store, "ws-1", deliveryTargetForTest(), "balance.{id}", payload, func() error {
		calls++
		return nil
	}); err != nil {
		t.Fatalf("retry after failure must run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("retry calls = %d, want 1", calls)
	}
}

// TestRunTargetOnce_NoKeyDeclaredRunsEveryTime pins the boundary: an entry that
// declares no key keeps the previous behaviour. Natural idempotency is the
// target's contract there (7.7.3 already requires `idempotent: true`).
func TestRunTargetOnce_NoKeyDeclaredRunsEveryTime(t *testing.T) {
	calls := 0
	run := func() error { calls++; return nil }
	for i := 0; i < 2; i++ {
		if err := runTargetOnce(context.Background(), nil, "ws-1", deliveryTargetForTest(), "", nil, run); err != nil {
			t.Fatalf("no key declared must not need a store: %v", err)
		}
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 — without a declared key the guard must not interfere", calls)
	}
}

// TestRunTargetOnce_KeyWithoutStoreFailsLoudly pins the honest failure: a
// declared key the runtime cannot honour must not run unprotected. Running
// would be exactly the silent double write this closes.
func TestRunTargetOnce_KeyWithoutStoreFailsLoudly(t *testing.T) {
	calls := 0
	err := runTargetOnce(context.Background(), nil, "ws-1", deliveryTargetForTest(), "balance.{id}",
		map[string]any{"id": "jrn-3"}, func() error { calls++; return nil })
	if err == nil {
		t.Fatal("a declared key with no store must fail, not run unprotected")
	}
	if !strings.Contains(err.Error(), "no idempotency store") {
		t.Errorf("the error should say why, got %q", err)
	}
	if calls != 0 {
		t.Fatalf("the consequence ran %d times despite the refusal", calls)
	}
}

// TestRunTargetOnce_UnresolvedKeyTemplateFails is the trap a naive
// implementation falls into: leaving `balance.{id}` as a literal gives every
// event lacking `id` the SAME key, so the second one is skipped as "already
// delivered" — a LOST consequence, worse than the duplicate.
func TestRunTargetOnce_UnresolvedKeyTemplateFails(t *testing.T) {
	store := idempotencyStoreForTest(t)
	calls := 0
	err := runTargetOnce(context.Background(), store, "ws-1", deliveryTargetForTest(), "balance.{id}",
		map[string]any{"number": "JV-9"}, func() error { calls++; return nil })
	if err == nil {
		t.Fatal("an unresolved key template must fail rather than become a shared literal key")
	}
	if !strings.Contains(err.Error(), "balance.{id}") || !strings.Contains(err.Error(), `"id"`) {
		t.Errorf("the error should name the template and the missing path, got %q", err)
	}
	if calls != 0 {
		t.Fatalf("the consequence ran %d times with an unresolvable key", calls)
	}
}

// TestResolveDeliveryKey_NestedPathAndValues covers the template itself,
// including a nested path, so the key can identify a delivery precisely.
func TestResolveDeliveryKey_NestedPathAndValues(t *testing.T) {
	got, err := resolveDeliveryKey("journal-reversed.{source.id}", map[string]any{
		"source": map[string]any{"id": "abc"},
	})
	if err != nil {
		t.Fatalf("nested path: %v", err)
	}
	if got != "journal-reversed.abc" {
		t.Fatalf("nested key = %q, want journal-reversed.abc", got)
	}

	// A non-string scalar is rendered, not dropped.
	got, err = resolveDeliveryKey("balance.{id}", map[string]any{"id": 42})
	if err != nil {
		t.Fatalf("numeric id: %v", err)
	}
	if got != "balance.42" {
		t.Fatalf("numeric key = %q, want balance.42", got)
	}
}

// TestRunTargetOnce_ScopedPerTarget keeps two different consequences of the same
// event apart: a shared key must not let one suppress the other.
func TestRunTargetOnce_ScopedPerTarget(t *testing.T) {
	store := idempotencyStoreForTest(t)
	ctx := context.Background()
	payload := map[string]any{"id": "jrn-4"}

	other := &spec.DeliveryTarget{Resource: "gl.gl-balance", Action: "recalculate"}
	calls := 0
	run := func() error { calls++; return nil }

	if err := runTargetOnce(ctx, store, "ws-1", deliveryTargetForTest(), "balance.{id}", payload, run); err != nil {
		t.Fatalf("first target: %v", err)
	}
	if err := runTargetOnce(ctx, store, "ws-1", other, "balance.{id}", payload, run); err != nil {
		t.Fatalf("second target: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 — the key scope includes the target action", calls)
	}
}

// errTestTarget is a stand-in for a transient target failure.
var errTestTarget = &testError{"target unavailable"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
