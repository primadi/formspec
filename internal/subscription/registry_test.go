package subscription

import (
	"context"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/pkg/spec"
)

func TestRegistry_AddGetList(t *testing.T) {
	reg := NewRegistry()
	sub := &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "billing/audit-log"},
	}
	reg.Add("billing", "invoice-audit", sub)

	got, ok := reg.Get("billing", "invoice-audit")
	if !ok {
		t.Fatal("expected subscription to be found")
	}
	if got.Handler.Ref != "billing/audit-log" {
		t.Errorf("handler.ref: want billing/audit-log, got %q", got.Handler.Ref)
	}

	infos := reg.List()
	if len(infos) != 1 {
		t.Fatalf("List: want 1, got %d", len(infos))
	}
	if infos[0].Name != "invoice-audit" || infos[0].Module != "billing" {
		t.Errorf("List[0]: want billing/invoice-audit, got %s/%s", infos[0].Module, infos[0].Name)
	}
}

func TestRegistry_ForEvent(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Ref: "billing/audit-log"},
	})
	reg.Add("billing", "notify", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Ref: "billing/notify"},
	})
	reg.Add("billing", "other", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_cancel"},
		Handler: spec.ImplDecl{Ref: "billing/other"},
	})

	subs := reg.ForEvent("billing.invoice.on_submit")
	if len(subs) != 2 {
		t.Fatalf("ForEvent(on_submit): want 2, got %d", len(subs))
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
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Ref: "billing/audit-log"},
	})
	// Re-register the same key with a different event — the old event index
	// must be removed (hot-reload semantics).
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_cancel"},
		Handler: spec.ImplDecl{Ref: "billing/audit-log"},
	})

	if len(reg.ForEvent("billing.invoice.on_submit")) != 0 {
		t.Fatal("old event index should be removed on re-registration")
	}
	if len(reg.ForEvent("billing.invoice.on_cancel")) != 1 {
		t.Fatal("new event index should be present after re-registration")
	}
}

// recordingExecutor records the params (and the dispatched action) it was
// called with, so the test can assert the subscription dispatch path.
type recordingExecutor struct {
	calls   []action.ExecuteParams
	actions []spec.Action
}

func (e *recordingExecutor) Execute(_ context.Context, act spec.Action, params action.ExecuteParams) (*action.ExecuteResult, error) {
	e.calls = append(e.calls, params)
	e.actions = append(e.actions, act)
	return &action.ExecuteResult{Data: map[string]any{"ok": true}}, nil
}

func TestDispatcher_Dispatch(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.audit-log"},
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	d := NewDispatcher(reg, disp)
	err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", map[string]any{"id": "INV-1"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if len(rec.calls) != 1 {
		t.Fatalf("handler called %d times, want 1", len(rec.calls))
	}
	call := rec.calls[0]
	if call.ActionName != "handle" {
		t.Errorf("action name: want handle, got %q", call.ActionName)
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

// A subscription's declared `uses` must reach the dispatched action, or a
// handler that needs `ctx.db` fails USES_VIOLATION in ProdMode/StrictMode with
// an instruction the manifest could not comply with — the handler had no place
// to declare it (kafe 10.56).
func TestDispatcher_CarriesUsesToTheHandler(t *testing.T) {
	reg := NewRegistry()
	uses := &spec.UsesDecl{Primitives: []string{"db"}}
	reg.Add("billing", "project", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.project-summary"},
		Uses:    uses,
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	d := NewDispatcher(reg, disp)
	if err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", nil); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(rec.actions) != 1 {
		t.Fatalf("handler called %d times, want 1", len(rec.actions))
	}
	// `uses` rides on the ACTION, which is where the executor reads it from.
	if rec.actions[0].Uses == nil {
		t.Fatal("the handler must receive the subscription's declared `uses`, got nil")
	}
	if got := rec.actions[0].Uses.Primitives; len(got) != 1 || got[0] != "db" {
		t.Errorf("uses.primitives: want [db], got %v", got)
	}
}

func TestDispatcher_NoMatchingSubscription(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Ref: "billing/audit-log"},
	})
	disp := action.NewDispatcher()

	d := NewDispatcher(reg, disp)
	// No subscription matches this event → no error, no dispatch.
	if err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_cancel", "billing/invoice", nil); err != nil {
		t.Fatalf("Dispatch for unmatched event: %v", err)
	}
}

func TestDispatcher_NoImplType(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Ref: "billing/audit-log"}, // no Type
	})
	disp := action.NewDispatcher()

	d := NewDispatcher(reg, disp)
	err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", nil)
	if err == nil {
		t.Fatal("expected error when handler has no impl type")
	}
}

func TestDispatcher_OneFailureDoesNotStopOthers(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "good", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.good"},
	})
	reg.Add("billing", "bad", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Ref: "billing.missing"}, // no Type → fails
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	d := NewDispatcher(reg, disp)
	err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", nil)
	if err == nil {
		t.Fatal("expected aggregated error when one handler fails")
	}
	// The good handler still ran despite the bad one failing.
	if len(rec.calls) != 1 {
		t.Fatalf("good handler called %d times, want 1", len(rec.calls))
	}
}
