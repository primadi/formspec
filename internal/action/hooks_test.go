package action

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestSelectHooks_PriorityOrdering(t *testing.T) {
	hooks := []spec.HookDecl{
		{On: spec.HookOnBefore, Action: "create", Priority: 20, Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "c"}},
		{On: spec.HookOnBefore, Action: "create", Priority: 5, Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "a"}},
		{On: spec.HookOnBefore, Action: "*", Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "b-default"}}, // no priority → default 10
		{On: spec.HookOnAfter, Action: "create", Priority: 1, Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "wrong-timing"}},
		{On: spec.HookOnBefore, Action: "update", Priority: 1, Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "wrong-action"}},
	}

	got := SelectHooks(hooks, spec.HookOnBefore, "create")
	if len(got) != 3 {
		t.Fatalf("got %d hooks, want 3: %+v", len(got), got)
	}
	wantOrder := []string{"a", "b-default", "c"}
	for i, w := range wantOrder {
		if got[i].Impl.Ref != w {
			t.Errorf("position %d: got ref %q, want %q", i, got[i].Impl.Ref, w)
		}
	}
}

// setupDispatcher builds a real Dispatcher wired to a real ScriptExecutor
// over a temp spec directory, writing each named script's body first — this
// exercises RunBeforePhase/RunAfterPhase through the same code path
// production uses (Dispatcher.Dispatch → ScriptExecutor → Starlark engine),
// not a fake. Script keys are "module/name" refs (e.g. "test/append_a"),
// mirroring the module-scoped scripts/ layout resolveScriptPath resolves
// against first (modules/{module}/scripts/{name}.star), same as production
// refs like "clinic/visit_complete".
func setupDispatcher(t *testing.T, scripts map[string]string) *Dispatcher {
	t.Helper()
	base := t.TempDir()
	for ref, body := range scripts {
		parts := strings.SplitN(ref, "/", 2)
		module, name := parts[0], parts[1]
		dir := filepath.Join(base, "modules", module, "scripts")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".star"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	disp := NewDispatcher()
	disp.RegisterExecutor(spec.ImplScriptRef, NewScriptExecutor(base))
	return disp
}

func appendScript(letter string) string {
	return "def execute(resource, params, ctx):\n" +
		"    trace = resource.field.trace\n" +
		"    if trace == None:\n" +
		"        trace = \"\"\n" +
		"    resource.set(\"trace\", trace + \"" + letter + "\")\n" +
		"    return ok()\n"
}

func TestRunBeforePhase_PriorityOrder_ThenActionImplLast(t *testing.T) {
	disp := setupDispatcher(t, map[string]string{
		"test/append_a": appendScript("A"),
		"test/append_b": appendScript("B"),
		"test/append_c": appendScript("C"),
	})

	hooks := []spec.HookDecl{
		{On: spec.HookOnBefore, Action: "create", Priority: 20, Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/append_b"}},
		{On: spec.HookOnBefore, Action: "create", Priority: 5, Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/append_a"}},
	}
	actionSpec := &spec.Action{Name: "create", Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/append_c"}}

	params := &ExecuteParams{Module: "pharmacy", Entity: "prescription", ActionName: "create", Resource: map[string]any{}}
	if err := RunBeforePhase(context.Background(), disp, hooks, actionSpec, "create", params); err != nil {
		t.Fatalf("RunBeforePhase: %v", err)
	}

	if got := params.Resource["trace"]; got != "ABC" {
		t.Errorf("trace = %v, want \"ABC\" (priority 5 hook, then priority 20 hook, then action's own impl last)", got)
	}
}

func TestRunBeforePhase_Abort_StopsRemainingHooksAndImpl(t *testing.T) {
	disp := setupDispatcher(t, map[string]string{
		"test/fail_hook":   "def execute(resource, params, ctx):\n    return fail(\"insufficient stock\")\n",
		"test/append_b":    appendScript("B"),
		"test/should_skip": appendScript("SKIPPED"),
	})

	hooks := []spec.HookDecl{
		{On: spec.HookOnBefore, Action: "create", Priority: 5, Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/fail_hook"}},
		{On: spec.HookOnBefore, Action: "create", Priority: 10, Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/append_b"}},
	}
	actionSpec := &spec.Action{Name: "create", Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/should_skip"}}

	params := &ExecuteParams{Module: "pharmacy", Entity: "otc-sale", ActionName: "create", Resource: map[string]any{}}
	err := RunBeforePhase(context.Background(), disp, hooks, actionSpec, "create", params)
	if err == nil {
		t.Fatal("expected RunBeforePhase to return an error when a before hook fails")
	}
	if !strings.Contains(err.Error(), "insufficient stock") {
		t.Errorf("error = %v, want it to contain \"insufficient stock\"", err)
	}
	if _, ok := params.Resource["trace"]; ok {
		t.Errorf("resource.trace = %v, want unset — no hook/impl after the failing one should have run", params.Resource["trace"])
	}
}

func TestRunAfterPhase_RunsBestEffort(t *testing.T) {
	disp := setupDispatcher(t, map[string]string{
		"test/append_after": appendScript("AFTER"),
	})
	hooks := []spec.HookDecl{
		{On: spec.HookOnAfter, Action: "create", Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/append_after"}},
	}
	params := ExecuteParams{Module: "pharmacy", Entity: "prescription", ActionName: "create", Resource: map[string]any{}}
	RunAfterPhase(context.Background(), disp, hooks, nil, "create", params)
	if got := params.Resource["trace"]; got != "AFTER" {
		t.Errorf("trace = %v, want \"AFTER\"", got)
	}
}

func TestRunOnErrorPhase_FiresOnBeforeHookFailure(t *testing.T) {
	disp := setupDispatcher(t, map[string]string{
		"test/fail_hook": "def execute(resource, params, ctx):\n    return fail(\"boom\")\n",
		"test/on_error":  "def execute(resource, params, ctx):\n    resource.set(\"saw_error\", params[\"_hook_error\"])\n    return ok()\n",
	})
	hooks := []spec.HookDecl{
		{On: spec.HookOnBefore, Action: "create", Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/fail_hook"}},
		{On: spec.HookOnError, Action: "create", Impl: &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "test/on_error"}},
	}
	params := &ExecuteParams{Module: "pharmacy", Entity: "prescription", ActionName: "create", Resource: map[string]any{}}
	err := RunBeforePhase(context.Background(), disp, hooks, nil, "create", params)
	if err == nil {
		t.Fatal("expected an error")
	}
	sawErr, _ := params.Resource["saw_error"].(string)
	if !strings.Contains(sawErr, "boom") {
		t.Errorf("on_error hook saw _hook_error = %q, want it to contain \"boom\"", sawErr)
	}
}
