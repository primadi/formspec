package action

import (
	"context"
	"errors"
	"testing"

	"github.com/primadi/forma/pkg/spec"
)

// testExecutor is a simple executor that returns a fixed result for testing.
type testExecutor struct {
	result *ExecuteResult
	err    error
}

func (e *testExecutor) Execute(_ context.Context, _ spec.Action, _ ExecuteParams) (*ExecuteResult, error) {
	return e.result, e.err
}

func TestNewDispatcher(t *testing.T) {
	d := NewDispatcher()
	if d == nil {
		t.Fatal("NewDispatcher returned nil")
	}
	if d.executors == nil {
		t.Fatal("executors map is nil")
	}
}

func TestRegisterExecutor(t *testing.T) {
	d := NewDispatcher()
	ex := &testExecutor{result: &ExecuteResult{}}
	d.RegisterExecutor(spec.ImplNative, ex)

	if !d.HasExecutor(spec.ImplNative) {
		t.Fatal("expected executor for native to be registered")
	}
}

func TestRegisterExecutor_DuplicatePanics(t *testing.T) {
	d := NewDispatcher()
	d.RegisterExecutor(spec.ImplScriptRef, &testExecutor{})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	d.RegisterExecutor(spec.ImplScriptRef, &testExecutor{})
}

func TestHasExecutor_False(t *testing.T) {
	d := NewDispatcher()
	if d.HasExecutor(spec.ImplSidecar) {
		t.Fatal("expected HasExecutor to return false for unregistered type")
	}
}

func TestDispatch_NoImpl(t *testing.T) {
	d := NewDispatcher()

	result, err := d.Dispatch(context.Background(), spec.Action{
		Name: "some-action",
		Impl: nil,
	}, ExecuteParams{
		Module:     "test",
		ActionName: "some-action",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestDispatch_NoExecutor(t *testing.T) {
	d := NewDispatcher()

	_, err := d.Dispatch(context.Background(), spec.Action{
		Name: "some-action",
		Impl: &spec.ImplDecl{Type: spec.ImplSidecar},
	}, ExecuteParams{
		Module:     "test",
		ActionName: "some-action",
	})

	if err == nil {
		t.Fatal("expected error for unregistered executor")
	}
}

func TestDispatch_Success(t *testing.T) {
	d := NewDispatcher()
	d.RegisterExecutor(spec.ImplNative, &testExecutor{
		result: &ExecuteResult{Data: map[string]any{"ok": true}},
	})

	result, err := d.Dispatch(context.Background(), spec.Action{
		Name: "do-stuff",
		Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "Handler.DoStuff"},
	}, ExecuteParams{
		Module:     "test",
		Entity:     "thing",
		ActionName: "do-stuff",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Data == nil {
		t.Fatal("expected data in result")
	}
}

func TestDispatch_ExecutorError(t *testing.T) {
	d := NewDispatcher()
	d.RegisterExecutor(spec.ImplScriptRef, &testExecutor{
		err: errors.New("script execution failed"),
	})

	_, err := d.Dispatch(context.Background(), spec.Action{
		Name: "fail-action",
		Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/fail"},
	}, ExecuteParams{
		Module:     "test",
		ActionName: "fail-action",
	})

	if err == nil {
		t.Fatal("expected error from executor")
	}
}

func TestRuntimeContext_Defaults(t *testing.T) {
	rc := &RuntimeContext{
		Tenant: &TenantInfo{ID: "demo", Name: "Demo Workspace"},
		User:   &UserInfo{ID: "user-1", Role: "admin"},
	}

	if rc.Tenant.ID != "demo" {
		t.Errorf("expected tenant id 'demo', got %q", rc.Tenant.ID)
	}
	if rc.User.Role != "admin" {
		t.Errorf("expected role 'admin', got %q", rc.User.Role)
	}
}
