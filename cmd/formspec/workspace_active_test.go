package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Gap #48: a `kind: Workspace` manifest registers a slug, it does not select
// one. The active workspace comes from `--workspace-id`. Declaring `kafe` and
// running dev used to store everything under the `default` tenant while
// `GET /kafe/...` answered 200 with zero rows — healthy looking, and different
// from what the manifest implies.
func TestResolveActiveWorkspace(t *testing.T) {
	// The real kafe tree declares exactly one workspace.
	t.Run("single declared workspace is adopted when the flag is absent", func(t *testing.T) {
		cfg := resolveActiveWorkspace(DevConfig{
			SpecPath:    "../../examples/kafe/spec",
			WorkspaceID: "default",
		})
		if cfg.WorkspaceID != "kafe" {
			t.Fatalf("workspace = %q, want kafe (the only declared one)", cfg.WorkspaceID)
		}
	})

	t.Run("explicit flag wins", func(t *testing.T) {
		cfg := resolveActiveWorkspace(DevConfig{
			SpecPath:            "../../examples/kafe/spec",
			WorkspaceID:         "staging",
			WorkspaceIDExplicit: true,
		})
		if cfg.WorkspaceID != "staging" {
			t.Fatalf("explicit workspace must not be overridden, got %q", cfg.WorkspaceID)
		}
	})

	t.Run("tree without workspaces keeps the default", func(t *testing.T) {
		cfg := resolveActiveWorkspace(DevConfig{
			SpecPath:    t.TempDir(),
			WorkspaceID: "default",
		})
		if cfg.WorkspaceID != "default" {
			t.Fatalf("workspace = %q, want default", cfg.WorkspaceID)
		}
	})

	t.Run("several declared workspaces leave the default alone", func(t *testing.T) {
		dir := t.TempDir()
		writeWorkspace(t, dir, "kafe")
		writeWorkspace(t, dir, "barbershop")
		cfg := resolveActiveWorkspace(DevConfig{SpecPath: dir, WorkspaceID: "default"})
		if cfg.WorkspaceID != "default" {
			t.Fatalf("with several declared, the default must stand (warning instead), got %q", cfg.WorkspaceID)
		}
		if got := declaredWorkspaces(dir); len(got) != 2 || got[0] != "barbershop" || got[1] != "kafe" {
			t.Fatalf("declaredWorkspaces = %v, want sorted [barbershop kafe]", got)
		}
	})
}

func writeWorkspace(t *testing.T, specPath, slug string) {
	t.Helper()
	dir := filepath.Join(specPath, "workspaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "apiVersion: formspec.dev/v1\nkind: Workspace\nmetadata:\n  name: " + slug + "\n  description: \"test\"\nspec:\n  isolation: shared\n"
	if err := os.WriteFile(filepath.Join(dir, slug+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
