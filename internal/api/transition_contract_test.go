package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// This file closes the enforcement gap in the action-input contract plan
// (docs_internal/plan/action-input-contract.md, Fase 3).
//
// `PATCH /{id}` is the ONLY path a transition without an `impl` can take, and it
// used to enforce neither the transition's `params.validate` nor its
// `conditions` — only `guard`, evaluated inside the store. A transition could
// therefore declare "a reason is required" and accept a bare
// `{"status":"cancelled"}` that skipped it entirely.
//
// The fixture mirrors the reported case: `void-order` moves `posted → voided`,
// declares `void_reason` as an input with a `required` rule, and guards on it
// with a condition reading `params.get('void_reason')`.

// transitionContractHarness is a PATCH-ready transition whose contract lives on
// the declared `actions:` entry (the shape kafe uses), so the test proves the
// contract is found through EffectiveActionSpec rather than only on the
// transition itself.
type transitionContractHarness struct {
	factory  *HandlerFactory
	recordID string
}

func setupTransitionContractHarness(t *testing.T, putContractOnAction bool) *transitionContractHarness {
	return setupTransitionContractFixture(t, putContractOnAction, 0)
}

// setupTransitionContractFixture builds the harness. `minReasonLen > 0` replaces
// the `required` rule with a length condition, which is how the CONDITION layer
// is exercised on its own: `required` already rejects an empty string, so a
// fixture carrying both can never reach the condition.
func setupTransitionContractFixture(t *testing.T, putContractOnAction bool, minReasonLen int) *transitionContractHarness {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "transition_contract.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)

	params := &spec.ParamsDecl{
		Inputs: []spec.ParamInput{{Name: "void_reason", Required: true}},
	}
	conditions := []spec.ConditionDecl{{
		Script:  "len(params.get('void_reason', '')) > 0",
		Message: "Alasan void wajib diisi",
	}}
	if minReasonLen > 0 {
		params = &spec.ParamsDecl{
			Inputs: []spec.ParamInput{{Name: "void_reason"}},
		}
		conditions = []spec.ConditionDecl{{
			Script:  fmt.Sprintf("len(params.get('void_reason', '')) >= %d", minReasonLen),
			Message: fmt.Sprintf("Alasan void minimal %d karakter", minReasonLen),
		}}
	} else {
		params.Validate = []spec.ParamValidation{
			{Field: "void_reason", Rules: []spec.ValidationRule{{Name: "required"}}},
		}
	}

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
			}},
		},
	}
	if putContractOnAction {
		// A declared entry carrying the contract: the transition names the same
		// `via` but declares nothing itself. EffectiveActionSpec must find it.
		orderSpec.Actions = []spec.Action{{
			Name:       "void-order",
			Params:     params,
			Conditions: conditions,
		}}
	} else {
		orderSpec.StateMachine.Transitions[0].Params = params
		orderSpec.StateMachine.Transitions[0].Conditions = conditions
	}

	registerTestEntity(t, d, reg, "billing", "order", orderSpec)

	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	recordID, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "tester",
		Data:        map[string]any{"status": "posted"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	factory := NewHandlerFactory(reg)
	// Production wires this from the entity registry; without it the handler
	// cannot resolve the state machine at all.
	factory.SetSpecLookup(func(m, n string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(m, n)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})
	return &transitionContractHarness{factory: factory, recordID: recordID}
}

// patchTransition issues the production PATCH shape for a transition: the target
// state in the state field, plus whatever caller-supplied inputs ride along.
func (h *transitionContractHarness) patchTransition(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest("PATCH", "/billing/orders/"+h.recordID, strings.NewReader(string(raw)))
	req.SetPathValue("id", h.recordID)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()
	h.factory.HandleUpdate("billing", "order")(rr, req)
	return rr
}

func (h *transitionContractHarness) stateOf(t *testing.T) (status, reason string) {
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
	status, _ = rec.Data["status"].(string)
	reason, _ = rec.Data["void_reason"].(string)
	return status, reason
}

