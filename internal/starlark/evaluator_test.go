package starlark

import (
	"math"
	"testing"
)

func TestEvalExpr_Arithmetic(t *testing.T) {
	result, err := EvalExpr("2 + 3", nil)
	if err != nil {
		t.Fatalf("EvalExpr failed: %v", err)
	}
	if result != int64(5) {
		t.Errorf("expected 5, got %v (%T)", result, result)
	}
}

func TestEvalExpr_WithEnv(t *testing.T) {
	result, err := EvalExpr("subtotal * 1.1", map[string]any{"subtotal": float64(100)})
	if err != nil {
		t.Fatalf("EvalExpr failed: %v", err)
	}
	got, ok := result.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T: %v", result, result)
	}
	if math.Abs(got-110) > 0.001 {
		t.Errorf("expected ~110, got %v", got)
	}
}

func TestEvalExpr_StringConcat(t *testing.T) {
	result, err := EvalExpr(`first_name + " " + last_name`, map[string]any{
		"first_name": "John",
		"last_name":  "Doe",
	})
	if err != nil {
		t.Fatalf("EvalExpr failed: %v", err)
	}
	if result != "John Doe" {
		t.Errorf("expected 'John Doe', got %v", result)
	}
}

func TestEvalExpr_BoolComparison(t *testing.T) {
	result, err := EvalExpr("total > 100", map[string]any{"total": float64(150)})
	if err != nil {
		t.Fatalf("EvalExpr failed: %v", err)
	}
	if result != true {
		t.Errorf("expected true, got %v", result)
	}

	result, err = EvalExpr("total > 100", map[string]any{"total": float64(50)})
	if err != nil {
		t.Fatalf("EvalExpr failed: %v", err)
	}
	if result != false {
		t.Errorf("expected false, got %v", result)
	}
}

func TestEvalExpr_Ternary(t *testing.T) {
	result, err := EvalExpr(`"high" if value > 50 else "low"`, map[string]any{"value": float64(75)})
	if err != nil {
		t.Fatalf("EvalExpr failed: %v", err)
	}
	if result != "high" {
		t.Errorf("expected 'high', got %v", result)
	}
}

func TestEvalExpr_ComplexFormula(t *testing.T) {
	result, err := EvalExpr("subtotal * (1 + tax_rate / 100)", map[string]any{
		"subtotal": float64(200),
		"tax_rate": float64(10),
	})
	if err != nil {
		t.Fatalf("EvalExpr failed: %v", err)
	}
	got, ok := result.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T: %v", result, result)
	}
	if math.Abs(got-220) > 0.001 {
		t.Errorf("expected ~220, got %v", got)
	}
}

func TestEvalExpr_EmptyEnv(t *testing.T) {
	result, err := EvalExpr("42", nil)
	if err != nil {
		t.Fatalf("EvalExpr failed: %v", err)
	}
	if result != int64(42) {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestEvalExpr_MathConstants(t *testing.T) {
	result, err := EvalExpr("math_pi", nil)
	if err != nil {
		t.Fatalf("EvalExpr failed: %v", err)
	}
	pi, ok := result.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T: %v", result, result)
	}
	if pi < 3.14 || pi > 3.15 {
		t.Errorf("expected pi ~3.14, got %v", pi)
	}
}

func TestEvalExpr_InvalidSyntax(t *testing.T) {
	_, err := EvalExpr("invalid syntax {{{", nil)
	if err == nil {
		t.Fatal("expected error for invalid syntax")
	}
}

func TestEvalExpr_UndefinedVariable(t *testing.T) {
	_, err := EvalExpr("undefined_var + 1", map[string]any{"some_key": float64(1)})
	if err == nil {
		t.Fatal("expected error for undefined variable")
	}
}
