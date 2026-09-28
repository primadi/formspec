package spec

import (
	"strings"
	"testing"
)

// transitionWithContract builds an entity whose transition contract can live
// either on the transition itself or on a declared action sharing its `via` —
// the two shapes the write paths must treat identically.
func transitionWithContract(onAction bool, params *ParamsDecl, conds []ConditionDecl) *EntitySpec {
	t := TransitionDecl{From: StateList{"posted"}, To: "voided", Action: "void-order"}
	if !onAction {
		t.Params = params
		t.Conditions = conds
	}
	d := &EntitySpec{
		Fields: []Field{
			{Name: "status", Type: FieldString},
			{Name: "void_reason", Type: FieldString},
		},
		StateMachine: &StateMachine{
			Field:       "status",
			Initial:     "posted",
			States:      []StateDecl{{Name: "posted"}, {Name: "voided"}},
			Transitions: []TransitionDecl{t},
		},
	}
	if onAction {
		d.Actions = []Action{{Name: "void-order", Params: params, Conditions: conds}}
	}
	return d
}

func TestEffectiveActionSpec_FindsDeclaredActionOrSynthesizedOne(t *testing.T) {
	params := &ParamsDecl{Inputs: []ParamInput{{Name: "void_reason"}}}

	for _, onAction := range []bool{true, false} {
		name := "contract on transition"
		if onAction {
			name = "contract on declared action"
		}
		t.Run(name, func(t *testing.T) {
			d := transitionWithContract(onAction, params, nil)
			got := EffectiveActionSpec(d, &d.StateMachine.Transitions[0])
			if got == nil {
				t.Fatal("EffectiveActionSpec returned nil — the contract would be unenforced")
			}
			if got.Params == nil || len(got.Params.Inputs) != 1 {
				t.Fatalf("contract not resolved: %#v", got.Params)
			}
		})
	}
}

// The overlay — the bug the kafe e2e caught (todo 5.24.4/5.24.5).
//
// `ActionSources` lets a DECLARED action win outright, which is right for
// "which entry is listed in actions[]" but wrong for "what contract governs
// this transition". A declared entry survives
// `ValidateActionTransitionDuplication` exactly when it adds something the
// transition LACKS, so the two legitimately describe different halves of one
// action. Reading the declared entry alone lost the transition's inputs: the
// value stayed in the request body and `params.get('void_reason')` saw nothing,
// so every void was refused with "Alasan void wajib diisi" while the body
// plainly contained it.
func TestEffectiveActionSpec_TransitionParamsWinOverDeclaredAction(t *testing.T) {
	// The kafe `void-order` shape: the declared action contributes
	// description + audit + conditions, the TRANSITION contributes the inputs.
	d := &EntitySpec{
		Fields: []Field{
			{Name: "status", Type: FieldString},
			{Name: "void_reason", Type: FieldString},
		},
		Actions: []Action{{
			Name:        "void-order",
			Description: "Batalkan pesanan yang sudah dibayar",
			Audit:       true,
			Conditions: []ConditionDecl{{
				Script:  "len(params.get('void_reason', '')) > 0",
				Message: "Alasan void wajib diisi",
			}},
		}},
		StateMachine: &StateMachine{
			Field: "status", Initial: "paid",
			States: []StateDecl{{Name: "paid"}, {Name: "cancelled"}},
			Transitions: []TransitionDecl{{
				From: StateList{"paid"}, To: "cancelled", Action: "void-order",
				Params: &ParamsDecl{Inputs: []ParamInput{{Name: "void_reason"}}},
			}},
		},
	}
	trans := &d.StateMachine.Transitions[0]

	got := EffectiveActionSpec(d, trans)
	if got == nil {
		t.Fatal("EffectiveActionSpec returned nil")
	}
	if got.Params == nil || len(got.Params.Inputs) != 1 {
		t.Fatalf("the transition's inputs were dropped in favour of the declared "+
			"action's: %#v — the value would never reach `params`, and a condition "+
			"reading it would refuse every call", got.Params)
	}
	// The declared entry's contributions must survive the overlay.
	if len(got.Conditions) != 1 {
		t.Fatalf("the declared gate was dropped: %#v", got.Conditions)
	}
	if got.Description == "" || !got.Audit {
		t.Errorf("the declared description/audit were lost: %#v", got)
	}

	// What the write path actually reads: the body value must resolve.
	params := TransitionInputParams(d, trans, map[string]any{"void_reason": "salah pesan"}, nil)
	if params["void_reason"] != "salah pesan" {
		t.Fatalf("TransitionInputParams = %#v, want the body's void_reason — this is "+
			"exactly the read that returned nothing before the overlay", params)
	}
}