// TestPatchTransition_MissingRequiredParamIsRejected is the regression this plan
// exists for: a bare `{"status":"voided"}` used to move the record, silently
// skipping the transition's declared contract.
func TestPatchTransition_MissingRequiredParamIsRejected(t *testing.T) {
	for _, onAction := range []bool{true, false} {
		name := "contract on transition"
		if onAction {
			name = "contract on declared action"
		}
		t.Run(name, func(t *testing.T) {
			h := setupTransitionContractHarness(t, onAction)

			rr := h.patchTransition(t, map[string]any{"status": "voided"})
			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("PATCH without the declared input = %d, want 422 — the "+
					"transition's own contract must be enforced on the path it is "+
					"actually applied through; body: %s", rr.Code, rr.Body.String())
			}
			status, _ := h.stateOf(t)
			if status != "posted" {
				t.Errorf("record moved to %q despite a rejected contract", status)
			}
		})
	}
}

// TestPatchTransition_EmptyParamIsRejected covers the empty case. An empty
// string satisfies presence but not the rule, so validation — not the condition —
// is what catches it: `required` on a string means non-empty.
func TestPatchTransition_EmptyParamIsRejected(t *testing.T) {
	h := setupTransitionContractHarness(t, false)

	rr := h.patchTransition(t, map[string]any{"status": "voided", "void_reason": ""})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty void_reason = %d, want 422; body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "void_reason") {
		t.Errorf("the rejection should name the parameter, got: %s", rr.Body.String())
	}
	status, _ := h.stateOf(t)
	if status != "posted" {
		t.Errorf("record moved to %q despite a rejected contract", status)
	}
}

// TestPatchTransition_ConditionFailureIsConditionFailed exercises the condition
// layer independently: with no `required` rule, a present-but-too-short value
// passes validation and must be stopped by the transition's own condition.
func TestPatchTransition_ConditionFailureIsConditionFailed(t *testing.T) {
	h := setupTransitionContractFixture(t, false, 5)

	rr := h.patchTransition(t, map[string]any{"status": "voided", "void_reason": "no"})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("too-short reason = %d, want 422; body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "CONDITION_FAILED") {
		t.Errorf("expected CONDITION_FAILED, got: %s", rr.Body.String())
	}
	status, _ := h.stateOf(t)
	if status != "posted" {
		t.Errorf("record moved to %q despite a failed condition", status)
	}

	ok := h.patchTransition(t, map[string]any{"status": "voided", "void_reason": "customer complaint"})
	if ok.Code != http.StatusOK {
		t.Fatalf("a reason satisfying the condition = %d, want 200; body: %s", ok.Code, ok.Body.String())
	}
}

// TestPatchTransition_WithParamSucceedsAndPersists is the happy path, and pins
// the other half of the contract: the collected value belongs on the RECORD, not
// only in the condition's evaluation scope.
func TestPatchTransition_WithParamSucceedsAndPersists(t *testing.T) {
	h := setupTransitionContractHarness(t, false)

	rr := h.patchTransition(t, map[string]any{"status": "voided", "void_reason": "customer complaint"})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH with the declared input = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	status, reason := h.stateOf(t)
	if status != "voided" {
		t.Errorf("state = %q, want voided", status)
	}
	if reason != "customer complaint" {
		t.Errorf("void_reason = %q, want it persisted with the state change — a "+
			"transition's input that only feeds the guard is a value the caller "+
			"supplied and nobody can read back", reason)
	}
}

// TestPatchTransition_NoContractIsUnchanged keeps the change additive: a
// transition that declares no contract must behave exactly as before.
func TestPatchTransition_NoContractIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "no_contract.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	orderSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields:  []spec.Field{{Name: "status", Type: spec.FieldString}},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "posted",
			States:  []spec.StateDecl{{Name: "posted"}, {Name: "voided"}},
			Transitions: []spec.TransitionDecl{
				{From: spec.StateList{"posted"}, To: "voided", Action: "void-order"},
			},
		},
	}
	registerTestEntity(t, d, reg, "billing", "order", orderSpec)

	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	id, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "t1", CreatedBy: "tester",
		Data: map[string]any{"status": "posted"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	factory := NewHandlerFactory(reg)
	factory.SetSpecLookup(func(m, n string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(m, n)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})

	req := httptest.NewRequest("PATCH", "/billing/orders/"+id, strings.NewReader(`{"status":"voided"}`))
	req.SetPathValue("id", id)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()
	factory.HandleUpdate("billing", "order")(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("a contract-free transition = %d, want 200 (the change must be "+
			"additive); body: %s", rr.Code, rr.Body.String())
	}
}
