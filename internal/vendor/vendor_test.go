package vendor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── Lock (13.1.1) ───

func TestLockRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "formspec.lock")
	lock := &Lock{Modules: []LockEntry{{
		Source: "github.com/acme/billing-module", Name: "billing", Alias: "acme-billing",
		Version: "1.0.0", Checksum: "sha256:abc", TrustTier: "community",
		InstalledAt: "2026-08-27T00:00:00Z",
	}}}
	if err := lock.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Modules) != 1 {
		t.Fatalf("modules = %d", len(loaded.Modules))
	}
	e := loaded.Modules[0]
	if e.EffectiveName() != "acme-billing" || e.DirName() != "acme-billing" {
		t.Errorf("effective name = %q", e.EffectiveName())
	}
	// Missing lock → empty, no error.
	if empty, err := LoadLock(filepath.Join(t.TempDir(), "nope.lock")); err != nil || len(empty.Modules) != 0 {
		t.Errorf("missing lock: %v %+v", err, empty)
	}
}

func TestTreeChecksum(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "impl"), 0755)
	os.WriteFile(filepath.Join(dir, "module.yaml"), []byte("a: 1\n"), 0644)
	os.WriteFile(filepath.Join(dir, "impl", "x.star"), []byte("def f():\n    pass\n"), 0644)

	c1, err := TreeChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c1, "sha256:") {
		t.Errorf("checksum = %q", c1)
	}
	// Deterministic.
	c2, _ := TreeChecksum(dir)
	if c1 != c2 {
		t.Error("checksum not deterministic")
	}
	// Content change → different checksum.
	os.WriteFile(filepath.Join(dir, "module.yaml"), []byte("a: 2\n"), 0644)
	c3, _ := TreeChecksum(dir)
	if c3 == c1 {
		t.Error("content change not detected")
	}
}

// ─── Alias Opsi B (13.1.3, D-e) ───

func TestResolveAlias(t *testing.T) {
	type tc struct {
		label  string
		name   string
		source string
		taken  []string
		want   string
	}
	cases := []tc{
		{label: "no conflict", name: "billing", source: "github.com/acme/billing-module", taken: nil, want: ""},
		{label: "org alias", name: "billing", source: "github.com/acme/billing-module", taken: []string{"billing"}, want: "acme-billing"},
		{label: "org taken numbered", name: "billing", source: "github.com/acme/billing-module", taken: []string{"billing", "acme-billing"}, want: "billing-2"},
		{label: "folder source", name: "billing", source: "/home/dev/modules/billing", taken: []string{"billing"}, want: "modules-billing"},
		{label: "no parent match", name: "billing", source: "billing", taken: []string{"billing"}, want: "billing-vendor"},
	}
	for _, c := range cases {
		if got := ResolveAlias(c.name, c.source, c.taken); got != c.want {
			t.Errorf("%s: ResolveAlias(%q, %q, %v) = %q, want %q", c.label, c.name, c.source, c.taken, got, c.want)
		}
	}
}

// ─── Marker (13.1.2, D-f/D-g) ───

const appFixture = `apiVersion: formspec.dev/v1
kind: App
metadata:
  name: myapp
  module: myapp
spec:
  version: "1.0"
  modules:
    - billing
    - cafe-master
`

func TestUpsertMarker_InsertInactive(t *testing.T) {
	out := UpsertMarker(appFixture, "github.com/acme/billing-module", "1.0.0", "acme-billing", false)
	ms := FindMarkers(out)
	if len(ms) != 1 {
		t.Fatalf("markers = %d", len(ms))
	}
	m := ms[0]
	if m.Source != "github.com/acme/billing-module" || m.Version != "1.0.0" {
		t.Errorf("marker = %+v", m)
	}
	if m.Active {
		t.Error("new marker without --use must be inactive")
	}
	if m.Entry != "acme-billing" {
		t.Errorf("entry = %q", m.Entry)
	}
	// Still valid YAML with the marker commented.
	if !strings.Contains(out, "# - acme-billing") {
		t.Errorf("entry not commented:\n%s", out)
	}
}

func TestUpsertMarker_PreservesActiveState(t *testing.T) {
	// Start active (developer uncommented).
	active := UpsertMarker(appFixture, "github.com/acme/billing-module", "1.0.0", "acme-billing", true)
	if !FindMarkers(active)[0].Active {
		t.Fatal("setup failed — marker not active")
	}
	// Re-install at a new version → stays active (D-g), version updated.
	updated := UpsertMarker(active, "github.com/acme/billing-module", "1.1.0", "acme-billing", false)
	m := FindMarkers(updated)[0]
	if !m.Active {
		t.Error("re-install must preserve active state (D-g)")
	}
	if m.Version != "1.1.0" {
		t.Errorf("version not updated: %q", m.Version)
	}
	// Inactive stays inactive on re-install WITHOUT --use (D-g).
	inactive := UpsertMarker(appFixture, "github.com/acme/billing-module", "1.0.0", "acme-billing", false)
	stillInactive := UpsertMarker(inactive, "github.com/acme/billing-module", "1.1.0", "acme-billing", false)
	if FindMarkers(stillInactive)[0].Active {
		t.Error("re-install without --use must preserve inactive state (D-g)")
	}
	// Explicit --use on re-install ACTIVATES — an explicit flag is a user
	// decision, not a surprise.
	activated := UpsertMarker(inactive, "github.com/acme/billing-module", "1.1.0", "acme-billing", true)
	if !FindMarkers(activated)[0].Active {
		t.Error("explicit --use on re-install must activate")
	}
}

