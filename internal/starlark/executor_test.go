package starlark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestExecuteScript_CompileCache_NoCrossCallStateBleed is the regression test
// for the compiled-program cache (internal/starlark/cache.go): the same
// script, invoked many times concurrently with different resource data, must
// never see another call's data. This is exactly the bug that caching the
// globals StringDict (instead of the compiled *Program) would introduce —
// Init() must produce a fresh globals dict per call even though the
// compiled Program itself is shared.
func TestExecuteScript_CompileCache_NoCrossCallStateBleed(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "echo.star")
	script := "def execute(resource, params, ctx):\n    return ok({\"seen\": resource.field.value})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	const n = 50
	var wg sync.WaitGroup
	errs := make([]error, n)
	mismatches := make([]bool, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			want := fmt.Sprintf("call-%d", i)
			res := NewResourceAPI("clinic", "visit", fmt.Sprintf("id-%d", i), 1, map[string]any{"value": want})
			ctxObj := NewCtxAPI("demo", "", "user", "", nil)
			ctxObj.Now = now

			result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
			if err != nil {
				errs[i] = err
				return
			}
			if !result.OK {
				errs[i] = fmt.Errorf("script failed: %s", result.Error)
				return
			}
			if got := result.Data["seen"]; got != want {
				mismatches[i] = true
				errs[i] = fmt.Errorf("call %d: got %v, want %v", i, got, want)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
}

// TestExecuteScript_CompileCache_InvalidatesOnEdit proves the cache is keyed
// by mtime, not just path — editing a script (as happens during dev hot
// editing) must be picked up without a process restart.
func TestExecuteScript_CompileCache_InvalidatesOnEdit(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "versioned.star")

	write := func(value string) {
		script := fmt.Sprintf("def execute(resource, params, ctx):\n    return ok({\"v\": %q})\n", value)
		if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
		// Ensure a distinct mtime — some filesystems have coarse mtime
		// resolution, which would make the cache (correctly) treat a
		// same-second edit as unchanged.
		future := time.Now().Add(time.Duration(len(value)+1) * time.Second)
		if err := os.Chtimes(scriptPath, future, future); err != nil {
			t.Fatal(err)
		}
	}

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})
	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	write("v1")
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil || !result.OK {
		t.Fatalf("first exec: result=%+v err=%v", result, err)
	}
	if got := result.Data["v"]; got != "v1" {
		t.Fatalf("first exec: got %v, want v1", got)
	}

	write("v2")
	result, err = ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil || !result.OK {
		t.Fatalf("second exec: result=%+v err=%v", result, err)
	}
	if got := result.Data["v"]; got != "v2" {
		t.Fatalf("second exec after edit: got %v, want v2 (cache did not invalidate on mtime change)", got)
	}
}
