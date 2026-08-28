package starlark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestCtxRequestID proves the correlation request ID propagates from the
// HTTP boundary into Starlark as ctx.request_id (todo 8.2.3, spec
// platform/09-observability.md §2.3).
func TestCtxRequestID(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "rid.star")
	script := "def execute(resource, params, ctx):\n    return ok({\"rid\": ctx.request_id})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	// With a request ID in the Go context → visible to the script.
	res, err := ExecuteScript(context.Background(), scriptPath,
		NewResourceAPI("m", "e", "id-1", 1, map[string]any{}), map[string]any{},
		newTestCtxWithRequestID("req-xyz-42"))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Data["rid"] != "req-xyz-42" {
		t.Errorf("ctx.request_id = %v, want req-xyz-42", res.Data["rid"])
	}

	// Outside a request context (REPL, scheduler) → empty string, not error.
	res, err = ExecuteScript(context.Background(), scriptPath,
		NewResourceAPI("m", "e", "id-1", 1, map[string]any{}), map[string]any{},
		newTestCtxWithRequestID(""))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Data["rid"] != "" {
		t.Errorf("ctx.request_id outside request = %v, want empty string", res.Data["rid"])
	}
}

// newTestCtxWithRequestID builds a CtxAPI with the given request ID —
// mirroring how the executor threads it from the Go context.
func newTestCtxWithRequestID(rid string) *CtxAPI {
	c := NewCtxAPI("demo", "", "user", "", nil)
	c.RequestID = rid
	return c
}
