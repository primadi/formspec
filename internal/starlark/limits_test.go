package starlark

import (
	"os"
	"strings"
	"testing"
)

// TestScriptLimits_QueryLimit verifies the max-DB-queries limit (7.14.1).
func TestScriptLimits_QueryLimit(t *testing.T) {
	l := NewScriptLimits()
	l.maxQueries = 2

	if err := l.CheckQuery(); err != nil {
		t.Fatalf("first query should be allowed: %v", err)
	}
	if err := l.CheckQuery(); err != nil {
		t.Fatalf("second query should be allowed: %v", err)
	}
	if err := l.CheckQuery(); err == nil {
		t.Fatal("third query should be blocked")
	}
}

// TestScriptLimits_RecordsLimit verifies the max-records-read limit (7.14.1).
func TestScriptLimits_RecordsLimit(t *testing.T) {
	l := NewScriptLimits()
	l.maxRecords = 100

	if err := l.AddRecordsRead(60); err != nil {
		t.Fatalf("60 records should be allowed: %v", err)
	}
	if err := l.AddRecordsRead(40); err != nil {
		t.Fatalf("cumulative 100 should be allowed: %v", err)
	}
	if err := l.AddRecordsRead(1); err == nil {
		t.Fatal("101st record should be blocked")
	}
}

// TestScriptLimits_NilReceiver verifies nil limits are a no-op (unit-test
// callers that don't set limits on the thread).
func TestScriptLimits_NilReceiver(t *testing.T) {
	var l *ScriptLimits
	if err := l.CheckQuery(); err != nil {
		t.Fatalf("nil limits should allow: %v", err)
	}
	if err := l.AddRecordsRead(10); err != nil {
		t.Fatalf("nil limits should allow records: %v", err)
	}
}

// TestExecuteScript_IterationLimit verifies the 100K step cap aborts a
// runaway loop (7.14.1).
func TestExecuteScript_IterationLimit(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/loop.star"
	script := `
def execute(resource, params, ctx):
    total = 0
    for i in range(100000000):
        total = total + i
    return ok()
`
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ctxObj := NewCtxAPI("demo", "demo", "u1", "admin", nil)
	res := NewResourceAPI("m", "e", "1", 0, nil)
	result, err := ExecuteScript(t.Context(), path, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript: %v", err)
	}
	if result.OK {
		t.Fatal("runaway loop should not succeed")
	}
	if !strings.Contains(result.Error, "too many") && !strings.Contains(result.Error, "execution") {
		t.Fatalf("expected step-limit error, got: %q", result.Error)
	}
}
