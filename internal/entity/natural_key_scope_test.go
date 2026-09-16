package entity

import (
	"context"
	"strings"
	"testing"
)

// Gap #9: a `natural_key_rule` with `scope_field` must produce a sequence per
// scope — kafe's order number restarts at 1 in every branch. That held on the
// automatic on-create path, but `ctx.next_key()` from a script passed an empty
// scope, so a scripted number quietly fell back to one global sequence: correct
// looking until two branches collided.
//
// Both halves of the fix are pinned here: the scope reaches the counter, and a
// scoped counter refuses to mint without one.
func TestGenerateNaturalKey_ScopedPerBranch(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/scoped-counter/spec")
	defer d.Close()
	if errs := reg.LoadEntities(); len(errs) > 0 {
		t.Fatalf("load entities: %v", errs)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}

	ctx := context.Background()
	for _, c := range []struct{ branch, want string }{
		{"B1", "INV-00001"},
		{"B1", "INV-00002"},
		{"B2", "INV-00001"}, // the second branch starts its own sequence
		{"B1", "INV-00003"},
	} {
		got, err := reg.GenerateNaturalKey(ctx, "kafe", "billing", "invoice", "number", c.branch)
		if err != nil {
			t.Fatalf("branch %s: %v", c.branch, err)
		}
		if got != c.want {
			t.Errorf("branch %s: key = %q, want %q — a single global counter would drift here", c.branch, got, c.want)
		}
	}
}

// A scoped counter with no scope must fail loudly: silently starting a global
// sequence is the failure this item removes, and it would only show up much later
// as a duplicate number.
func TestGenerateNaturalKey_ScopedCounterRefusesEmptyScope(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/scoped-counter/spec")
	defer d.Close()
	if errs := reg.LoadEntities(); len(errs) > 0 {
		t.Fatalf("load entities: %v", errs)
	}

	_, err := reg.GenerateNaturalKey(context.Background(), "kafe", "billing", "invoice", "number", "")
	if err == nil {
		t.Fatal("expected an error for a scoped counter without a scope value")
	}
	// The message must say what to pass, not merely refuse.
	for _, want := range []string{"branch_id", "ctx.next_key", "number"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q so the fix is obvious", err.Error(), want)
		}
	}
}

// An unscoped counter keeps working with an empty scope: the guard is about rules
// that declare one, not about the call shape.
func TestGenerateNaturalKey_UnscopedUnaffected(t *testing.T) {
	reg, d := setupTestRegistry(t, "registry_fixtures/scoped-counter/spec")
	defer d.Close()
	if errs := reg.LoadEntities(); len(errs) > 0 {
		t.Fatalf("load entities: %v", errs)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}

	got, err := reg.GenerateNaturalKey(context.Background(), "kafe", "billing", "menu-item", "code", "")
	if err != nil {
		t.Fatalf("unscoped counter: %v", err)
	}
	if got != "MI-00001" {
		t.Fatalf("key = %q, want MI-00001", got)
	}
}