// Both declaration sites gate the SAME transition, so neither may be dropped.
// Replacing would let a declared gate stop being enforced with no error.
func TestEffectiveActionSpec_KeepsBothConditionSets(t *testing.T) {
	d := &EntitySpec{
		Fields:  []Field{{Name: "status", Type: FieldString}},
		Actions: []Action{{Name: "ship", Conditions: []ConditionDecl{{Script: "True", Message: "action"}}}},
		StateMachine: &StateMachine{
			Field: "status", Initial: "a",
			States: []StateDecl{{Name: "a"}, {Name: "b"}},
			Transitions: []TransitionDecl{{
				From: StateList{"a"}, To: "b", Action: "ship",
				Conditions: []ConditionDecl{{Script: "True", Message: "transition"}},
			}},
		},
	}
	got := EffectiveActionSpec(d, &d.StateMachine.Transitions[0])
	if len(got.Conditions) != 2 {
		t.Fatalf("conditions = %#v, want both declarations kept", got.Conditions)
	}
	// The transition's own gate is evaluated first — it is the more specific
	// declaration, and an evaluation error from it should name the closer source.
	if got.Conditions[0].Message != "transition" {
		t.Errorf("the transition's condition should come first, got %#v", got.Conditions)
	}
}

// The overlay must not mutate the shared source slice: `ActionSources()` returns
// a fresh slice per call today, but a caller that cached one would see the
// transition's params leak onto the declared action.
func TestEffectiveActionSpec_DoesNotMutateActionSources(t *testing.T) {
	d := transitionWithContract(true, &ParamsDecl{Inputs: []ParamInput{{Name: "a"}}}, nil)
	d.StateMachine.Transitions[0].Params = &ParamsDecl{Inputs: []ParamInput{{Name: "b"}}}

	_ = EffectiveActionSpec(d, &d.StateMachine.Transitions[0])

	for i := range d.Actions {
		if d.Actions[i].Name != "void-order" {
			continue
		}
		if d.Actions[i].Params != nil && len(d.Actions[i].Params.Inputs) > 0 &&
			d.Actions[i].Params.Inputs[0].Name == "b" {
			t.Fatal("the overlay wrote the transition's params onto the declared action")
		}
	}
}

func TestEffectiveActionSpec_NilWhenNoVia(t *testing.T) {
	d := &EntitySpec{
		Fields: []Field{{Name: "status", Type: FieldString}},
		StateMachine: &StateMachine{
			Field: "status", Initial: "a",
			States:      []StateDecl{{Name: "a"}, {Name: "b"}},
			Transitions: []TransitionDecl{{From: StateList{"a"}, To: "b"}},
		},
	}
	if got := EffectiveActionSpec(d, &d.StateMachine.Transitions[0]); got != nil {
		t.Fatalf("a transition with no `via` names no action, got %#v", got)
	}
}

func TestTransitionInputParams_ReadsDeclaredNamesOnly(t *testing.T) {
	d := transitionWithContract(false, &ParamsDecl{
		Inputs: []ParamInput{{Name: "void_reason"}},
	}, nil)
	trans := &d.StateMachine.Transitions[0]

	got := TransitionInputParams(d, trans, map[string]any{
		"void_reason": "customer complaint",
		"status":      "voided", // not declared as an input — must not be promoted
	}, map[string]any{"status": "posted"})

	if got["void_reason"] != "customer complaint" {
		t.Errorf("declared input not read from the body: %#v", got)
	}
	if _, present := got["status"]; present {
		t.Errorf("an undeclared key was promoted into the condition scope: %#v — a "+
			"caller could then smuggle any record field into a guard", got)
	}
}

func TestTransitionInputParams_FallsBackToRecord(t *testing.T) {
	d := transitionWithContract(false, &ParamsDecl{
		Inputs: []ParamInput{{Name: "void_reason"}},
	}, nil)
	trans := &d.StateMachine.Transitions[0]

	// The caller omitted the (optional) input; the value already on the record
	// must satisfy a `required` check instead of failing one it cannot address.
	got := TransitionInputParams(d, trans, map[string]any{"status": "voided"},
		map[string]any{"void_reason": "already there"})

	if got["void_reason"] != "already there" {
		t.Errorf("record fallback missing: %#v", got)
	}
}

