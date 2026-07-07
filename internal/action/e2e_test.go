package action

import (
	"context"
	"testing"

	"github.com/forma/forma/pkg/spec"
)

// TestFullActionPipeline_E2E exercises the complete action execution flow:
// 1. Dispatch a script action (condition → starlark execution → result)
// 2. Dispatch a native action
// 3. Dispatch an action with no impl (no-op)
func TestFullActionPipeline_E2E_NoImplAction(t *testing.T) {
	disp := NewDispatcher()

	result, err := disp.Dispatch(context.Background(), spec.Action{
		Name: "noop",
		Impl: nil,
	}, ExecuteParams{
		Module:     "test",
		Entity:     "thing",
		ActionName: "noop",
	})
	if err != nil {
		t.Fatalf("no-impl action should not error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestFullActionPipeline_E2E_NativeAction(t *testing.T) {
	disp := NewDispatcher()
	nativeEx := NewNativeExecutor()

	// Register a native handler
	nativeEx.Register("test.thing.do-work", func(ctx context.Context, params ExecuteParams) (any, error) {
		return map[string]any{
			"result":  "success",
			"input":   params.Params["input"],
			"user_id": params.UserID,
		}, nil
	})

	disp.RegisterExecutor(spec.ImplNative, nativeEx)

	result, err := disp.Dispatch(context.Background(), spec.Action{
		Name: "do-work",
		Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "test.thing.do-work"},
	}, ExecuteParams{
		Module:     "test",
		Entity:     "thing",
		ActionName: "do-work",
		Params:     map[string]any{"input": "hello"},
		UserID:     "user-1",
	})

	if err != nil {
		t.Fatalf("native action failed: %v", err)
	}
	m, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result.Data)
	}
	if m["result"] != "success" {
		t.Errorf("expected success, got %v", m["result"])
	}
}

// TestConditionEvaluation verifies the condition evaluator works
// with several expression patterns.
func TestConditionEvaluation_E2E(t *testing.T) {
	tests := []struct {
		name     string
		cond     spec.ConditionDecl
		data     map[string]any
		wantPass bool
	}{
		{
			name:     "simple equality pass",
			cond:     spec.ConditionDecl{Script: "status == 'draft'", Message: "not draft"},
			data:     map[string]any{"status": "draft"},
			wantPass: true,
		},
		{
			name:     "simple equality fail",
			cond:     spec.ConditionDecl{Script: "status == 'draft'", Message: "not draft"},
			data:     map[string]any{"status": "paid"},
			wantPass: false,
		},
		{
			name:     "numeric comparison",
			cond:     spec.ConditionDecl{Script: "total > 0", Message: "total is zero"},
			data:     map[string]any{"total": float64(100)},
			wantPass: true,
		},
		{
			name:     "numeric comparison fail",
			cond:     spec.ConditionDecl{Script: "total > 100", Message: "total too small"},
			data:     map[string]any{"total": float64(50)},
			wantPass: false,
		},
		{
			name:     "multi-condition",
			cond:     spec.ConditionDecl{Script: "total > 0 and status == 'draft'"},
			data:     map[string]any{"total": float64(100), "status": "draft"},
			wantPass: true,
		},
		{
			name:     "resource access",
			cond:     spec.ConditionDecl{Script: "resource['is_blacklisted'] != True", Message: "blocked"},
			data:     map[string]any{"is_blacklisted": false, "resource": map[string]any{"is_blacklisted": false}},
			wantPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EvaluateConditions([]spec.ConditionDecl{tt.cond}, tt.data, nil)
			passed := err == nil
			if passed != tt.wantPass {
				t.Errorf("EvaluateConditions: want pass=%v, got pass=%v (err=%v)", tt.wantPass, passed, err)
			}
		})
	}
}
