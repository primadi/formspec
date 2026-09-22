package starlark

import "testing"

// fail(msg) must ABORT the script, not merely return a value. A returning
// builtin let execution continue on the next statement, so a guard-style
// `if bad: fail(...)` fell through: the script kept running with its data still
// unset, created a half-built record, and reported ok(). The contract
// (06-script-runtime.md §1) has always been "the entrypoint returns ok() or
// fail(msg)".
func TestFailAbortsScript(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    fail(\"insufficient account settings\")\n"+
		"    resource.create(\"gl.journal-entry\", {\"marker\": \"execution_continued\"})\n"+
		"    return ok({\"reached\": \"after_fail\"})\n")

	created := false
	res := NewResourceAPI("gl", "journal-entry", "", 0, map[string]any{})
	res.SetCreateFunc(func(module, entity string, data map[string]any) (string, error) {
		created = true
		return "should-not-exist", nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(t.Context(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript returned a transport error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.OK {
		t.Error("script reported ok() despite fail()")
	}
	if result.Error != "insufficient account settings" {
		t.Errorf("Error = %q, want the author's own fail() message verbatim", result.Error)
	}
	if created {
		t.Error("resource.create ran AFTER fail() — fail() did not abort the script")
	}
}

// fail() inside a helper function must unwind to the top too — this is how the
// journalize script reports incomplete GL account settings.
func TestFailAbortsFromNestedHelper(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def guard():\n"+
		"    fail(\"nested failure\")\n"+
		"\n"+
		"def execute(resource, params, ctx):\n"+
		"    guard()\n"+
		"    resource.create(\"gl.journal-entry\", {\"marker\": \"continued\"})\n"+
		"    return ok({})\n")

	created := false
	res := NewResourceAPI("gl", "journal-entry", "", 0, map[string]any{})
	res.SetCreateFunc(func(module, entity string, data map[string]any) (string, error) {
		created = true
		return "id", nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(t.Context(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript returned a transport error: %v", err)
	}
	if result.OK {
		t.Error("script reported ok() despite fail() in a helper")
	}
	if result.Error != "nested failure" {
		t.Errorf("Error = %q, want \"nested failure\"", result.Error)
	}
	if created {
		t.Error("execution continued past fail() called from a helper")
	}
}
