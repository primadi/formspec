package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestRegistry serves the given schemas dir (cmd/formspec → ../.. before chdir)
// as a v1 schema registry, so `formspec init` can fetch schemas without network.
func newTestRegistry(t *testing.T, schemasDir string) *httptest.Server {
	t.Helper()
	root, err := os.ReadFile(filepath.Join(schemasDir, "formspec.schema.json"))
	if err != nil {
		t.Fatalf("read repo schemas: %v", err)
	}
	kindFiles, _ := filepath.Glob(filepath.Join(schemasDir, "kinds", "*.schema.json"))
	kinds := make([]string, 0, len(kindFiles))
	for _, f := range kindFiles {
		kinds = append(kinds, strings.TrimSuffix(filepath.Base(f), ".schema.json"))
	}
	index, _ := json.Marshal(map[string]any{"version": "v1", "kinds": kinds})

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/formspec.schema.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(root)
	})
	mux.HandleFunc("/v1/index.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(index)
	})
	mux.HandleFunc("/v1/kinds/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/kinds/"), ".schema.json")
		data, err := os.ReadFile(filepath.Join(schemasDir, "kinds", name+".schema.json"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

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
	repoSchemas := filepath.Join(oldWD, "..", "..", "schemas")
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	// Schemas are fetched from the registry (online by design). Point the
	// registry at a local test server and isolate the cache.
	srv := newTestRegistry(t, repoSchemas)
	t.Setenv("FORMSPEC_SCHEMA_REGISTRY", srv.URL)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

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
	} {
		if !strings.Contains(content, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
}