func TestTransitionInputParams_BodyWinsOverRecord(t *testing.T) {
	d := transitionWithContract(false, &ParamsDecl{
		Inputs: []ParamInput{{Name: "void_reason"}},
	}, nil)
	trans := &d.StateMachine.Transitions[0]

	got := TransitionInputParams(d, trans,
		map[string]any{"void_reason": "new reason"},
		map[string]any{"void_reason": "old reason"})

	if got["void_reason"] != "new reason" {
		t.Errorf("the request body must win, got %#v", got["void_reason"])
	}
}

func TestTransitionInputParams_ResolvesInputSets(t *testing.T) {
	d := transitionWithContract(false, &ParamsDecl{InputsFrom: []string{"reason"}}, nil)
	d.InputSets = []InputSet{{Name: "reason", Inputs: []ParamInput{{Name: "void_reason"}}}}
	trans := &d.StateMachine.Transitions[0]

	got := TransitionInputParams(d, trans, map[string]any{"void_reason": "because"}, nil)
	if got["void_reason"] != "because" {
		t.Errorf("inputs_from not resolved: %#v", got)
	}
}

func TestTransitionInputParams_ValidateOnlyStillNamesParams(t *testing.T) {
	// A contract expressed only as `validate:` has no `inputs`, but a condition
	// still reads the named parameter — so the names must come through.
	d := transitionWithContract(false, &ParamsDecl{
		Validate: []ParamValidation{{Field: "void_reason", Rules: []ValidationRule{{Name: "required"}}}},
	}, nil)
	trans := &d.StateMachine.Transitions[0]

	got := TransitionInputParams(d, trans, map[string]any{"void_reason": "x"}, nil)
	if got["void_reason"] != "x" {
		t.Errorf("validate-only contract lost its parameter names: %#v", got)
	}
}

func TestTransitionInputParams_NilWithoutContract(t *testing.T) {
	d := transitionWithContract(false, nil, nil)
	trans := &d.StateMachine.Transitions[0]
	if got := TransitionInputParams(d, trans, map[string]any{"void_reason": "x"}, nil); got != nil {
		t.Errorf("no contract means nothing to read, got %#v", got)
	}
}

func TestPersistableInputs_ReferringPersistsAdHocDoesNot(t *testing.T) {
	d := &EntitySpec{
		Fields: []Field{{Name: "void_reason", Type: FieldString}},
	}
	got := PersistableInputs(d, &ParamsDecl{
		Inputs: []ParamInput{
			{Name: "void_reason"},                    // refers to a field → persists
			{Name: "approver_note", Type: FieldText}, // ad-hoc → handler only
		},
	})
	if len(got) != 1 || got[0] != "void_reason" {
		t.Fatalf("PersistableInputs = %v, want only the referring input", got)
	}
}

func TestPersistableInputs_ExplicitOverride(t *testing.T) {
	d := &EntitySpec{Fields: []Field{{Name: "void_reason", Type: FieldString}}}
	no := false

	got := PersistableInputs(d, &ParamsDecl{
		Inputs: []ParamInput{{Name: "void_reason", Persist: &no}},
	})
	if len(got) != 0 {
		t.Errorf("`persist: false` must keep the value off the record, got %v", got)
	}

	// An ad-hoc name cannot be persisted even when asked — there is no field.
	yes := true
	got = PersistableInputs(d, &ParamsDecl{
		Inputs: []ParamInput{{Name: "note", Type: FieldText, Persist: &yes}},
	})
	if len(got) != 0 {
		t.Errorf("an ad-hoc input has no destination field, got %v", got)
	}
}

// TestValidateActionInputs_RejectsPersistTrueOnAdHoc pins the validator half of
// the rule above: the manifest is refused rather than silently not persisting.
func TestValidateActionInputs_RejectsPersistTrueOnAdHoc(t *testing.T) {
	yes := true
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "note", Type: FieldText, Persist: &yes}},
	}))
	if err == nil || !strings.Contains(err.Error(), "persist") {
		t.Fatalf("persist: true with no destination field must be refused, got: %v", err)
	}
}
