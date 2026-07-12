package action

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveScriptPath covers the two script-directory layouts used across
// examples: per-module "scripts/" subdirectories (Clinic, Inventory,
// General-Ledger, Order-to-Cash) and flat "spec/scripts/" (Customer,
// Midtrans-Payment-Gateway).
func TestResolveScriptPath(t *testing.T) {
	base := t.TempDir()

	moduleScriptsDir := filepath.Join(base, "modules", "clinic", "scripts")
	if err := os.MkdirAll(moduleScriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	moduleScriptPath := filepath.Join(moduleScriptsDir, "visit_complete.star")
	if err := os.WriteFile(moduleScriptPath, []byte("def execute(resource, params, ctx):\n    return ok()\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	flatScriptsDir := filepath.Join(base, "scripts")
	if err := os.MkdirAll(flatScriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	flatScriptPath := filepath.Join(flatScriptsDir, "order_checkout.star")
	if err := os.WriteFile(flatScriptPath, []byte("def execute(resource, params, ctx):\n    return ok()\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolve := resolveScriptPath(base)

	t.Run("module-scoped scripts subdirectory", func(t *testing.T) {
		got, err := resolve("clinic/visit_complete")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if got != moduleScriptPath {
			t.Errorf("got %q, want %q", got, moduleScriptPath)
		}
	})

	t.Run("flat top-level scripts directory fallback", func(t *testing.T) {
		got, err := resolve("billing/order_checkout")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if got != flatScriptPath {
			t.Errorf("got %q, want %q", got, flatScriptPath)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := resolve("clinic/does_not_exist")
		if err == nil {
			t.Fatal("expected error for missing script")
		}
	})
}
