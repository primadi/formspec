package vendor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustFixtureDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustFixtureFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Fixture: project with a vendored module containing a Form (whitelisted)
// and an Entity (not whitelisted).
func setupOverrideFixture(t *testing.T) (project, specDir, vendorSrc string) {
	t.Helper()
	project = t.TempDir()
	specDir = filepath.Join(project, "spec")
	mustFixtureDir(t, filepath.Join(specDir, "modules", "shop"))
	mustFixtureFile(t, filepath.Join(specDir, "app.yaml"), []byte(appFixture))

	vendorSrc = t.TempDir()
	mustFixtureDir(t, filepath.Join(vendorSrc, "forms"))
	mustFixtureFile(t, filepath.Join(vendorSrc, "module.yaml"), []byte(
		"apiVersion: formspec.dev/v1\nkind: Module\nmetadata:\n  name: billing\n  description: test\nspec:\n  version: 1.0.0\n"))
	mustFixtureFile(t, filepath.Join(vendorSrc, "forms", "checkout.yaml"), []byte(
		"apiVersion: formspec.dev/v1\nkind: Form\nmetadata:\n  name: checkout\n  module: billing\nspec:\n  version: v1\n  entity: billing/invoice\n  layout:\n    mode: modal\n"))
	mustFixtureDir(t, filepath.Join(vendorSrc, "entity"))
	mustFixtureFile(t, filepath.Join(vendorSrc, "entity", "invoice.yaml"), []byte(
		"apiVersion: formspec.dev/v1\nkind: Entity\nmetadata:\n  name: invoice\n  module: billing\nspec:\n  version: v1\n  characteristic: master\n  lifecycle: plain_crud\n  plural: invoices\n  fields:\n    - name: number\n      type: string\n      required: true\n"))
	return
}

func TestAdopt_WhitelistedKind(t *testing.T) {
	project, specDir, vendorSrc := setupOverrideFixture(t)
	if _, err := Install(t.Context(), vendorSrc, Options{ProjectRoot: project, SpecPath: specDir, Use: true}); err != nil {
		t.Fatal(err)
	}

	res, err := Adopt(project, specDir, "billing", "Form", "checkout")
	if err != nil {
		t.Fatal(err)
	}
	// Shadow copy written.
	data, err := os.ReadFile(res.OverridePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "kind: Form") {
		t.Errorf("shadow copy content wrong:\n%s", data)
	}
	// Fork base recorded in lock.
	lock, _ := LoadLock(filepath.Join(project, "formspec.lock"))
	entry := lock.FindByEffectiveName("billing")
	if len(entry.Overrides) != 1 {
		t.Fatalf("overrides = %d, want 1", len(entry.Overrides))
	}
	ov := entry.Overrides[0]
	if ov.Kind != "Form" || ov.Name != "checkout" || ov.BaseChecksum == "" {
		t.Errorf("override entry = %+v", ov)
	}
	if !strings.HasPrefix(ov.Origin, "vendors/") {
		t.Errorf("origin = %q, want vendors/...", ov.Origin)
	}

	// Diff: identical → no drift, empty unified.
	diff, err := DiffOverride(project, specDir, "billing", "form", "checkout")
	if err != nil {
		t.Fatal(err)
	}
	if diff.Drift || diff.Unified != "" {
		t.Errorf("fresh adopt should have no drift/diff: %+v", diff)
	}

	// Upstream changes → drift detected (§5.3).
	mustFixtureFile(t, filepath.Join(vendorSrc, "forms", "checkout.yaml"), []byte(
		"apiVersion: formspec.dev/v1\nkind: Form\nmetadata:\n  name: checkout\n  module: billing\nspec:\n  version: v2\n  entity: billing/invoice\n  layout:\n    mode: drawer\n"))
	// Re-install to bring the new upstream into vendors/.
	if _, err := Install(t.Context(), vendorSrc, Options{ProjectRoot: project, SpecPath: specDir, Use: true}); err != nil {
		t.Fatal(err)
	}
	diff, err = DiffOverride(project, specDir, "billing", "Form", "checkout")
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Drift {
		t.Error("upstream change must be flagged as drift")
	}
	if !strings.Contains(diff.Unified, "-    mode: drawer") || !strings.Contains(diff.Unified, "+    mode: modal") {
		t.Errorf("diff missing expected changes (- upstream, + shadow):\n%s", diff.Unified)
	}
	// CheckDrift reports it too.
	drifts, err := CheckDrift(project)
	if err != nil || len(drifts) != 1 {
		t.Fatalf("CheckDrift = %+v err=%v", drifts, err)
	}
}

func TestAdopt_RejectsNonWhitelistedKind(t *testing.T) {
	project, specDir, vendorSrc := setupOverrideFixture(t)
	if _, err := Install(t.Context(), vendorSrc, Options{ProjectRoot: project, SpecPath: specDir, Use: true}); err != nil {
		t.Fatal(err)
	}
	_, err := Adopt(project, specDir, "billing", "Entity", "invoice")
	if err == nil || !strings.Contains(err.Error(), "Entity Extension") {
		t.Errorf("Entity adopt must be rejected with guidance, got: %v", err)
	}
	// No shadow copy written.
	if _, err := os.Stat(OverridePath(project, "billing", "Entity", "invoice")); !os.IsNotExist(err) {
		t.Error("shadow copy must not be written for non-whitelisted kind")
	}
}

func TestValidateOverridesDir(t *testing.T) {
	project := t.TempDir()
	// Empty/missing overrides dir → OK.
	if err := ValidateOverridesDir(project); err != nil {
		t.Fatalf("missing dir: %v", err)
	}
	// Whitelisted kind → OK.
	ovDir := filepath.Join(project, "overrides", "billing")
	mustFixtureDir(t, ovDir)
	mustFixtureFile(t, filepath.Join(ovDir, "form.checkout.yaml"), []byte(
		"apiVersion: formspec.dev/v1\nkind: Form\nmetadata:\n  name: checkout\n  module: billing\nspec: {version: v1, entity: billing/invoice}\n"))
	if err := ValidateOverridesDir(project); err != nil {
		t.Fatalf("whitelisted kind rejected: %v", err)
	}
	// Entity → boot refusal.
	mustFixtureFile(t, filepath.Join(ovDir, "entity.invoice.yaml"), []byte(
		"apiVersion: formspec.dev/v1\nkind: Entity\nmetadata:\n  name: invoice\n  module: billing\nspec: {version: v1}\n"))
	err := ValidateOverridesDir(project)
	if err == nil || !strings.Contains(err.Error(), "not shadow-copyable") {
		t.Errorf("Entity under overrides/ must refuse boot, got: %v", err)
	}
}
