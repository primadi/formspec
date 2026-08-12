package action

import (
	"context"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestNativeExecutor_RegisterAndExecute(t *testing.T) {
	ex := NewNativeExecutor()

	ex.Register("test.thing.do-stuff", func(ctx context.Context, params ExecuteParams) (any, error) {
		return map[string]any{"result": "success"}, nil
	})

	result, err := ex.Execute(context.TODO(), spec.Action{
		Name: "do-stuff",
		Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "test.thing.do-stuff"},
	}, ExecuteParams{
		Module:     "test",
		Entity:     "thing",
		ActionName: "do-stuff",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected data in result")
	}
}

func TestNativeExecutor_ResolveByTypeName(t *testing.T) {
	ex := NewNativeExecutor()

	ex.Register("billing.OrderResource.UpdateDiscountRule", func(ctx context.Context, params ExecuteParams) (any, error) {
		return map[string]any{"discount": "updated"}, nil
	})

	result, err := ex.Execute(context.TODO(), spec.Action{
		Name: "update-discount-rule",
		Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "OrderResource.UpdateDiscountRule"},
	}, ExecuteParams{
		Module:     "billing",
		Entity:     "order",
		ActionName: "update-discount-rule",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.Data.(map[string]any)
	if !ok || m["discount"] != "updated" {
		t.Fatalf("unexpected result: %v", result.Data)
	}
}

func TestNativeExecutor_NotRegistered(t *testing.T) {
	ex := NewNativeExecutor()

	_, err := ex.Execute(context.TODO(), spec.Action{
		Name: "missing",
		Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "Missing.Method"},
	}, ExecuteParams{
		Module:     "test",
		ActionName: "missing",
	})

	if err == nil {
		t.Fatal("expected error for unregistered handler")
	}
}

func TestNativeExecutor_DuplicatePanics(t *testing.T) {
	ex := NewNativeExecutor()

	ex.Register("test.key", func(ctx context.Context, params ExecuteParams) (any, error) { return nil, nil })

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	ex.Register("test.key", func(ctx context.Context, params ExecuteParams) (any, error) { return nil, nil })
}

func TestSidecarExecutor_NotImplemented(t *testing.T) {
	ex := NewSidecarExecutor()

	_, err := ex.Execute(context.TODO(), spec.Action{
		Name: "sidecar-action",
		Impl: &spec.ImplDecl{Type: spec.ImplSidecar, Ref: "some-sidecar"},
	}, ExecuteParams{
		Module:     "test",
		ActionName: "sidecar-action",
	})

	if err == nil {
		t.Fatal("expected error for sidecar executor")
	}
}
