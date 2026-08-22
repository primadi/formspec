package auth

import (
	"testing"
)

func TestEvaluateGrantConditions_Pass(t *testing.T) {
	conds := []ConditionGrant{
		{Expr: `resource["branch"] == "JKT"`, Message: "only Jakarta branch"},
	}
	err := EvaluateGrantConditions(conds, map[string]any{"branch": "JKT"}, nil)
	if err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestEvaluateGrantConditions_Fail(t *testing.T) {
	conds := []ConditionGrant{
		{Expr: `resource["branch"] == "JKT"`, Message: "only Jakarta branch"},
	}
	err := EvaluateGrantConditions(conds, map[string]any{"branch": "BDG"}, nil)
	if err == nil {
		t.Fatal("expected failure for non-matching branch")
	}
	if err.Error() != "abac: only Jakarta branch" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEvaluateGrantConditions_EmptyExprSkipped(t *testing.T) {
	conds := []ConditionGrant{{Expr: ""}}
	if err := EvaluateGrantConditions(conds, nil, nil); err != nil {
		t.Fatalf("expected empty expr skipped, got %v", err)
	}
}

func TestEvaluateGrantConditions_WithParams(t *testing.T) {
	conds := []ConditionGrant{
		{Expr: `params["amount"] <= 1000`, Message: "amount too high"},
	}
	if err := EvaluateGrantConditions(conds, nil, map[string]any{"amount": 500}); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
	if err := EvaluateGrantConditions(conds, nil, map[string]any{"amount": 5000}); err == nil {
		t.Fatal("expected failure for amount > 1000")
	}
}
