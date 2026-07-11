package validation

import (
	"testing"

	"github.com/primadi/forma/pkg/spec"
)

func TestValidateActionParams_Required(t *testing.T) {
	action := spec.Action{
		Name: "checkout",
		Params: &spec.ParamsDecl{
			Validate: []spec.ParamValidation{
				{Field: "customer_id", Rules: []spec.ValidationRule{{Name: "required"}}},
				{Field: "items", Rules: []spec.ValidationRule{{Name: "required"}}},
			},
		},
	}

	// Missing required params
	errs := ValidateActionParams(map[string]any{"customer_id": "123"}, action.Params.Validate)
	if len(errs) == 0 {
		t.Fatal("expected error for missing required param 'items'")
	}

	// All required params present
	errs = ValidateActionParams(map[string]any{"customer_id": "123", "items": []any{"a"}}, action.Params.Validate)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateActionParams_MinMax(t *testing.T) {
	action := spec.Action{
		Name: "search",
		Params: &spec.ParamsDecl{
			Validate: []spec.ParamValidation{
				{Field: "q", Rules: []spec.ValidationRule{{Name: "min_length", Value: 3}}},
				{Field: "limit", Rules: []spec.ValidationRule{{Name: "max", Value: 100}}},
			},
		},
	}

	// Too short query
	errs := ValidateActionParams(map[string]any{"q": "ab"}, action.Params.Validate)
	if len(errs) == 0 {
		t.Fatal("expected error for too short query (min_length)")
	}

	// Valid query
	errs = ValidateActionParams(map[string]any{"q": "abc", "limit": float64(10)}, action.Params.Validate)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}

	// Limit too high
	errs = ValidateActionParams(map[string]any{"q": "abc", "limit": float64(200)}, action.Params.Validate)
	if len(errs) == 0 {
		t.Fatal("expected error for limit > 100")
	}
}

func TestValidateActionParams_Positive(t *testing.T) {
	action := spec.Action{
		Name: "transfer",
		Params: &spec.ParamsDecl{
			Validate: []spec.ParamValidation{
				{Field: "amount", Rules: []spec.ValidationRule{{Name: "positive"}}},
			},
		},
	}

	// Zero amount → should fail
	errs := ValidateActionParams(map[string]any{"amount": float64(0)}, action.Params.Validate)
	if len(errs) == 0 {
		t.Fatal("expected error for zero amount")
	}

	// Positive amount → should pass
	errs = ValidateActionParams(map[string]any{"amount": float64(100)}, action.Params.Validate)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateActionParams_Empty(t *testing.T) {
	// No params declared → always valid
	errs := ValidateActionParams(map[string]any{"anything": "value"}, nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors for nil params, got: %v", errs)
	}

	// Empty validate list → always valid
	errs = ValidateActionParams(map[string]any{}, []spec.ParamValidation{})
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty validate list, got: %v", errs)
	}
}

func TestValidateCrossField_After_Before(t *testing.T) {
	// after: end_date must be after start_date
	data := map[string]any{
		"start_date": "2026-01-01",
		"end_date":   "2026-01-15",
	}

	err := ValidateCrossField("end_date", data["end_date"],
		spec.ValidationRule{Name: "after", Value: "start_date"}, data)
	if err != nil {
		t.Errorf("expected no error for end_date after start_date, got: %v", err)
	}

	// Reversed: end_date before start_date → fail
	data2 := map[string]any{
		"start_date": "2026-02-01",
		"end_date":   "2026-01-15",
	}
	err = ValidateCrossField("end_date", data2["end_date"],
		spec.ValidationRule{Name: "after", Value: "start_date"}, data2)
	if err == nil {
		t.Fatal("expected error for end_date before start_date")
	}

	// before: start_date must be before end_date
	err = ValidateCrossField("start_date", data2["start_date"],
		spec.ValidationRule{Name: "before", Value: "end_date"}, data2)
	if err == nil {
		t.Fatal("expected error for start_date after end_date")
	}
}

func TestValidateCrossField_AfterFieldAlias(t *testing.T) {
	data := map[string]any{
		"check_in":  "2026-06-01",
		"check_out": "2026-06-05",
	}

	// after_field is an alias for after
	err := ValidateCrossField("check_out", data["check_out"],
		spec.ValidationRule{Name: "after_field", Value: "check_in"}, data)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateCrossField_SkipOnMissing(t *testing.T) {
	// Both fields missing → skip (no error)
	data := map[string]any{}
	err := ValidateCrossField("end_date", nil,
		spec.ValidationRule{Name: "after", Value: "start_date"}, data)
	if err != nil {
		t.Errorf("expected no error when reference field is missing, got: %v", err)
	}
}

func TestValidateCrossField_Exists(t *testing.T) {
	// exists is a stub for now (Fase 2) — always passes
	err := ValidateCrossField("customer_id", "uuid-123",
		spec.ValidationRule{Name: "exists", Value: "billing.customer"},
		map[string]any{})
	if err != nil {
		t.Errorf("expected no error for exists stub, got: %v", err)
	}
}

func TestToInt_ToFloat(t *testing.T) {
	if n := toInt(42); n != 42 {
		t.Errorf("toInt(42) = %d, want 42", n)
	}
	if n := toInt(3.14); n != 3 {
		t.Errorf("toInt(3.14) = %d, want 3", n)
	}
	if n := toFloat(3.14); n != 3.14 {
		t.Errorf("toFloat(3.14) = %v, want 3.14", n)
	}
	if n := toFloat(42); n != 42.0 {
		t.Errorf("toFloat(42) = %v, want 42.0", n)
	}
	if n := toFloat("not-a-number"); n != 0 {
		t.Errorf("toFloat(string) = %v, want 0", n)
	}
}
