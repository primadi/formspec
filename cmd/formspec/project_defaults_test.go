package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadProjectDefaults_ReadsConfigFile locks the parity bug fixed on
// 2026-09-29: every data-lifecycle command carried its own literals
// (`spec`, `sqlite:.formspec/data.db`, workspace "demo"), which are the values
// `formspec dev` uses only when formspec-app.yaml is ABSENT. In a project that
// has the config file, the command therefore opened a different database than
// the server it was supposed to be helping.
//
// The assertion is on the resolved triple, and the config deliberately differs
// from every fallback so a regression cannot pass by coincidence.
func TestLoadProjectDefaults_ReadsConfigFile(t *testing.T) {
	dir := t.TempDir()
	writeProjectConfig(t, dir, `spec: my-spec
dsn: sqlite:.formspec/custom.db
workspace-id: acme
`)
	chdir(t, dir)

	d := loadProjectDefaults()
	if d.SpecPath != "my-spec" {
		t.Errorf("SpecPath = %q, want my-spec (from the config file, not the fallback)", d.SpecPath)
	}
	if d.DSN != "sqlite:.formspec/custom.db" {
		t.Errorf("DSN = %q, want sqlite:.formspec/custom.db", d.DSN)
	}
	if d.WorkspaceID != "acme" {
		t.Errorf("WorkspaceID = %q, want acme", d.WorkspaceID)
	}
	if !d.WorkspaceExplicit {
		t.Error("WorkspaceExplicit = false, want true: a workspace named in the config file is a choice, not a fallback")
	}
}

// TestLoadProjectDefaults_NoConfigKeepsFallbacks pins the no-config behaviour:
// the historical literals are still the answer when there is nothing to read,
// so this change does not move the ground under projects without a config file.
func TestLoadProjectDefaults_NoConfigKeepsFallbacks(t *testing.T) {
	chdir(t, t.TempDir())

	d := loadProjectDefaults()
	if d.SpecPath != "spec" || d.DSN != "sqlite:.formspec/data.db" {
		t.Errorf("defaults without a config file = (%q, %q), want (spec, sqlite:.formspec/data.db)", d.SpecPath, d.DSN)
	}
	if d.WorkspaceExplicit {
		t.Error("WorkspaceExplicit = true with no config file: nothing chose the workspace")
	}
}

// TestFinishProjectDefaults_FlagBeatsConfig is the rule stated in the docs:
// the config file supplies DEFAULTS, a flag WINS. A command-line `--dsn` that
// still lost to the file would be the same class of bug in the other direction.
func TestFinishProjectDefaults_FlagBeatsConfig(t *testing.T) {
	dir := t.TempDir()
	writeProjectConfig(t, dir, "spec: spec\ndsn: sqlite:.formspec/from-config.db\n")
	chdir(t, dir)

	// The caller passed --dsn, so it hands over its own value and does not call
	// loadProjectDefaults for it.
	_, dsn, _ := finishProjectDefaults("spec", "sqlite:.formspec/from-flag.db", "default", true)
	if want := filepath.Join(dir, ".formspec", "from-flag.db"); dsn != "sqlite:"+want {
		t.Errorf("DSN = %q, want sqlite:%s — the flag must win over the config file", dsn, want)
	}
}

// TestFinishProjectDefaults_AnchorsDSNAndAdoptsWorkspace ties the two steps to
// the behaviour that made the repair loop fail: the DSN has to be anchored to
// the project root, and a spec tree declaring exactly one workspace has to give
// that workspace to the command — otherwise the console and the migration write
// under a tenant the app never reads.
func TestFinishProjectDefaults_AnchorsDSNAndAdoptsWorkspace(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec")
	writeWorkspaceManifest(t, specPath, "kafe")

	_, dsn, ws := finishProjectDefaults(specPath, "sqlite:.formspec/kafe.db", "default", false)
	if want := filepath.Join(dir, ".formspec", "kafe.db"); dsn != "sqlite:"+want {
		t.Errorf("DSN = %q, want sqlite:%s (anchored to the project root)", dsn, want)
	}
	if ws != "kafe" {
		t.Errorf("workspace = %q, want kafe (the only one declared, so it is adopted)", ws)
	}

	// An explicit choice always wins over adoption.
	_, _, explicit := finishProjectDefaults(specPath, "sqlite:.formspec/kafe.db", "staging", true)
	if explicit != "staging" {
		t.Errorf("explicit workspace = %q, want staging", explicit)
	}
}

// ─── helpers ───

func writeProjectConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "formspec-app.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeWorkspaceManifest(t *testing.T, specDir, slug string) {
	t.Helper()
	p := filepath.Join(specDir, "workspaces", slug+".yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir workspaces: %v", err)
	}
	body := "apiVersion: formspec.dev/v1\nkind: Workspace\nmetadata:\n  name: " + slug + "\nspec: {}\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write workspace manifest: %v", err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}
