package integrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// newTestEntityRegistry builds an entity registry backed by an in-memory
// SQLite database with a single registered entity.
func newTestEntityRegistry(t *testing.T, module, name string, es *spec.EntitySpec) *entity.Registry {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	r := db.NewMigrationRunner(d, db.DriverSQLite)
	if err := r.EnsureSystemTables(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTables: %v", err)
	}

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	if err := reg.RegisterCoreEntity(module, name, "test", es); err != nil {
		t.Fatalf("RegisterCoreEntity: %v", err)
	}
	return reg
}

func TestRegistry_AddGetList(t *testing.T) {
	reg := NewRegistry()
	it := &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "create"},
	}
	reg.Add("billing", "invoice-to-gl", it)

	got, ok := reg.Get("billing", "invoice-to-gl")
	if !ok {
		t.Fatal("expected integrator to be found")
	}
	if got.Call.Resource != "gl.journal-entry" || got.Call.Action != "create" {
		t.Errorf("call: want gl.journal-entry.create, got %s.%s", got.Call.Resource, got.Call.Action)
	}

	infos := reg.List()
	if len(infos) != 1 {
		t.Fatalf("List: want 1, got %d", len(infos))
	}
	if infos[0].Name != "invoice-to-gl" || infos[0].Listen != "billing.invoice.on_submit" {
		t.Errorf("List[0]: want billing/invoice-to-gl listen billing.invoice.on_submit, got %s/%s %s",
			infos[0].Module, infos[0].Name, infos[0].Listen)
	}
}

func TestRegistry_ForEvent(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "it1", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "create"},
	})
	reg.Add("billing", "it2", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "cancel"},
	})
	reg.Add("billing", "it3", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_cancel"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "cancel"},
	})

	if len(reg.ForEvent("billing.invoice.on_submit")) != 2 {
		t.Fatal("ForEvent(on_submit): want 2")
	}
	if len(reg.ForEvent("billing.invoice.on_cancel")) != 1 {
		t.Fatal("ForEvent(on_cancel): want 1")
	}
	if len(reg.ForEvent("billing.invoice.nonexistent")) != 0 {
		t.Fatal("ForEvent(nonexistent): want 0")
	}
}

func TestRegistry_ReAddReplacesIndex(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "it", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "create"},
	})
	// Re-register with a different event — old index must be removed.
	reg.Add("billing", "it", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_cancel"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "cancel"},
	})

	if len(reg.ForEvent("billing.invoice.on_submit")) != 0 {
		t.Fatal("old event index should be removed on re-registration")
	}
	if len(reg.ForEvent("billing.invoice.on_cancel")) != 1 {
		t.Fatal("new event index should be present after re-registration")
	}
}

// recordingExecutor records the params it was dispatched with, so the test
// can assert the integrator dispatch path.
type recordingExecutor struct {
	calls []action.ExecuteParams
}

func (e *recordingExecutor) Execute(_ context.Context, _ spec.Action, params action.ExecuteParams) (*action.ExecuteResult, error) {
	e.calls = append(e.calls, params)
	return &action.ExecuteResult{Data: map[string]any{"ok": true}}, nil
}

func TestDispatcher_Dispatch(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "invoice-to-gl", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "create"},
	})

	entityReg := newTestEntityRegistry(t, "gl", "journal-entry", &spec.EntitySpec{
		Actions: []spec.Action{{Name: "create", Impl: &spec.ImplDecl{Type: spec.ImplNative}}},
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	d := NewDispatcher(reg, entityReg, nil, disp)
	err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", map[string]any{"id": "INV-1"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if len(rec.calls) != 1 {
		t.Fatalf("target action called %d times, want 1", len(rec.calls))
	}
	call := rec.calls[0]
	if call.Module != "gl" || call.Entity != "journal-entry" || call.ActionName != "create" {
		t.Errorf("dispatch target: want gl/journal-entry/create, got %s/%s/%s", call.Module, call.Entity, call.ActionName)
	}
	if call.WorkspaceID != "ws-1" {
		t.Errorf("workspace: want ws-1, got %q", call.WorkspaceID)
	}
	// The event payload is merged with the reserved _event metadata.
	if call.Params["id"] != "INV-1" {
		t.Errorf("payload id: want INV-1, got %v", call.Params["id"])
	}
	ev, ok := call.Params["_event"].(map[string]any)
	if !ok {
		t.Fatalf("_event metadata missing: %#v", call.Params["_event"])
	}
	if ev["name"] != "billing.invoice.on_submit" || ev["resource"] != "billing/invoice" {
		t.Errorf("_event metadata: want name+resource, got %#v", ev)
	}
}

func TestDispatcher_NoMatchingIntegrator(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "it", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "create"},
	})
	entityReg := entity.NewRegistry(nil, db.DriverSQLite, "")
	disp := action.NewDispatcher()

	d := NewDispatcher(reg, entityReg, nil, disp)
	// No integrator matches this event → no error, no dispatch.
	if err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_cancel", "billing/invoice", nil); err != nil {
		t.Fatalf("Dispatch for unmatched event: %v", err)
	}
}

func TestDispatcher_TargetActionNotFound(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "it", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.missing", Action: "create"},
	})
	entityReg := entity.NewRegistry(nil, db.DriverSQLite, "")
	disp := action.NewDispatcher()

	d := NewDispatcher(reg, entityReg, nil, disp)
	err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", nil)
	if err == nil {
		t.Fatal("expected error when target action is missing")
	}
}

func TestDispatcher_OneFailureDoesNotStopOthers(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "good", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.journal-entry", Action: "create"},
	})
	reg.Add("billing", "bad", &spec.IntegratorSpec{
		Listen: &spec.IntegratorListen{Resource: "billing.invoice", Event: "on_submit"},
		Call:   &spec.IntegratorCall{Resource: "gl.missing", Action: "create"},
	})

	entityReg := newTestEntityRegistry(t, "gl", "journal-entry", &spec.EntitySpec{
		Actions: []spec.Action{{Name: "create", Impl: &spec.ImplDecl{Type: spec.ImplNative}}},
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	d := NewDispatcher(reg, entityReg, nil, disp)
	err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", nil)
	if err == nil {
		t.Fatal("expected aggregated error when one integrator fails")
	}
	// The good integrator still ran despite the bad one failing.
	if len(rec.calls) != 1 {
		t.Fatalf("good integrator called %d times, want 1", len(rec.calls))
	}
}