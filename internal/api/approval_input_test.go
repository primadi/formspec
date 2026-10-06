package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/approval"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// This file closes the reporting half of the action-input contract plan
// (docs_internal/plan/action-input-contract.md, Fase 4 / D5).
//
// The reported defect: an approval-gated transition returns 202 BEFORE any
// write, and `handleApproval` read only the `decision` verb — so the
// inputs the requester collected were dropped. An approval-gated `void-order`
// with a `void_reason` input ended as a voided record with no reason, and a
// transition guarding on that value failed a check nobody could satisfy.
//
// The fixture is exactly that shape: a transition behind a one-step workflow
// whose declared input is required.

// approvalInputHarness drives an approval-gated transition that declares an
// input contract.
type approvalInputHarness struct {
	factory  *HandlerFactory
	recordID string
}

func setupApprovalInputHarness(t *testing.T) *approvalInputHarness {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "approval_input.db"), nil)
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
			Initial: "posted",
			States:  []spec.StateDecl{{Name: "posted"}, {Name: "voided"}},
			Transitions: []spec.TransitionDecl{{
				From: spec.StateList{"posted"}, To: "voided", Action: "void-order",
				Params: &spec.ParamsDecl{
					Inputs: []spec.ParamInput{{Name: "void_reason", Required: true}},
				},
				Approval: &spec.ApprovalSpec{
					Steps:    []spec.ApprovalStep{{Roles: []string{"supervisor"}}},
					OnReject: &spec.ApprovalReject{To: "posted"},
				},
			}},
		},
	}
	registerTestEntity(t, d, reg, "billing", "order", orderSpec)

	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	recordID, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "t1", CreatedBy: "clerk-1",
		// Seed a record that is already past the initial state. That is a
		// system-shaped write (it reproduces a stored row), so it declares
		// itself instead of relying on the rule not existing (kafe 10.72).
		SystemCaller: true,
		Data:         map[string]any{"status": "posted"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	wfReg := approval.NewRegistry()
	wfReg.AddEntity("billing", "order", &orderSpec)

	factory := NewHandlerFactory(reg)
	factory.SetApprovalRegistry(wfReg)
	factory.SetApprovalRequestStore(db.NewApprovalRequestStore(d, db.DriverSQLite))
	factory.SetSpecLookup(func(m, n string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(m, n)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})
	factory.dispatcher.RegisterExecutor(spec.ImplNative, noopExecutor{})

	return &approvalInputHarness{factory: factory, recordID: recordID}
}

// patch drives the transition the way the derived UI does for a transition with
// no `impl`: PATCH with the target state plus the collected inputs.
func (h *approvalInputHarness) patch(t *testing.T, actor, role string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest("PATCH", "/billing/orders/"+h.recordID, strings.NewReader(string(raw)))
	req.SetPathValue("id", h.recordID)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), actor))
	if role != "" {
		req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
			UserID: actor, WorkspaceID: "t1",
			Roles: []string{role}, Permissions: []string{"*"},
		}))
	}
	rr := httptest.NewRecorder()
	h.factory.HandleUpdate("billing", "order")(rr, req)
	return rr
}

func (h *approvalInputHarness) record(t *testing.T) (status, reason string) {
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
		t.Fatalf("GetByID: %v", err)
	}
	status, _ = rec.Data["status"].(string)
	reason, _ = rec.Data["void_reason"].(string)
	return status, reason
}

// TestApprovalInput_PersistedThroughApproval is the regression: the requester's
// declared input must reach the record once approval completes, even though the
// approving call never mentions it.
func TestApprovalInput_PersistedThroughApproval(t *testing.T) {
	h := setupApprovalInputHarness(t)

	// 1. The requester supplies the reason; the call starts approval and writes
	//    nothing.
	rr := h.patch(t, "clerk-1", "", map[string]any{
		"status": "voided", "void_reason": "customer complaint",
	})
	if rr.Code != http.StatusAccepted {
		t.Fatalf("start approval = %d, want 202; body: %s", rr.Code, rr.Body.String())
	}
	if status, reason := h.record(t); status != "posted" || reason != "" {
		t.Fatalf("202 must write nothing: status=%q reason=%q", status, reason)
	}

	// 2. The approver approves WITHOUT repeating the reason — the whole point,
	//    since the approval call is a different request from a different person.
	rr = h.patch(t, "supervisor-1", "supervisor", map[string]any{
		"status": "voided", "decision": "approve",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("approve = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	status, reason := h.record(t)
	if status != "voided" {
		t.Fatalf("state = %q, want voided", status)
	}
	if reason != "customer complaint" {
		t.Errorf("void_reason = %q, want the requester's stored value — an input "+
			"collected before an approval must survive it, or the value the "+
			"requester supplied is unreachable by anyone", reason)
	}
}

// TestApprovalInput_ApproverValueWins lets the approver correct the input: their
// explicit value is not overwritten by the stored one.
func TestApprovalInput_ApproverValueWins(t *testing.T) {
	h := setupApprovalInputHarness(t)

	if rr := h.patch(t, "clerk-1", "", map[string]any{
		"status": "voided", "void_reason": "original",
	}); rr.Code != http.StatusAccepted {
		t.Fatalf("start approval = %d, want 202: %s", rr.Code, rr.Body.String())
	}

	if rr := h.patch(t, "supervisor-1", "supervisor", map[string]any{
		"status": "voided", "decision": "approve", "void_reason": "corrected by supervisor",
	}); rr.Code != http.StatusOK {
		t.Fatalf("approve = %d, want 200: %s", rr.Code, rr.Body.String())
	}

	_, reason := h.record(t)
	if reason != "corrected by supervisor" {
		t.Errorf("void_reason = %q, want the approver's correction", reason)
	}
}

// TestApprovalInput_MissingInputStillRejected keeps Fase 3's enforcement on the
// approval path: the requesting call cannot skip the contract just because an
// approval will follow.
func TestApprovalInput_MissingInputStillRejected(t *testing.T) {
	h := setupApprovalInputHarness(t)

	rr := h.patch(t, "clerk-1", "", map[string]any{"status": "voided"})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing required input = %d, want 422; body: %s", rr.Code, rr.Body.String())
	}
	if status, _ := h.record(t); status != "posted" {
		t.Errorf("record must be untouched, status=%q", status)
	}
}

// TestApprovalInput_StoredOnTheApprovalRow proves the value is actually carried
// by the approval row rather than smuggled through the record, which is the only
// durable place it can live between the two requests.
func TestApprovalInput_StoredOnTheApprovalRow(t *testing.T) {
	h := setupApprovalInputHarness(t)

	if rr := h.patch(t, "clerk-1", "", map[string]any{
		"status": "voided", "void_reason": "customer complaint",
	}); rr.Code != http.StatusAccepted {
		t.Fatalf("start approval = %d, want 202: %s", rr.Code, rr.Body.String())
	}

	row, err := h.factory.approvalRequests.GetByRecord(t.Context(), "t1", "billing.order", h.recordID)
	if err != nil {
		t.Fatalf("GetByRecord: %v", err)
	}
	if row == nil {
		t.Fatal("no pending approval row")
	}
	if row.Params["void_reason"] != "customer complaint" {
		t.Errorf("approval row params = %#v, want the requester's input stored on "+
			"the row — it is the only durable carrier across the 202 boundary", row.Params)
	}
	// The approval verb is not an entity field and must not be stored as one.
	if _, present := row.Params["decision"]; present {
		t.Errorf("`decision` is an approval-flow verb, not a record field: %#v", row.Params)
	}
}
