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

// sagaTestHarness builds an integrator dispatcher with a real SQLite saga
// store and a recording executor.
type sagaTestHarness struct {
	disp     *Dispatcher
	rec      *recordingExecutor
	saga     *db.SagaStore
	entityReg *entity.Registry
}

func newSagaTestHarness(t *testing.T, it *spec.IntegratorSpec, targetActions []spec.Action) *sagaTestHarness {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "saga.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })
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