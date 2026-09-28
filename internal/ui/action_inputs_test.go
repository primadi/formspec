package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// inputContractEntity is an entity whose transition declares an input contract —
// the shape the renderer needs to build a form for `void-order`.
func inputContractEntity() *spec.EntitySpec {
	return &spec.EntitySpec{
		Fields: []spec.Field{
			{Name: "status", Type: spec.FieldString},
			{Name: "void_reason", Type: spec.FieldString, Title: "Alasan Void"},
		},
		InputSets: []spec.InputSet{{
			Name:   "reason",
			Inputs: []spec.ParamInput{{Name: "void_reason", Widget: spec.WidgetTextarea}},
		}},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "paid",
			States:  []spec.StateDecl{{Name: "paid"}, {Name: "cancelled"}},
			Transitions: []spec.TransitionDecl{{
				From:              spec.StateList{"paid"},
				To:                "cancelled",
				Action:            "void-order",
				RequirePermission: "orders.void-order",
				Params: &spec.ParamsDecl{
					Inputs: []spec.ParamInput{{Name: "void_reason", Widget: spec.WidgetTextarea, RequiredWhen: "fields.status == 'paid'"}},
					Render: &spec.ParamsRenderHint{Mode: "modal"},
				},
			}},
		},
	}
}

func marshalEntitySchema(t *testing.T, es *spec.EntitySpec) map[string]any {
	t.Helper()
	raw, err := json.Marshal(buildEntitySchema(EntityDescriptor{Module: "cafe-order", Name: "order", Spec: es}))
	if err != nil {
		t.Fatalf("marshal bundle entity: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal bundle entity: %v", err)
	}
	return out
}

func TestActionSummary_CarriesInputContract(t *testing.T) {
	got := marshalEntitySchema(t, inputContractEntity())

	actions, _ := got["actions"].([]any)
	if len(actions) == 0 {
		t.Fatal("no actions in the bundle")
	}
	var found map[string]any
	for _, a := range actions {
		m, _ := a.(map[string]any)
		if m["name"] == "void-order" {
			found = m
		}
	}
	if found == nil {
		t.Fatalf("transition `via` missing from actions[]: %v", actions)
	}

	// Without `params` in the bundle the renderer can only know that the button
	// might need something — which is exactly the dead-end `has_params` was.
	params, _ := found["params"].(map[string]any)
	if params == nil {
		t.Fatal("ActionSummary dropped `params` — the renderer cannot build the input form")
	}
	inputs, _ := params["inputs"].([]any)
	if len(inputs) != 1 {
		t.Fatalf("params.inputs = %v, want 1 entry", params["inputs"])
	}
	in, _ := inputs[0].(map[string]any)
	if in["name"] != "void_reason" {
		t.Errorf("input name lost: %v", in)
	}
	if in["widget"] != "textarea" {
		t.Errorf("input widget lost: %v", in)
	}
	if in["required_when"] != "fields.status == 'paid'" {
		t.Errorf("input predicate lost: %v", in)
	}
	render, _ := params["render"].(map[string]any)
	if render == nil || render["mode"] != "modal" {
		t.Errorf("params.render lost: %v", params["render"])
	}

	if found["has_params"] != true {
		t.Errorf("has_params should be true for an action declaring inputs, got %v", found["has_params"])
	}
}

func TestEntitySchema_CarriesInputSets(t *testing.T) {
	got := marshalEntitySchema(t, inputContractEntity())

	sets, _ := got["input_sets"].([]any)
	if len(sets) != 1 {
		t.Fatalf("input_sets = %v, want 1 entry — an action's `inputs_from` resolves "+
			"against these, so omitting them makes the reference unresolvable in the browser", got["input_sets"])
	}
	set, _ := sets[0].(map[string]any)
	if set["name"] != "reason" {
		t.Errorf("set name lost: %v", set)
	}
	inputs, _ := set["inputs"].([]any)
	if len(inputs) != 1 {
		t.Fatalf("set inputs = %v, want 1 entry", set["inputs"])
	}
}

// TestEntitySchema_NoParamsSerialisedWhenAbsent pins the deliberate difference
// from `HasRoute`: an absent `params` and an empty one mean the same thing
// ("nothing to collect"), so omitting it is honest and keeps the bundle small.
// Unlike `has_route`, no client has to distinguish "absent" from "false".
func TestEntitySchema_NoParamsSerialisedWhenAbsent(t *testing.T) {
	es := &spec.EntitySpec{
		Fields:  []spec.Field{{Name: "status", Type: spec.FieldString}},
		Actions: []spec.Action{{Name: "ship"}},
	}
	got := marshalEntitySchema(t, es)
	actions, _ := got["actions"].([]any)
	for _, a := range actions {
		m, _ := a.(map[string]any)
		if m["name"] == "ship" {
			if _, present := m["params"]; present {
				t.Errorf("params should be omitted for an action with no contract, got %v", m["params"])
			}
			if _, present := m["has_params"]; present {
				t.Errorf("has_params should be omitted when false, got %v", m["has_params"])
			}
			return
		}
	}
	t.Fatal("action `ship` not found in the bundle")
}

// The state machine is shipped raw, so a transition's own contract reaches the
// client even when the `via` is ALSO declared under `actions:` — the two are
// separate declarations and the transition's is the one the PATCH path reads.
func TestEntitySchema_TransitionCarriesParamsRaw(t *testing.T) {
	raw, err := json.Marshal(buildEntitySchema(EntityDescriptor{Module: "cafe-order", Name: "order", Spec: inputContractEntity()}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"required_when":"fields.status == 'paid'"`) {
		t.Errorf("transition params missing from the raw state_machine projection: %s", raw)
	}
}
