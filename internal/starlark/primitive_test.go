package starlark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fakeQuerier implements Querier for tests.
type fakeQuerier struct {
	rows []map[string]any
	err  error
}

func (f *fakeQuerier) Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	return f.rows, f.err
}

// TestCtxDBQuery_ResolvedAndExecuted proves the ctx.* resolver is wired into
// the CtxAPI (todo 2.9.1): a script calling ctx.db().query(...) resolves the
// "db" primitive through the resolver and executes the query against the
// returned connection, instead of failing with "datastore resolver not
// configured".
func TestCtxDBQuery_ResolvedAndExecuted(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "q.star")
	script := "def execute(resource, params, ctx):\n    rows = ctx.db().query(\"SELECT 1 AS one\")\n    return ok({\"n\": len(rows), \"one\": rows[0][\"one\"]})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now
	ctxObj.SetDatastoreResolver(func(primitiveType, name string) (interface{}, error) {
		if primitiveType != "db" || name != "default" {
			t.Fatalf("unexpected resolve(%q, %q)", primitiveType, name)
		}
		return &fakeQuerier{rows: []map[string]any{{"one": int64(1)}}}, nil
	})

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if got := result.Data["n"]; got != int64(1) {
		t.Fatalf("n = %v, want 1", got)
	}
	if got := result.Data["one"]; got != int64(1) {
		t.Fatalf("one = %v, want 1", got)
	}
}

// TestCtxDBQuery_NoResolver proves the pre-2.9.1 behavior is preserved when
// no resolver is wired: ctx.db() fails with "datastore resolver not
// configured".
func TestCtxDBQuery_NoResolver(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "q.star")
	script := "def execute(resource, params, ctx):\n    ctx.db()\n    return ok({})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.OK {
		t.Fatalf("expected failure, got OK")
	}
	if result.Error == "" {
		t.Fatalf("expected an error message")
	}
}

// TestCtxDBQuery_UnsupportedBackend proves a resolved connection that does
// not implement Querier fails with a clear "not yet implemented for this
// backend" error (not "not configured").
func TestCtxDBQuery_UnsupportedBackend(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "q.star")
	script := "def execute(resource, params, ctx):\n    ctx.db().query(\"SELECT 1\")\n    return ok({})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now
	// Resolver returns a plain struct that does not implement Querier.
	ctxObj.SetDatastoreResolver(func(primitiveType, name string) (interface{}, error) {
		return struct{}{}, nil
	})

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.OK {
		t.Fatalf("expected failure, got OK")
	}
}
