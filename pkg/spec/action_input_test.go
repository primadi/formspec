package spec

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// baseEntity is the smallest entity the input-contract checks need: one field to
// refer to, and enough shape to pass the rest of ValidateEntitySpec untouched.
func baseEntity(params *ParamsDecl) *EntitySpec {
	return &EntitySpec{
		Fields: []Field{
			{Name: "void_reason", Type: FieldString},
			{Name: "tags", Type: FieldJSON, Multiple: boolPtr(true)},
		},
		StateMachine: &StateMachine{
			Field:   "status",
			Initial: "paid",
			States:  []StateDecl{{Name: "paid"}, {Name: "cancelled"}},
			Transitions: []TransitionDecl{
				{From: StateList{"paid"}, To: "cancelled", Action: "void-order", Params: params},
			},
		},
	}
}

func boolPtr(b bool) *bool { return &b }

func TestValidateActionInputs_ReferringInputInheritsField(t *testing.T) {
	// The whole point of the referring form: no `type` is declared, because the
	// Entity field already carries it.
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "void_reason", Widget: WidgetTextarea}},
	}))
	if err != nil {
		t.Fatalf("referring input should validate: %v", err)
	}
}

func TestValidateActionInputs_AdHocRequiresType(t *testing.T) {
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "approver_note"}},
	}))
	if err == nil {
		t.Fatal("ad-hoc input without a type must be rejected — there is nothing to render or validate")
	}
	for _, want := range []string{"approver_note", "ad-hoc", "type"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %q, got: %v", want, err)
		}
	}
}

func TestValidateActionInputs_AdHocWithTypeIsAccepted(t *testing.T) {
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "approver_note", Type: FieldText}},
	}))
	if err != nil {
		t.Fatalf("ad-hoc input with a type should validate: %v", err)
	}
}

func TestValidateActionInputs_UnknownAdHocType(t *testing.T) {
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "n", Type: FieldType("relaion")}},
	}))
	if err == nil || !strings.Contains(err.Error(), "relaion") {
		t.Fatalf("unknown ad-hoc type must be named, got: %v", err)
	}
}

func TestValidateActionInputs_ReferringInputRefusesSecondType(t *testing.T) {
	// Two types for one value, with only the field's actually enforced: the
	// declaration that LOOKS authoritative (the input's) is the one that is not.
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "void_reason", Type: FieldText}},
	}))
	if err == nil {
		t.Fatal("a referring input must not redeclare the field's type")
	}
	if !strings.Contains(err.Error(), "type") {
		t.Errorf("error should explain the type conflict, got: %v", err)
	}
}

func TestValidateActionInputs_DuplicateInputName(t *testing.T) {
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{
			{Name: "void_reason"},
			{Name: "void_reason", Widget: WidgetTextarea},
		},
	}))
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("duplicate input name must be rejected, got: %v", err)
	}
}

func TestValidateActionInputs_UnknownWidget(t *testing.T) {
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "void_reason", Widget: FormWidget("text-area")}},
	}))
	if err == nil || !strings.Contains(err.Error(), "text-area") {
		t.Fatalf("widget typo must be rejected and named, got: %v", err)
	}
}

func TestValidateActionInputs_WidgetCardinalityMismatch(t *testing.T) {
	// The cardinality rule only speaks when a choice set is declared, so the
	// fixture needs one — `tags` (a bare set with no options) is deliberately
	// outside the rule's reach.
	d := baseEntity(nil)
	d.Fields = append(d.Fields, Field{
		Name:     "reason_code",
		Type:     FieldString,
		Options:  []FieldOption{{Value: "spoil"}, {Value: "mistake"}},
		Multiple: boolPtr(true),
	})

	ok := ValidateActionInputs(withTransitionParams(d, &ParamsDecl{
		Inputs: []ParamInput{{Name: "reason_code", Widget: WidgetSelectMultiTag}},
	}))
	if ok != nil {
		t.Fatalf("multi-tag widget on a set field should validate: %v", ok)
	}
	err := ValidateActionInputs(withTransitionParams(d, &ParamsDecl{
		Inputs: []ParamInput{{Name: "reason_code", Widget: WidgetSelect}},
	}))
	if err == nil {
		t.Fatal("single-value widget on a set field must be rejected")
	}
}

