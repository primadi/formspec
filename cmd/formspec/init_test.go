package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunInit_ScaffoldsProject verifies `formspec init` produces the standard
// layout from the embedded template tree, extracts embedded AI skills into
// .agents/skills/, renders the module-named App / Workspace manifests, and
// emits AGENTS.md with the 4-phase workflow, App-shape guidance, and the
// published docs reference.
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
	assertExists("AGENTS.md")
	assertExists(".github/copilot-instructions.md")
	assertExists(".vscode/settings.json")
	assertExists("spec/apps")
	assertExists("spec/modules")
	assertExists("spec/workspaces")
	// Module-named manifests rendered from app.tmpl.yaml / ws.tmpl.yaml.
	assertExists(filepath.Join("spec", "apps", "testapp.yaml"))
	assertExists(filepath.Join("spec", "workspaces", "testapp.yaml"))
	// The App mounts module "testapp", so the scaffold MUST also declare that
	// Module — without it the App mounts nothing and the project fails its own
	// documented gate (`formspec validate --spec spec`). The module template's
	// _module_ directory placeholder resolves to the module name.
	assertExists(filepath.Join("spec", "modules", "testapp", "module.yaml"))
	moduleYAML, err := os.ReadFile(filepath.Join(root, "spec", "modules", "testapp", "module.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if mod := string(moduleYAML); !strings.Contains(mod, "kind: Module") ||
		!strings.Contains(mod, "name: testapp") {
		t.Fatalf("spec/modules/testapp/module.yaml not rendered as expected:\n%s", mod)
	}

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

	// App manifest is rendered from the template tree: module name substituted
	// and the App-shape hints present.
	appYAML, err := os.ReadFile(filepath.Join(root, "spec", "apps", "testapp.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if app := string(appYAML); !strings.Contains(app, "name: testapp") ||
		!strings.Contains(app, "app_renderer") || !strings.Contains(app, "access") {
		t.Fatalf("spec/apps/testapp.yaml not rendered as expected:\n%s", app)
	}

	// `formspec init` scaffolds a Starlark-only project — sidecar apps are
	// created separately by `formspec generate <lang>-app`, so no app/ dir.
	if _, err := os.Stat(filepath.Join(root, "app")); err == nil {
		t.Fatal("app/ sidecar directory should not be scaffolded by init")
	}

	// AGENTS.md references the workflow + validation gate; copilot-instructions
	// is a thin pointer to it.
	ci, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
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
		"App Shape",
		"all 34 FormSpec resource kinds",
		"https://docs.formspec.dev/kind/",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
}
