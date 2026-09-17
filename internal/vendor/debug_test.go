package vendor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// mustMkdirAll / mustWriteFile keep the fixture setup readable: a failure to
// create the fixture is a test failure, not something to carry on from.
func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDebugFindUpstream(t *testing.T) {
	// Recreate the smoke layout.
	project := t.TempDir()
	specDir := filepath.Join(project, "spec")
	mustMkdirAll(t, specDir)
	mustWriteFile(t, filepath.Join(specDir, "app.yaml"), appFixture)
	src := t.TempDir()
	mustMkdirAll(t, filepath.Join(src, "forms"))
	mustWriteFile(t, filepath.Join(src, "module.yaml"), "apiVersion: formspec.dev/v1\nkind: Module\nmetadata:\n  name: billing\nspec:\n  version: 1.0.0\n")
	mustWriteFile(t, filepath.Join(src, "forms", "checkout.yaml"), "apiVersion: formspec.dev/v1\nkind: Form\nmetadata:\n  name: checkout\n  module: billing\nspec:\n  version: v1\n")

	if _, err := Install(t.Context(), src, Options{ProjectRoot: project, SpecPath: specDir, Use: true}); err != nil {
		t.Fatal(err)
	}

	loader := manifest.NewLoader(specDir)
	vendorBase := filepath.Join(project, "vendors")
	entries, _ := os.ReadDir(vendorBase)
	for _, e := range entries {
		if e.IsDir() {
			loader.AddRoot(filepath.Join(vendorBase, e.Name()))
		}
	}
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range res.Manifests {
		t.Logf("loaded: module=%q kind=%q name=%q source=%q", m.Metadata.Module, m.Kind, m.Metadata.Name, m.Source)
	}
}
