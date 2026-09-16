package starlark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// `ctx.next_key` from a script (gap #9). The counter can be scoped by a field
// (kafe: the order number restarts per branch), and the script is the caller that
// knows which scope it is minting for — so the scope travels as an argument, and
// the backing handler must receive it. Before this, the script path passed an
// empty scope and a scoped counter quietly produced one global sequence.
func TestCtxNextKey_ScopeReachesHandler(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "key.star")
	script := "def execute(resource, params, ctx):\n" +
		"    scoped = ctx.next_key(\"number\", scope=\"B1\")\n" +
		"    unscoped = ctx.next_key(\"code\")\n" +
		"    return ok({\"scoped\": scoped, \"unscoped\": unscoped})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotField, gotScope string
	ctxObj := NewCtxAPI("kafe", "", "kasir1", "", nil)
	ctxObj.Now = now
	ctxObj.NextKey = func(fieldName, scope string) (string, error) {
		gotField, gotScope = fieldName, scope
		if scope == "" {
			return "MI-00001", nil
		}
		return "ORD-00001", nil
	}

	res := NewResourceAPI("cafe-order", "order", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if gotField != "code" || gotScope != "" {
		t.Errorf("second call: field=%q scope=%q, want the unscoped call to pass an empty scope", gotField, gotScope)
	}
	if result.Data["scoped"] != "ORD-00001" || result.Data["unscoped"] != "MI-00001" {
		t.Fatalf("unexpected keys: %#v", result.Data)
	}
}

// The scoped call must arrive with its scope; a handler that never sees it cannot
// scope anything, which was the whole bug.
func TestCtxNextKey_ScopedArgumentIsForwarded(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "key.star")
	script := "def execute(resource, params, ctx):\n" +
		"    return ok({\"key\": ctx.next_key(\"number\", scope=\"B7\")})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	seen := ""
	ctxObj := NewCtxAPI("kafe", "", "kasir1", "", nil)
	ctxObj.Now = now
	ctxObj.NextKey = func(fieldName, scope string) (string, error) {
		seen = scope
		return "ORD-00042", nil
	}

	res := NewResourceAPI("cafe-order", "order", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil || !result.OK {
		t.Fatalf("execute: %v / %v", err, result.Error)
	}
	if seen != "B7" {
		t.Fatalf("handler saw scope %q, want B7", seen)
	}
	if result.Data["key"] != "ORD-00042" {
		t.Fatalf("key = %v", result.Data["key"])
	}
}
