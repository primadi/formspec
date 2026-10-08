package devserver

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTree creates files under root, making parent directories as needed.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// chdir switches into dir for the duration of the test. FindDistUpwards walks
// up from CWD, so the working directory IS the input.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// TestFindDistUpwards_WalksUpAndRequiresShell covers the two properties that
// make SPA auto-detection safe: it must find a bundle from ANY subdirectory of
// the checkout (people run commands from wherever they are), and it must refuse
// a directory without index.html — both producers are non-atomic (`rm -rf` then
// `cp -r`), and serving a half-synced dist renders a blank page for a reason
// nobody can see from the browser.
func TestFindDistUpwards_WalksUpAndRequiresShell(t *testing.T) {
	repo := t.TempDir()
	writeTree(t, repo, map[string]string{
		"renderers/react-shadcn/dist/index.html":    "<html>spa</html>",
		"renderers/react-shadcn/dist/assets/a.js":   "console.log(1)",
		"cmd/formspec-registry/web/dist/index.html": "<html>synced</html>",
		"deeply/nested/place/.keep":                 "",
	})

	t.Run("finds the renderer dist from the repository root", func(t *testing.T) {
		chdir(t, repo)
		got := FindWebDist()
		want := filepath.Join(repo, "renderers", "react-shadcn", "dist")
		if got != want {
			t.Errorf("FindWebDist() = %q, want %q", got, want)
		}
	})

	t.Run("walks up from a nested working directory", func(t *testing.T) {
		chdir(t, filepath.Join(repo, "deeply", "nested", "place"))
		if got := FindWebDist(); got == "" {
			t.Error("FindWebDist() = \"\", want the bundle found by walking up")
		}
	})

	t.Run("finds a caller-supplied relative path", func(t *testing.T) {
		chdir(t, repo)
		got := FindDistUpwards("cmd", "formspec-registry", "web", "dist")
		want := filepath.Join(repo, "cmd", "formspec-registry", "web", "dist")
		if got != want {
			t.Errorf("FindDistUpwards(cmd/…) = %q, want %q", got, want)
		}
	})

	t.Run("a dist without index.html is not a bundle", func(t *testing.T) {
		skewed := t.TempDir()
		// Assets synced, shell missing: exactly the interrupted `cp -r` state.
		writeTree(t, skewed, map[string]string{
			"renderers/react-shadcn/dist/assets/a.js": "console.log(1)",
		})
		chdir(t, skewed)
		if got := FindWebDist(); got != "" {
			t.Errorf("FindWebDist() = %q, want \"\" for a dist without index.html", got)
		}
	})

	t.Run("no checkout above CWD yields empty", func(t *testing.T) {
		chdir(t, t.TempDir())
		if got := FindWebDist(); got != "" {
			t.Errorf("FindWebDist() = %q, want \"\"", got)
		}
	})
}
