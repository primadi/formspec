package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunInit_ScaffoldsProject verifies `formspec init` produces the standard
// layout, extracts embedded AI skills into .agents/skills/, writes the JSON
// Schema + .vscode settings, and emits a copilot-instructions.md that
// references the 4-phase workflow and the `formspec validate` gate.
func TestRunInit_ScaffoldsProject(t *testing.T) {
	// runInit builds the project as a subdirectory of the current working
	// directory, so run it from an isolated temp dir and restore afterwards.
	work := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	runInit([]string{"--force", "testapp"})

	root := filepath.Join(work, "testapp")
	assertExists := func(rel string) {
		t.Helper()
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}

	// Standard layout
	assertExists("formspec-app.yaml")
	assertExists(".gitignore")
	assertExists(".github/copilot-instructions.md")
	assertExists(".vscode/settings.json")
	assertExists("spec/apps")
	assertExists("spec/modules")

	// Embedded AI skills extracted into .agents/skills/<name>/SKILL.md
	skillsDir := filepath.Join(root, ".agents", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("read .agents/skills: %v", err)
	}
	var skillNames []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(skillsDir, e.Name(), "SKILL.md")); err != nil {
			t.Fatalf("skill %s missing SKILL.md: %v", e.Name(), err)
		}
		skillNames = append(skillNames, e.Name())
	}
	if len(skillNames) < 4 {
		t.Fatalf("expected at least 4 skills, got %d: %v", len(skillNames), skillNames)
	}

	// JSON Schema extracted
	schemaEntries, err := os.ReadDir(filepath.Join(root, "schemas"))
	if err != nil || len(schemaEntries) == 0 {
		t.Fatalf("expected non-empty schemas/ (err=%v)", err)
	}

	// copilot-instructions references the workflow + validation gate
	ci, err := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(ci)
	for _, want := range []string{
		"formspec-app-workflow",
		"formspec validate --spec spec",
		"Discovery",
		"Draft",
		"Iterate",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("copilot-instructions.md missing %q", want)
		}
	}
}
