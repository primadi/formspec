package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.starlark.net/starlark"

	fsstarlark "github.com/primadi/formspec/internal/starlark"
)

// TestReplExecFile_RunsEveryStatement locks the bug where `formspec repl -f`
// silently executed almost nothing (2026-09-28).
//
// The `-f` path went through replEval, i.e. syntax.ParseCompoundStmt — the
// REPL's modal parser, which stops at the end of the FIRST compound statement
// and treats a blank line as end-of-input. For a console prompt that is correct
// behaviour; for a script file it means the repair stops after one statement,
// and a file that begins with a comment executes nothing at all. Nothing failed:
// exit status was 0 and the console printed "Ran <file>.", so the documented
// data-repair surface reported success while leaving the data untouched.
//
// The test is deliberately shaped like a real repair script (leading comment,
// blank lines, several statements) because that is the input the old parser
// choked on — a one-line script would have passed either way.
func TestReplExecFile_RunsEveryStatement(t *testing.T) {
	var ran []string
	record := starlark.NewBuiltin("record", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		s, ok := starlark.AsString(args[0])
		if !ok {
			return nil, fmt.Errorf("record: want a string, got %s", args[0].Type())
		}
		ran = append(ran, s)
		return starlark.None, nil
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "repair.star")
	script := `# One-off data repair.
#
# Leading comments and blank lines are the normal shape of a repair script.

record("first")

record("second")
record("third")
`
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	thread := &starlark.Thread{Name: "test"}
	predeclared := starlark.StringDict{
		"ctx":      fsstarlark.NewCtxAPI("demo", "", "repl", "", nil),
		"resource": fsstarlark.NewResourceAPI("", "", "", 0, map[string]any{}),
		"record":   record,
	}
	if err := replExecFile(thread, predeclared, path, script); err != nil {
		t.Fatalf("exec file: %v", err)
	}

	want := []string{"first", "second", "third"}
	if len(ran) != len(want) {
		t.Fatalf("statements executed: got %v (%d), want %v (%d)", ran, len(ran), want, len(want))
	}
	for i := range want {
		if ran[i] != want[i] {
			t.Fatalf("statement %d: got %q, want %q (full: %v)", i, ran[i], want[i], ran)
		}
	}
}

// TestReplExecFile_PropagatesError verifies a failing statement stops the run
// with a non-nil error — the file path must not exit 0 on a broken repair.
func TestReplExecFile_PropagatesError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.star")
	script := "x = 1\nfail_now = undefined_name\n"
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	thread := &starlark.Thread{Name: "test"}
	err := replExecFile(thread, starlark.StringDict{}, path, script)
	if err == nil {
		t.Fatal("expected an error for an undefined name, got nil (a broken repair would report success)")
	}
}
