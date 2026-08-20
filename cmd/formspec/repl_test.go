package main

import (
	"testing"

	"go.starlark.net/starlark"

	fsstarlark "github.com/primadi/formspec/internal/starlark"
)

// TestReplEvalBasic verifies the one-shot eval path evaluates a simple
// expression against the predeclared globals.
func TestReplEvalBasic(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	globals := starlark.StringDict{
		"ctx":      fsstarlark.NewCtxAPI("demo", "", "repl", "", nil),
		"resource": fsstarlark.NewResourceAPI("", "", "", 0, map[string]any{}),
	}
	if err := replEval(thread, globals, "x = 1 + 2"); err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if got := globals["x"]; got != starlark.MakeInt(3) {
		t.Fatalf("expected x=3, got %v", got)
	}
}

// TestReplEvalCtx verifies ctx.* is reachable from the console.
func TestReplEvalCtx(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	ctxObj := fsstarlark.NewCtxAPI("demo", "", "repl", "", nil)
	ctxObj.Config = fsstarlark.NewConfigAPI(map[string]any{"currency": "IDR"})
	globals := starlark.StringDict{
		"ctx":      ctxObj,
		"resource": fsstarlark.NewResourceAPI("", "", "", 0, map[string]any{}),
	}
	if err := replEval(thread, globals, "cur = ctx.config.get('currency')"); err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if got := globals["cur"]; got != starlark.String("IDR") {
		t.Fatalf("expected cur=IDR, got %v", got)
	}
}

// TestReplEvalError verifies a bad expression returns an error.
func TestReplEvalError(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	globals := starlark.StringDict{}
	if err := replEval(thread, globals, "this is not valid starlark !!!"); err == nil {
		t.Fatal("expected error for invalid expression")
	}
}
