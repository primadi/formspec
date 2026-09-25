package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/workflow"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// This file closes todo 7.4.7: the approval interception path
// (HandleCustomAction → RequiresApproval → handleWorkflowApproval) was verified
// only at the unit level (internal/workflow) — no test drove a real record
// through HTTP. The harness below wires the same dependencies production wires
// (workflow registry + approval store + a state machine) and exercises the flow
// end to end over net/http.

// approvalHarness is one ready-to-drive approval scenario.
type approvalHarness struct {
	factory   *HandlerFactory
	recordID  string
	requester string // owner of the seeded record (cannot approve own request)
}

// postTransition issues a custom action over HTTP, resolving the record through
// the {id} path value — the production route shape.
func (h *approvalHarness) postTransition(t *testing.T, action, actor string, body map[string]any, role string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = strings.NewReader(string(raw))
	}

	req := httptest.NewRequest("POST", "/billing/orders/"+h.recordID+"/"+action, reader)
	req.SetPathValue("id", h.recordID)
	ctx := WithWorkspace(req.Context(), "t1")
	ctx = WithUser(ctx, actor)
	if role != "" {
		ctx = WithIdentity(ctx, &auth.Identity{
			UserID:      actor,
			WorkspaceID: "t1",
			Roles:       []string{role},
			Permissions: []string{"*"},
		})
	}
	req = req.WithContext(ctx)

	actionSpec := spec.Action{
		Name: action,
		Impl: &spec.ImplDecl{Type: spec.ImplNative},
	}
	rr := httptest.NewRecorder()
	h.factory.HandleCustomAction("billing", "order", action, actionSpec, "")(rr, req)
	return rr
}

// setupApprovalHarness registers a stateful "billing/order" entity with a
// draft→posted transition, attaches a one-step workflow that intercepts
// `void-order` (requiring role "supervisor"), and seeds one record owned by
// "owner-1".
func setupApprovalHarness(t *testing.T) *approvalHarness {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "approval_api.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)

	orderSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields: []spec.Field{
			{Name: "status", Type: spec.FieldString},
			{Name: "void_reason", Type: spec.FieldString},
		},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			States: []spec.StateDecl{
				{Name: "draft"}, {Name: "posted"}, {Name: "voided"},
			},
			Transitions: []spec.TransitionDecl{{From: spec.StateList{"posted"}, To: "voided", Action: "void-order"}},
		},
	}
	registerTestEntity(t, d, reg, "billing", "order", orderSpec)

	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	recordID, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "owner-1",
		Data:        map[string]any{"status": "posted", "void_reason": "customer complaint"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	// Workflow registry: one workflow intercepting billing.order's void-order.
	wfReg := workflow.NewRegistry()
	wfReg.Add("billing", "void-approval", &spec.WorkflowSpec{
		Entity: "billing.order",
		On: &spec.WorkflowTrigger{
			Transition: &spec.WorkflowTransitionRef{Name: "void-order"},
		},
		Steps: []spec.WorkflowStep{
			{Roles: []string{"supervisor"}, Title: "Supervisor approval"},
		},
		OnReject: &spec.WorkflowReject{To: "posted"},
	})

	factory := NewHandlerFactory(reg)
	factory.SetWorkflowRegistry(wfReg)
	factory.SetWorkflowApprovalStore(db.NewWorkflowApprovalStore(d, db.DriverSQLite))
	// Production wires this from the entity registry; without it the handler
	// cannot resolve the state machine and never reaches interception.
	factory.SetSpecLookup(func(m, n string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(m, n)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})
	// The transition itself is applied by the workflow/state-machine layer, so
	// the action's own impl can be a no-op — but the dispatcher still needs an
	// executor registered for the impl type it is handed.
	factory.dispatcher.RegisterExecutor(spec.ImplNative, noopExecutor{})

	return &approvalHarness{factory: factory, recordID: recordID, requester: "owner-1"}
}