// withTransitionParams returns a copy of d whose single transition carries the
// given params, so a test can vary the contract without rebuilding the fixture.
func withTransitionParams(d *EntitySpec, p *ParamsDecl) *EntitySpec {
	d.StateMachine.Transitions[0].Params = p
	return d
}

func TestValidateActionInputs_InputsFromMustResolve(t *testing.T) {
	d := baseEntity(&ParamsDecl{InputsFrom: []string{"reason"}})
	err := ValidateActionInputs(d)
	if err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("unresolved inputs_from must be rejected and named, got: %v", err)
	}
}

func TestValidateActionInputs_InputsFromResolves(t *testing.T) {
	d := baseEntity(&ParamsDecl{InputsFrom: []string{"reason"}})
	d.InputSets = []InputSet{{Name: "reason", Inputs: []ParamInput{{Name: "void_reason"}}}}
	if err := ValidateActionInputs(d); err != nil {
		t.Fatalf("a declared input set should resolve: %v", err)
	}
}

func TestValidateActionInputs_InputsFromDuplicateAcrossSetAndInline(t *testing.T) {
	// The set and the inline list are merged before checking, so a name in both
	// is caught even though neither list is internally inconsistent.
	d := baseEntity(&ParamsDecl{
		Inputs:     []ParamInput{{Name: "void_reason"}},
		InputsFrom: []string{"reason"},
	})
	d.InputSets = []InputSet{{Name: "reason", Inputs: []ParamInput{{Name: "void_reason"}}}}
	err := ValidateActionInputs(d)
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("a name both inline and via inputs_from must be rejected, got: %v", err)
	}
}

func TestValidateActionInputs_RenderModeEnum(t *testing.T) {
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Render: &ParamsRenderHint{Mode: "popup"},
	}))
	if err == nil || !strings.Contains(err.Error(), "popup") {
		t.Fatalf("unknown render mode must be rejected and named, got: %v", err)
	}
	for _, mode := range []string{"modal", "drawer", "separate_page"} {
		ok := ValidateActionInputs(baseEntity(&ParamsDecl{
			Inputs: []ParamInput{{Name: "void_reason"}},
			Render: &ParamsRenderHint{Mode: mode},
		}))
		if ok != nil {
			t.Errorf("render mode %q should validate: %v", mode, ok)
		}
	}
}

func TestValidateActionInputs_PersistTrueNeedsAField(t *testing.T) {
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "approver_note", Type: FieldText, Persist: boolPtr(true)}},
	}))
	if err == nil || !strings.Contains(err.Error(), "persist") {
		t.Fatalf("persist: true with no destination field must be rejected, got: %v", err)
	}
	// persist: false is the whole point of an ad-hoc input — allowed.
	ok := ValidateActionInputs(baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "approver_note", Type: FieldText, Persist: boolPtr(false)}},
	}))
	if ok != nil {
		t.Fatalf("persist: false on an ad-hoc input should validate: %v", ok)
	}
}

func TestValidateActionInputs_AdditiveOnExistingManifests(t *testing.T) {
	// Every manifest written before this contract carries only `validate:` —
	// that must keep passing, since there are no inputs to check.
	err := ValidateActionInputs(baseEntity(&ParamsDecl{
		Validate: []ParamValidation{{Field: "void_reason", Rules: []ValidationRule{{Name: "required"}}}},
	}))
	if err != nil {
		t.Fatalf("params.validate alone must still validate: %v", err)
	}
}

