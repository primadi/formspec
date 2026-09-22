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

// TestHonestyScan_HookUsesDeclared proves a hook's `uses` is scanned (#34): a
// hook script using ctx.db() without declaring it is an error, and declaring it
// clears the scan. Before #34, HookDecl had no `uses`, so hook access was
// invisible in the consent footprint.
func TestHonestyScan_HookUsesDeclared(t *testing.T) {
	build := func(t *testing.T, usesYAML string) []honestyIssue {
		t.Helper()
		dir := t.TempDir()
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
		write("modules/alpha/master/order/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: order
  module: alpha
spec:
  version: v1
  characteristic: master
  fields:
    - name: number
      type: string
  hooks:
    - on: before
      action: create
      impl: { type: script_ref, ref: guard }
`+usesYAML+`  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`)
		write("modules/alpha/master/order/scripts/guard.star",
			"def execute(resource, params, ctx):\n    rows = ctx.db().query(\"SELECT 1\")\n    return ok({})\n")
		return scanHonesty(loadHonestyManifests(t, dir), dir)
	}

	undeclared := build(t, "")
	found := false
	for _, iss := range undeclared {
		if iss.Severity == "error" && strings.Contains(iss.Message, "hook:before:create") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected undeclared hook ctx.db error, got %+v", undeclared)
	}

	declared := build(t, "      uses: { primitives: [db] }\n")
	for _, iss := range declared {
		if iss.Severity == "error" {
			t.Fatalf("declared hook uses should be clean, got %+v", iss)
		}
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

// TestHonestyScan_HookScriptCompileError proves a hook script that cannot be
// parsed is reported as an error. Hooks previously only produced the
// ctx.environment warning, so three non-compiling guard scripts on the write
// path passed `formspec validate` with 0 problems (gap #50).
func TestHonestyScan_HookScriptCompileError(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("modules/alpha/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: alpha
spec:
  version: 1.0.0
`)
	write("modules/alpha/master/thing/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: thing
  module: alpha
spec:
  version: v1
  characteristic: master
  plural: things
  fields:
    - name: name
      type: string
  hooks:
    - on: before
      action: create
      impl: { type: script_ref, ref: alpha/broken }
    - on: before
      action: update
      impl: { type: script_ref, ref: alpha/missing }
`)
	// Implicit adjacent string-literal concatenation is valid Python but not
	// Starlark — the exact defect that shipped in the kafe guard scripts.
	write("modules/alpha/scripts/broken.star", "def execute(resource, params, ctx):\n    rows = ctx.db().query(\n        \"SELECT 1 \"\n        \"FROM t\",\n    )\n    return ok()\n")

	manifests := loadHonestyManifests(t, dir)
	issues := scanHonesty(manifests, dir)

	var compileErr, notFound bool
	for _, iss := range issues {
		if iss.Severity != "error" {
			continue
		}
		if strings.Contains(iss.Message, "failed to compile") {
			compileErr = true
		}
		if strings.Contains(iss.Message, "script not found") {
			notFound = true
		}
	}
	if !compileErr {
		t.Errorf("expected a compile error for the broken hook script, got %+v", issues)
	}
	if !notFound {
		t.Errorf("expected a 'script not found' error for the missing hook script, got %+v", issues)
	}
}
