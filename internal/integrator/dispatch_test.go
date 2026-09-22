package integrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/service"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// TestApplyCallMap pins S6 (item 6.2): an integrator's `map:` builds the target
// action's params from the source event payload, interpolating `{dotted.path}`
// templates. A value that is exactly one token keeps the resolved value's type
// (so a money object stays an object); nested maps/lists interpolate
// recursively.
func TestApplyCallMap(t *testing.T) {
	payload := map[string]any{
		"number":       "ORD-1",
		"subtotal":     map[string]any{"amount": "50000", "currency": "IDR"},
		"tax_amount":   map[string]any{"amount": "5000", "currency": "IDR"},
		"total_amount": map[string]any{"amount": "55000", "currency": "IDR"},
		"paid_at":      "2026-09-20T10:00:00Z",
	}

	m := map[string]any{
		"entry_date": "{paid_at}",
		"memo":       "Penjualan {number}",
		"lines": []any{
			map[string]any{"account": "1-1000", "debit": "{total_amount}"},
			map[string]any{"account": "4-1000", "credit": "{subtotal}"},
			map[string]any{"account": "2-2000", "credit": "{tax_amount}"},
		},
	}

	got := applyCallMap(m, payload)

	// Whole-token value keeps its type: the money object, not its string form.
	if debit, ok := got["lines"].([]any)[0].(map[string]any)["debit"].(map[string]any); !ok || debit["amount"] != "55000" {
		t.Errorf("debit = %#v, want the money object {amount:55000}", got["lines"].([]any)[0].(map[string]any)["debit"])
	}
	// Interpolated string.
	if got["memo"] != "Penjualan ORD-1" {
		t.Errorf("memo = %v, want \"Penjualan ORD-1\"", got["memo"])
	}
	if got["entry_date"] != "2026-09-20T10:00:00Z" {
		t.Errorf("entry_date = %v, want the paid_at value", got["entry_date"])
	}
	// Nested list of maps interpolated.
	lines := got["lines"].([]any)
	if len(lines) != 3 {
		t.Fatalf("lines = %d entries, want 3", len(lines))
	}
	if lines[1].(map[string]any)["account"] != "4-1000" {
		t.Errorf("lines[1].account = %v, want 4-1000", lines[1].(map[string]any)["account"])
	}
}

// TestApplyCallMap_UnresolvedTokenLeftVerbatim pins that a typo is visible in
// the payload rather than silently becoming an empty string.
func TestApplyCallMap_UnresolvedTokenLeftVerbatim(t *testing.T) {
	got := applyCallMap(map[string]any{"memo": "x {nope}"}, map[string]any{"number": "ORD-1"})
	if got["memo"] != "x {nope}" {
		t.Errorf("memo = %v, want the token left verbatim", got["memo"])
	}
}

// sagaTestHarness builds an integrator dispatcher with a real SQLite saga
// store and a recording executor.
type sagaTestHarness struct {
	disp      *Dispatcher
	rec       *recordingExecutor
	saga      *db.SagaStore
	entityReg *entity.Registry
}

func newSagaTestHarness(t *testing.T, it *spec.IntegratorSpec, targetActions []spec.Action) *sagaTestHarness {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "saga.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	r := db.NewMigrationRunner(d, db.DriverSQLite)
	if err := r.EnsureSystemTables(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTables: %v", err)
	}

	reg := NewRegistry()
	reg.Add("billing", "it", it)

	entityReg := entity.NewRegistry(d, db.DriverSQLite, dir)
	if err := entityReg.RegisterCoreEntity("gl", "journal-entry", "test", &spec.EntitySpec{
		Actions: targetActions,
	}); err != nil {
		t.Fatalf("RegisterCoreEntity: %v", err)
	}

	svcReg := service.NewRegistry()
	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	saga := db.NewSagaStore(d, db.DriverSQLite)
	return &sagaTestHarness{
		disp:      NewDispatcher(reg, entityReg, svcReg, disp, saga),
		rec:       rec,
		saga:      saga,
		entityReg: entityReg,
	}
}

func TestSaga_RegisterAndComplete(t *testing.T) {
	it := &spec.IntegratorSpec{
		Listen:     &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:       &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "create"},
		Compensate: "recreate",
	}
	h := newSagaTestHarness(t, it, []spec.Action{
		{Name: "create", Impl: &spec.ImplDecl{Type: spec.ImplNative}},
		{Name: "recreate", Impl: &spec.ImplDecl{Type: spec.ImplNative}},
	})

	err := h.disp.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", map[string]any{"id": "INV-1"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// The target action ran.
	if len(h.rec.calls) != 1 {
		t.Fatalf("target action called %d times, want 1", len(h.rec.calls))
	}

	// The saga entry should be completed (target succeeded).
	pending, err := h.saga.ListPending(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending saga entries after success, got %d", len(pending))
	}
}

func TestSaga_CompensateOnFailure(t *testing.T) {
	it := &spec.IntegratorSpec{
		Listen:     &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:       &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "create"},
		Compensate: "recreate",
	}
	// The "create" action is NOT registered → dispatch fails → compensate runs.
	h := newSagaTestHarness(t, it, []spec.Action{
		{Name: "recreate", Impl: &spec.ImplDecl{Type: spec.ImplNative}},
	})

	err := h.disp.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", map[string]any{"id": "INV-1"})
	if err == nil {
		t.Fatal("expected dispatch to fail when target action is missing")
	}

	// The compensate action should have run (recreate).
	if len(h.rec.calls) != 1 {
		t.Fatalf("compensate action called %d times, want 1", len(h.rec.calls))
	}
	if h.rec.calls[0].ActionName != "recreate" {
		t.Errorf("compensate action: want recreate, got %q", h.rec.calls[0].ActionName)
	}

	// The saga entry should be compensated.
	pending, err := h.saga.ListPending(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending saga entries after compensation, got %d", len(pending))
	}
}

func TestSaga_NoCompensateDeclared(t *testing.T) {
	it := &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "create"},
		// No Compensate declared.
	}
	h := newSagaTestHarness(t, it, []spec.Action{
		{Name: "create", Impl: &spec.ImplDecl{Type: spec.ImplNative}},
	})

	err := h.disp.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", map[string]any{"id": "INV-1"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// No saga entry should be registered (no compensate declared).
	pending, err := h.saga.ListPending(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no saga entries without compensate, got %d", len(pending))
	}
}