func TestRemoveMarker(t *testing.T) {
	with := UpsertMarker(appFixture, "github.com/acme/billing-module", "1.0.0", "acme-billing", false)
	out, removed := RemoveMarker(with, "github.com/acme/billing-module")
	if !removed || strings.Contains(out, "formspec:vendor") {
		t.Errorf("marker not removed:\n%s", out)
	}
	if !strings.Contains(out, "- billing") {
		t.Errorf("existing modules damaged:\n%s", out)
	}
}

// ─── Install / Uninstall / Verify (13.1.2/13.1.5/13.1.6) ───

func writeVendorSource(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "entity"), 0755)
	os.WriteFile(filepath.Join(src, "module.yaml"), []byte(
		"apiVersion: formspec.dev/v1\nkind: Module\nmetadata:\n  name: billing\n  description: test\n"), 0644)
	os.WriteFile(filepath.Join(src, "entity", "invoice.yaml"), []byte(
		"apiVersion: formspec.dev/v1\nkind: Entity\nmetadata:\n  name: invoice\n  module: billing\nspec:\n  version: v1\n  characteristic: transaction\n  lifecycle: plain_crud\n  plural: invoices\n  transaction_date: issued_at\n  fields:\n    - name: issued_at\n      type: date\n      required: true\n"), 0644)
	return src
}

func TestInstall_Flow(t *testing.T) {
	src := writeVendorSource(t)
	project := t.TempDir()
	specDir := filepath.Join(project, "spec")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "app.yaml"), []byte(appFixture), 0644)

	res, err := Install(t.Context(), src, Options{
		ProjectRoot: project, SpecPath: specDir, Use: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Entry.Name != "billing" || res.Entry.EffectiveName() != "billing" {
		t.Errorf("entry = %+v", res.Entry)
	}
	if res.Active {
		t.Error("install without --use must be inactive")
	}
	// vendors/ populated + lock written.
	if _, err := os.Stat(res.Dir); err != nil {
		t.Errorf("vendors dir missing: %v", err)
	}
	lock, err := LoadLock(filepath.Join(project, "formspec.lock"))
	if err != nil || len(lock.Modules) != 1 {
		t.Fatalf("lock: %v %+v", err, lock)
	}
	if lock.Modules[0].Checksum == "" {
		t.Error("checksum empty")
	}
	// Marker written into the App manifest.
	appData, _ := os.ReadFile(filepath.Join(specDir, "app.yaml"))
	if !strings.Contains(string(appData), "formspec:vendor") {
		t.Error("marker not written to App manifest")
	}

	// Re-install → updated, still one lock entry, state preserved.
	res2, err := Install(t.Context(), src, Options{ProjectRoot: project, SpecPath: specDir, Use: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Updated {
		t.Error("second install should be an update")
	}
	lock2, _ := LoadLock(filepath.Join(project, "formspec.lock"))
	if len(lock2.Modules) != 1 {
		t.Errorf("lock duplicated on re-install: %d entries", len(lock2.Modules))
	}
	if !res2.Active {
		t.Error("re-install with --use on previously-inactive module: marker had no entry state — expected active")
	}

	// Verify passes on an untouched tree.
	results, err := Verify(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("verify = %+v", results)
	}

	// Tamper → verify fails (13.1.6).
	os.WriteFile(filepath.Join(res.Dir, "module.yaml"), []byte("tampered: true\n"), 0644)
	results, _ = Verify(project)
	if results[0].OK {
		t.Error("tampered vendors/ must fail verify")
	}

	// Uninstall cleans everything.
	removed, err := Uninstall(project, specDir, "billing")
	if err != nil || !removed {
		t.Fatalf("uninstall: %v %v", removed, err)
	}
	if _, err := os.Stat(res.Dir); !os.IsNotExist(err) {
		t.Error("vendors dir not removed")
	}
	lock3, _ := LoadLock(filepath.Join(project, "formspec.lock"))
	if len(lock3.Modules) != 0 {
		t.Error("lock entry not removed")
	}
	appData, _ = os.ReadFile(filepath.Join(specDir, "app.yaml"))
	if strings.Contains(string(appData), "formspec:vendor") {
		t.Error("marker not removed from App manifest")
	}
}

func TestInstall_AliasOnConflict(t *testing.T) {
	src := writeVendorSource(t)
	project := t.TempDir()
	specDir := filepath.Join(project, "spec")
	os.MkdirAll(filepath.Join(specDir, "modules", "billing"), 0755) // local module named billing
	os.WriteFile(filepath.Join(specDir, "app.yaml"), []byte(appFixture), 0644)

	res, err := Install(t.Context(), src, Options{ProjectRoot: project, SpecPath: specDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Entry.EffectiveName() == "billing" {
		t.Fatal("expected an alias when a local module has the same name")
	}
	// Vendored module.yaml normalized to the effective name (13.1.4).
	data, err := os.ReadFile(filepath.Join(res.Dir, "module.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "name: "+res.Entry.EffectiveName()) {
		t.Errorf("module.yaml not normalized to effective name:\n%s", data)
	}
}

func TestActiveModules(t *testing.T) {
	project := t.TempDir()
	specDir := filepath.Join(project, "spec")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "app.yaml"), []byte(appFixture), 0644)

	// Nothing installed → empty, no error.
	active, err := ActiveModules(project, specDir)
	if err != nil || len(active) != 0 {
		t.Fatalf("empty project: %v %v", active, err)
	}

	src := writeVendorSource(t)
	if _, err := Install(t.Context(), src, Options{ProjectRoot: project, SpecPath: specDir, Use: true}); err != nil {
		t.Fatal(err)
	}
	active, err = ActiveModules(project, specDir)
	if err != nil || len(active) != 1 || active[0] != "billing" {
		t.Fatalf("active = %v %v", active, err)
	}
}
