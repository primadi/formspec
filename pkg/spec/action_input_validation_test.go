package spec

import "testing"

// The action-input contract has two halves that grew up separately, and a
// declaration is only a contract if the SERVER enforces it. These tests pin
// EffectiveParamValidation as the single place both halves are combined.

func TestEffectiveParamValidation_IncludesValidateRules(t *testing.T) {
	got := EffectiveParamValidation(nil, &ParamsDecl{
		Validate: []ParamValidation{{Field: "note", Rules: []ValidationRule{{Name: "min_length", Value: 3}}}},
	})
	if len(got) != 1 || got[0].Field != "note" || len(got[0].Rules) != 1 {
		t.Fatalf("validate rules lost: %#v", got)
	}
}

func TestEffectiveParamValidation_InputRequiredBecomesARule(t *testing.T) {
	// `required: true` on an input is the renderable spelling of the same
	// statement `validate: [{rules: [required]}]` makes. Enforcing only the
	// latter rendered a required field the server accepted as missing.
	got := EffectiveParamValidation(nil, &ParamsDecl{
		Inputs: []ParamInput{{Name: "void_reason", Required: true}},
	})
	if len(got) != 1 || got[0].Field != "void_reason" {
		t.Fatalf("required input not turned into a rule: %#v", got)
	}
	if !hasRule(got[0].Rules, "required") {
		t.Errorf("rules = %#v, want a `required` entry", got[0].Rules)
	}
}

func TestEffectiveParamValidation_InputRulesCarried(t *testing.T) {
	got := EffectiveParamValidation(nil, &ParamsDecl{
		Inputs: []ParamInput{{Name: "note", Rules: []ValidationRule{{Name: "max_length", Value: 10}}}},
	})
	if len(got) != 1 || !hasRule(got[0].Rules, "max_length") {
		t.Fatalf("input rules lost: %#v", got)
	}
}

func TestEffectiveParamValidation_InheritsEntityFieldRules(t *testing.T) {
	// An input that names a field inherits that field's rules, so the two cannot
	// disagree about one value.
	d := &EntitySpec{Fields: []Field{
		{Name: "void_reason", Type: FieldString, Rules: []ValidationRule{{Name: "min_length", Value: 5}}},
	}}
	got := EffectiveParamValidation(d, &ParamsDecl{Inputs: []ParamInput{{Name: "void_reason"}}})
	if len(got) != 1 || !hasRule(got[0].Rules, "min_length") {
		t.Fatalf("field rules not inherited: %#v", got)
	}
}

func TestEffectiveParamValidation_NoDuplicateRequired(t *testing.T) {
	d := &EntitySpec{Fields: []Field{
		{Name: "void_reason", Type: FieldString, Rules: []ValidationRule{{Name: "required"}}},
	}}
	got := EffectiveParamValidation(d, &ParamsDecl{
		Inputs:   []ParamInput{{Name: "void_reason", Required: true}},
		Validate: []ParamValidation{{Field: "void_reason", Rules: []ValidationRule{{Name: "required"}}}},
	})
	if len(got) != 1 {
		t.Fatalf("one field must produce one entry, got %#v", got)
	}
	n := 0
	for _, r := range got[0].Rules {
		if r.Name == "required" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("`required` declared 3 ways produced %d entries: %#v", n, got[0].Rules)
	}
}

func TestEffectiveParamValidation_ResolvesInputSets(t *testing.T) {
	d := &EntitySpec{
		Fields:    []Field{{Name: "void_reason", Type: FieldString}},
		InputSets: []InputSet{{Name: "reason", Inputs: []ParamInput{{Name: "void_reason", Required: true}}}},
	}
	got := EffectiveParamValidation(d, &ParamsDecl{InputsFrom: []string{"reason"}})
	if len(got) != 1 || !hasRule(got[0].Rules, "required") {
		t.Fatalf("input set not resolved: %#v", got)
	}
}

func TestEffectiveParamValidation_NilAndEmpty(t *testing.T) {
	if got := EffectiveParamValidation(nil, nil); got != nil {
		t.Errorf("nil params must yield nothing, got %#v", got)
	}
	if got := EffectiveParamValidation(nil, &ParamsDecl{}); len(got) != 0 {
		t.Errorf("an empty contract must yield nothing, got %#v", got)
	}
}
