package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// buildHonestySpec writes a minimal spec with one entity whose action is
// implemented by the given script body and declares the given uses YAML
// fragment ("" = no uses block).
func buildHonestySpec(t *testing.T, dir, scriptBody, usesYAML string) {
	t.Helper()

	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("apps/test.yaml", `apiVersion: formspec.dev/v1
kind: App
metadata:
  name: test
spec:
  version: 1.0.0
  root_url: /app/test
  modules:
    - alpha
`)

	write("modules/alpha/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: alpha
spec:
  version: 1.0.0
`)

	entity := `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: order
  module: alpha
spec:
  version: v1
  characteristic: transaction
  fields:
    - name: transaction_date
      type: date
      required: true
      index: true
    - name: number
      type: string
  actions:
    - name: probe
      required_permission: alpha.orders.probe
      impl: { type: script_ref, ref: probe }
` + usesYAML + `  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`
	write("modules/alpha/transaction/order/entity.yaml", entity)
	write("modules/alpha/transaction/order/scripts/probe.star", scriptBody)
}

func loadHonestyManifests(t *testing.T, specPath string) []manifest.RawManifest {
	t.Helper()
	res, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	return res.Manifests
}

// TestHonestyScan_UndeclaredPrimitive proves a script using ctx.db() without
// declaring it produces an ERROR (todo 3.1.1a).
func TestHonestyScan_UndeclaredPrimitive(t *testing.T) {
	dir := t.TempDir()
	buildHonestySpec(t, dir,
		"def execute(resource, params, ctx):\n    rows = ctx.db().query(\"SELECT 1\")\n    return ok({\"n\": len(rows)})\n",
		"")
	manifests := loadHonestyManifests(t, dir)

	issues := scanHonesty(manifests, dir)
	var errs []string
	for _, iss := range issues {
		if iss.Severity == "error" && strings.Contains(iss.Message, "ctx.db") {
			errs = append(errs, iss.Message)
		}
	}
	if len(errs) == 0 {
		t.Fatalf("expected undeclared ctx.db error, got %+v", issues)
	}
}

// TestHonestyScan_DeclaredButUnused proves a declared-but-unused primitive
// produces a WARNING with fix metadata (todo 3.1.1a).
func TestHonestyScan_DeclaredButUnused(t *testing.T) {
	dir := t.TempDir()
	buildHonestySpec(t, dir,
		"def execute(resource, params, ctx):\n    return ok({})\n",
		"      uses:\n        primitives: [db, cache]\n")
	manifests := loadHonestyManifests(t, dir)

	issues := scanHonesty(manifests, dir)
	prims := map[string]bool{}
	for _, iss := range issues {
		if iss.FixKind == "primitive" {
			prims[iss.Entry] = true
		}
	}
	if !prims["db"] || !prims["cache"] {
		t.Fatalf("expected unused warnings for db+cache, got %+v", issues)
	}
}

// TestHonestyScan_HonestUsesClean proves a script whose usage matches its
// declarations produces no issues.
func TestHonestyScan_HonestUsesClean(t *testing.T) {
	dir := t.TempDir()
	buildHonestySpec(t, dir,
		"def execute(resource, params, ctx):\n    rows = ctx.cache().get(\"k\")\n    return ok({})\n",
		"      uses:\n        primitives: [cache]\n")
	manifests := loadHonestyManifests(t, dir)

	issues := scanHonesty(manifests, dir)
	if len(issues) != 0 {
		t.Fatalf("expected clean scan, got %+v", issues)
	}
}

// TestHonestyScan_EnvironmentWarning proves ctx.environment branching warns.
func TestHonestyScan_EnvironmentWarning(t *testing.T) {
	dir := t.TempDir()
	buildHonestySpec(t, dir,
		"def execute(resource, params, ctx):\n    if ctx.environment == \"production\":\n        return fail(\"nope\")\n    return ok({})\n",
		"")
	manifests := loadHonestyManifests(t, dir)

	issues := scanHonesty(manifests, dir)
	found := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "ctx.environment") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ctx.environment warning, got %+v", issues)
	}
}

// TestHonestyScan_UndeclaredResource proves resource.fetch to an undeclared
// cross-module target errors.
func TestHonestyScan_UndeclaredResource(t *testing.T) {
	dir := t.TempDir()
	buildHonestySpec(t, dir,
		"def execute(resource, params, ctx):\n    other = resource.fetch(\"pharmacy.medicine\", \"m-1\")\n    return ok({})\n",
		"")
	manifests := loadHonestyManifests(t, dir)

	issues := scanHonesty(manifests, dir)
	found := false
	for _, iss := range issues {
		if iss.Severity == "error" && strings.Contains(iss.Message, "pharmacy.medicine") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected undeclared resource error, got %+v", issues)
	}
}

// TestHonestyFix_RemovesUnused proves --fix removes declared-but-unused
// entries from the manifest file and never adds declarations.
func TestHonestyFix_RemovesUnused(t *testing.T) {
	dir := t.TempDir()
	buildHonestySpec(t, dir,
		"def execute(resource, params, ctx):\n    return ok({})\n",
		"      uses:\n        primitives: [db, cache]\n        resources:\n          - pharmacy.medicine\n")
	manifests := loadHonestyManifests(t, dir)

	issues := scanHonesty(manifests, dir)
	removed := applyHonestyFix(manifests, issues)
	if removed == 0 {
		t.Fatalf("expected fixes to be applied")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "modules/alpha/transaction/order/entity.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "primitives") || strings.Contains(s, "pharmacy.medicine") {
		t.Fatalf("unused entries not removed:\n%s", s)
	}
	if strings.Contains(s, "uses:") {
		t.Fatalf("empty uses block should have been pruned:\n%s", s)
	}

	// Re-scan must be clean now.
	manifests2 := loadHonestyManifests(t, dir)
	if issues := scanHonesty(manifests2, dir); len(issues) != 0 {
		t.Fatalf("expected clean re-scan after fix, got %+v", issues)
	}
}