// TestWorkflowApproval_OverHTTP_AcceptThenExecute walks the whole interception
// path over HTTP: the transition is intercepted (202, record unchanged), an
// approval from a non-requester holding the step role is accepted, and only
// then does the record actually move to the target state.
func TestWorkflowApproval_OverHTTP_AcceptThenExecute(t *testing.T) {
	h := setupApprovalHarness(t)

	// 1. Requesting the transition starts the approval flow — 202, not applied.
	rr := h.postTransition(t, "void-order", "clerk-1", nil, "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("start approval: expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	var started struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &started); err != nil {
		t.Fatalf("unmarshal 202 body: %v", err)
	}
	if started.Data["status"] != "approval_required" {
		t.Errorf("expected status=approval_required, got %v", started.Data["status"])
	}
	if got := readOrderStatus(t, h); got != "posted" {
		t.Fatalf("record must NOT transition while approval is pending: status=%q", got)
	}

	// 2. Someone without the step role cannot approve.
	rr = h.postTransition(t, "void-order", "clerk-2",
		map[string]any{"decision": "approve"}, "cashier")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("wrong role approve: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. The requester cannot approve their own request.
	rr = h.postTransition(t, "void-order", h.requester,
		map[string]any{"decision": "approve"}, "supervisor")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("self-approve: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}

	// 4. A role-qualified non-requester approves → transition executes.
	rr = h.postTransition(t, "void-order", "supervisor-1",
		map[string]any{"decision": "approve"}, "supervisor")
	if rr.Code != http.StatusOK {
		t.Fatalf("approve: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := readOrderStatus(t, h); got != "voided" {
		t.Fatalf("after approval the record must be in the target state: status=%q, want voided", got)
	}
}

// TestWorkflowApproval_OverHTTP_Reject covers the other branch: a rejection is
// recorded and the record moves to the workflow's on_reject target (here the
// unreachable-but-explicit "posted" — i.e. it stays put), never the transition
// target.
func TestWorkflowApproval_OverHTTP_Reject(t *testing.T) {
	h := setupApprovalHarness(t)

	if rr := h.postTransition(t, "void-order", "clerk-1", nil, ""); rr.Code != http.StatusAccepted {
		t.Fatalf("start approval: expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	rr := h.postTransition(t, "void-order", "supervisor-1",
		map[string]any{"decision": "reject"}, "supervisor")
	if rr.Code != http.StatusOK {
		t.Fatalf("reject: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal reject body: %v", err)
	}
	if body.Data["status"] != "rejected" {
		t.Errorf("expected status=rejected, got %v", body.Data["status"])
	}
	if got := readOrderStatus(t, h); got != "posted" {
		t.Errorf("rejected transition must not reach the target state: status=%q, want posted", got)
	}
}

// TestWorkflowApproval_OverHTTP_PendingWithoutDecision guards the contract a
// client hits by retrying the same transition without a decision: a pending
// approval exists, so the call is a 422 telling the caller to decide — it must
// not silently re-create the request or execute the transition.
func TestWorkflowApproval_OverHTTP_PendingWithoutDecision(t *testing.T) {
	h := setupApprovalHarness(t)

	if rr := h.postTransition(t, "void-order", "clerk-1", nil, ""); rr.Code != http.StatusAccepted {
		t.Fatalf("start approval: expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	rr := h.postTransition(t, "void-order", "clerk-1", nil, "")
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("pending without decision: expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := readOrderStatus(t, h); got != "posted" {
		t.Errorf("record must remain untouched: status=%q", got)
	}
}

// TestWorkflowApproval_NoWorkflowTransitionsDirectly is the control case: with
// no intercepting workflow the same action reaches dispatch immediately (200)
// instead of entering the approval flow (202), proving the 202s above come from
// interception rather than from the action being inert.
//
// Note it does NOT assert the record's status becomes "voided": applying a
// state-machine transition is the business action's own job (its executor's
// handler), not the workflow layer's, so with a no-op executor the state stays
// put. Asserting otherwise would encode a wrong mental model of where the
// transition is applied.
func TestWorkflowApproval_NoWorkflowTransitionsDirectly(t *testing.T) {
	h := setupApprovalHarness(t)
	// Replace the registry with an empty one.
	h.factory.SetWorkflowRegistry(workflow.NewRegistry())

	rr := h.postTransition(t, "void-order", "clerk-1", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("unintercepted transition: expected 200 (dispatched), got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Code == http.StatusAccepted {
		t.Fatal("unintercepted transition must not enter the approval flow")
	}
}

// readOrderStatus reads the record's status straight from the store, so the
// assertions above observe persisted state rather than the response body.
func readOrderStatus(t *testing.T, h *approvalHarness) string {
	t.Helper()
	provider, ok := h.factory.registry.(*entity.Registry)
	if !ok {
		t.Fatalf("registry is %T, want *entity.Registry", h.factory.registry)
	}
	store, err := provider.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	rec, err := store.GetByID(t.Context(), db.GetByIDParams{WorkspaceID: "t1", ID: h.recordID})
	if err != nil {
		t.Fatalf("GetByID(%s): %v", h.recordID, err)
	}
	status, _ := rec.Data["status"].(string)
	return status
}

// noopExecutor satisfies action.Executor without doing anything — the approval
// interception tests exercise the workflow layer, not business logic.
type noopExecutor struct{}

func (noopExecutor) Execute(context.Context, spec.Action, action.ExecuteParams) (*action.ExecuteResult, error) {
	return &action.ExecuteResult{}, nil
}

// TestWorkflowApproval_RequesterCannotSelfApprove_Regression pins the specific
// defect the harness in this file uncovered (todo 7.4.7): created_by lives on
// the record's framework columns, not in the Data map, so reading it from
// resourceData left RequesterID empty and 7.4.5 never fired. Before the fix the
// requester's own "approve" returned 200 and completed the transition.
func TestWorkflowApproval_RequesterCannotSelfApprove_Regression(t *testing.T) {
	h := setupApprovalHarness(t)

	if rr := h.postTransition(t, "void-order", "clerk-1", nil, ""); rr.Code != http.StatusAccepted {
		t.Fatalf("start approval: expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	// h.requester owns the seeded record AND holds the required role, so the
	// only thing that can reject this is the self-approval rule.
	rr := h.postTransition(t, "void-order", h.requester,
		map[string]any{"decision": "approve"}, "supervisor")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("requester self-approval must be 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := readOrderStatus(t, h); got != "posted" {
		t.Errorf("record must be untouched after a denied self-approval: status=%q", got)
	}
}
