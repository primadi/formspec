package vendor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

func TestDebugFindUpstream(t *testing.T) {
	// Recreate the smoke layout.
	project := t.TempDir()
	specDir := filepath.Join(project, "spec")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "app.yaml"), []byte(appFixture), 0644)
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "forms"), 0755)
	os.WriteFile(filepath.Join(src, "module.yaml"), []byte("apiVersion: formspec.dev/v1\nkind: Module\nmetadata:\n  name: billing\nspec:\n  version: 1.0.0\n"), 0644)
	os.WriteFile(filepath.Join(src, "forms", "checkout.yaml"), []byte("apiVersion: formspec.dev/v1\nkind: Form\nmetadata:\n  name: checkout\n  module: billing\nspec:\n  version: v1\n"), 0644)

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