func TestValidateActionInputs_CheckedOnDeclaredActionsAndSets(t *testing.T) {
	d := &EntitySpec{
		Fields: []Field{{Name: "amount", Type: FieldDecimal}},
		Actions: []Action{{
			Name:   "refund",
			Params: &ParamsDecl{Inputs: []ParamInput{{Name: "missing"}}},
		}},
	}
	err := ValidateActionInputs(d)
	if err == nil {
		t.Fatal("an action's inputs must be checked too, not only transitions'")
	}
	if !strings.Contains(err.Error(), `action "refund"`) {
		t.Errorf("error should locate the action, got: %v", err)
	}

	err = ValidateActionInputs(&EntitySpec{
		Fields:    []Field{{Name: "amount", Type: FieldDecimal}},
		InputSets: []InputSet{{Name: "bad", Inputs: []ParamInput{{Name: "nope"}}}},
	})
	if err == nil || !strings.Contains(err.Error(), `input_sets "bad"`) {
		t.Fatalf("an input set's inputs must be checked, got: %v", err)
	}
}

// TestActionSources_CarriesParams pins the trap this contract had to avoid: the
// synthesized action for a transition `via` copies `Params` by pointer, so the
// renderer sees a transition's inputs through `actions[]` as well as through
// `state_machine`. A field-by-field literal is exactly where this is lost.
func TestActionSources_CarriesParams(t *testing.T) {
	d := baseEntity(&ParamsDecl{
		Inputs: []ParamInput{{Name: "void_reason", Widget: WidgetTextarea}},
		Render: &ParamsRenderHint{Mode: "drawer"},
	})
	sources := d.ActionSources()
	if len(sources) != 1 {
		t.Fatalf("want one synthesized action, got %d", len(sources))
	}
	p := sources[0].Params
	if p == nil {
		t.Fatal("ActionSources dropped Params — the input contract would be invisible to the renderer")
	}
	if len(p.Inputs) != 1 || p.Inputs[0].Name != "void_reason" {
		t.Fatalf("inputs not carried: %#v", p.Inputs)
	}
	if p.Render == nil || p.Render.Mode != "drawer" {
		t.Fatalf("render hint not carried: %#v", p.Render)
	}
}

// TestTransitionDecl_UnmarshalCarriesParamInputs guards the same load-time trap
// the `,inline` unmarshaler exists for: a manifest's `params.inputs` must
// survive YAML decoding, not merely exist as a struct field.
func TestTransitionDecl_UnmarshalCarriesParamInputs(t *testing.T) {
	src := `
from: paid
to: cancelled
via: void-order
params:
  inputs:
    - name: void_reason
      widget: textarea
      required_when: "fields.status == 'paid'"
  inputs_from: [reason]
  render:
    mode: modal
`
	var t2 TransitionDecl
	if err := yaml.Unmarshal([]byte(src), &t2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if t2.Params == nil || len(t2.Params.Inputs) != 1 {
		t.Fatalf("params.inputs lost: %#v", t2.Params)
	}
	in := t2.Params.Inputs[0]
	if in.Name != "void_reason" || in.Widget != WidgetTextarea {
		t.Errorf("input fields lost: %#v", in)
	}
	if in.RequiredWhen != "fields.status == 'paid'" {
		t.Errorf("required_when lost: %q", in.RequiredWhen)
	}
	if len(t2.Params.InputsFrom) != 1 || t2.Params.InputsFrom[0] != "reason" {
		t.Errorf("inputs_from lost: %#v", t2.Params.InputsFrom)
	}
	if t2.Params.Render == nil || t2.Params.Render.Mode != "modal" {
		t.Errorf("render hint lost: %#v", t2.Params.Render)
	}
}

// TestValidFieldType covers the ad-hoc type check, which has no storage layer to
// catch a typo the way a real field does.
func TestValidFieldType(t *testing.T) {
	for _, ok := range []FieldType{FieldString, FieldText, FieldDecimal, FieldBoolean, FieldJSON} {
		if !ValidFieldType(ok) {
			t.Errorf("%q should be a valid field type", ok)
		}
	}
	for _, bad := range []FieldType{"", "textt", "relaion"} {
		if ValidFieldType(bad) {
			t.Errorf("%q should not be a valid field type", bad)
		}
	}
}
