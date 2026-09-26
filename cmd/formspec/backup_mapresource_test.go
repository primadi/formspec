package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// writeMapResourceSpec writes a spec with TWO entities in the same module so a
// restore can be redirected from one onto the other. `customer` holds the
// seeded records; `lead` is the empty target.
func writeMapResourceSpec(t *testing.T, dir string) {
	t.Helper()
	entity := func(name string) string {
		return `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: ` + name + `, module: alpha }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
    - { name: name, type: string }
`
	}
	for _, name := range []string{"customer", "lead"} {
		p := filepath.Join(dir, "modules", "alpha", "master", name+".yaml")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(entity(name)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seed := filepath.Join(dir, "seed.yaml")
	content := `apiVersion: formspec.dev/v1
kind: Seed
metadata: { name: demo, module: alpha }
spec:
  entities:
    - entity: customer
      records:
        - { code: C-001, name: "PT Maju" }
        - { code: C-002, name: "PT Mundur" }
`
	if err := os.WriteFile(seed, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestParseResourceMap covers both accepted spellings and the rejection cases.
// Accepting `module_entity` matters because that is the spelling an operator
// reads out of `formspec backup inspect`, while a spec uses `module/entity` —
// forcing one spelling would invite a silent translation mistake.
func TestParseResourceMap(t *testing.T) {
	got, err := parseResourceMap([]string{"alpha/customer=alpha/lead"})
	if err != nil {
		t.Fatalf("slash form: %v", err)
	}
	if got["alpha/customer"] != "alpha/lead" {
		t.Errorf("slash form -> %v, want alpha/customer=alpha/lead", got)
	}

	got, err = parseResourceMap([]string{"alpha_customer=alpha_lead"})
	if err != nil {
		t.Fatalf("underscore form: %v", err)
	}
	if got["alpha/customer"] != "alpha/lead" {
		t.Errorf("underscore form -> %v, want alpha/customer=alpha/lead", got)
	}

	if got, err := parseResourceMap(nil); err != nil || got != nil {
		t.Errorf("no pairs: got (%v, %v), want (nil, nil)", got, err)
	}

	for _, bad := range []string{"alpha/customer", "=alpha/lead", "alpha/customer=", "customer"} {
		if _, err := parseResourceMap([]string{bad}); err == nil {
			t.Errorf("parseResourceMap(%q) should fail", bad)
		}
	}
}

// TestRestoreMapResource pins todo 3.7.7 end to end: a backup of `customer` is
// restored onto `lead`, and the records land there — while the source entity
// stays empty. This is the "load a prod sample into a dev entity" use case.
func TestRestoreMapResource(t *testing.T) {
	dir := t.TempDir()
	writeMapResourceSpec(t, dir)

	// Source: 2 seeded customers.
	reg, database := seedRegistry(t, dir)
	backupFile := filepath.Join(t.TempDir(), "backup.tar")
	manifest := createBackup(t, reg, backupFile)
	// Both entities are backed up; only `customer` carries records.
	var sourced int
	for _, tb := range manifest.Tables {
		if tb.Entity == "customer" {
			sourced = tb.Count
		}
	}
	if sourced != 2 {
		t.Fatalf("expected alpha/customer to hold 2 records, got %d (%+v)", sourced, manifest.Tables)
	}
	_ = database.Close()

	// Fresh DB, same spec: restore customer -> lead.
	reg2, database2 := loadRegistryForTest(t, dir)
	defer func() { _ = database2.Close() }()

	remap, err := parseResourceMap([]string{"alpha/customer=alpha/lead"})
	if err != nil {
		t.Fatal(err)
	}
	rpt, _, err := restoreFromWithStorage(context.Background(), reg2, backupFile, "skip", false, nil, remap, "demo")
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if rpt.Restored != 2 || rpt.Failed != 0 {
		t.Fatalf("expected 2 restored / 0 failed, got %d/%d", rpt.Restored, rpt.Failed)
	}

	// The report is keyed by the ARCHIVE entry, with the target recorded
	// separately — otherwise an operator cannot tell which file moved where.
	if len(rpt.Entities) != 1 {
		t.Fatalf("expected 1 entity report, got %d", len(rpt.Entities))
	}
	e := rpt.Entities[0]
	if e.Module != "alpha" || e.Entity != "customer" {
		t.Errorf("report keyed by %s/%s, want the archive entry alpha/customer", e.Module, e.Entity)
	}
	if e.MappedTo != "alpha/lead" {
		t.Errorf("MappedTo = %q, want alpha/lead", e.MappedTo)
	}

	// Records landed in the TARGET...
	leadStore, err := reg2.GetEntityStore("alpha", "lead")
	if err != nil {
		t.Fatal(err)
	}
	leads, err := leadStore.List(context.Background(), db.ListParams{WorkspaceID: "demo", Page: 1, PerPage: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(leads.Data) != 2 {
		t.Errorf("lead has %d records, want 2 (the mapped ones)", len(leads.Data))
	}

	// ...and NOT in the source, which the mapping had to skip.
	custStore, err := reg2.GetEntityStore("alpha", "customer")
	if err != nil {
		t.Fatal(err)
	}
	custs, err := custStore.List(context.Background(), db.ListParams{WorkspaceID: "demo", Page: 1, PerPage: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(custs.Data) != 0 {
		t.Errorf("customer has %d records, want 0 — the mapping must redirect, not duplicate", len(custs.Data))
	}
}
