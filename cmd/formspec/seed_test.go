package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

func writeSeedSpec(t *testing.T, dir string) {
	t.Helper()
	entityPath := filepath.Join(dir, "modules", "alpha", "master", "customer.yaml")
	if err := os.MkdirAll(filepath.Dir(entityPath), 0o755); err != nil {
		t.Fatal(err)
	}
	entityContent := `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: customer, module: alpha }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true, rules: [required] }
    - { name: name, type: string }
`
	if err := os.WriteFile(entityPath, []byte(entityContent), 0o644); err != nil {
		t.Fatal(err)
	}

	seedPath := filepath.Join(dir, "seed.yaml")
	seedContent := `apiVersion: formspec.dev/v1
kind: Seed
metadata: { name: demo, module: alpha }
spec:
  entities:
    - entity: customer
      records:
        - { code: C-001, name: "PT Maju" }
        - { code: C-002, name: "PT Mundur" }
`
	if err := os.WriteFile(seedPath, []byte(seedContent), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSeedInsertsAndSkips(t *testing.T) {
	dir := t.TempDir()
	writeSeedSpec(t, dir)

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = database.Close() }()

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	for _, loadErr := range reg.LoadEntities() {
		t.Fatalf("load entity: %v", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}

	ctx := context.Background()
	inserted, skipped, failed := seedAll(ctx, res, reg, "", "demo")
	if failed != 0 {
		t.Fatalf("expected 0 failed, got %d", failed)
	}
	if inserted != 2 {
		t.Fatalf("expected 2 inserted, got %d", inserted)
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped, got %d", skipped)
	}

	// Second run: both records already exist by natural key → skipped.
	inserted, skipped, failed = seedAll(ctx, res, reg, "", "demo")
	if inserted != 0 || skipped != 2 || failed != 0 {
		t.Fatalf("expected 0/2/0 (insert/skip/fail), got %d/%d/%d", inserted, skipped, failed)
	}
}

func TestSeedModuleFilter(t *testing.T) {
	dir := t.TempDir()
	writeSeedSpec(t, dir)

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = database.Close() }()

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	for _, loadErr := range reg.LoadEntities() {
		t.Fatalf("load entity: %v", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}

	// Filtering to a non-existent module seeds nothing.
	inserted, skipped, failed := seedAll(context.Background(), res, reg, "other", "demo")
	if inserted != 0 || skipped != 0 || failed != 0 {
		t.Fatalf("expected 0/0/0, got %d/%d/%d", inserted, skipped, failed)
	}
}

// writeRefSeedSpec writes two entities in a PARENT→CHILD relation plus a seed
// whose child block names the parent by natural key through `$ref`.
func writeRefSeedSpec(t *testing.T, dir string) {
	t.Helper()
	parent := filepath.Join(dir, "modules", "alpha", "master", "supplier.yaml")
	if err := os.MkdirAll(filepath.Dir(parent), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, []byte(`apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: supplier, module: alpha }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true }
    - { name: name, type: string }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(dir, "modules", "alpha", "transaction", "po.yaml")
	if err := os.MkdirAll(filepath.Dir(child), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte(`apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: po, module: alpha }
spec:
  version: v1
  characteristic: transaction
  fields:
    - { name: number, type: string, natural_key: true }
    - { name: transaction_date, type: date }
    - name: supplier_id
      type: relation
      relation: { type: belongs_to, resource: alpha.supplier }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// The po block comes FIRST on purpose: resolution must not depend on YAML
	// block ordering being dependency-ordered.
	seed := filepath.Join(dir, "seed.yaml")
	if err := os.WriteFile(seed, []byte(`apiVersion: formspec.dev/v1
kind: Seed
metadata: { name: demo, module: alpha }
spec:
  entities:
    - entity: po
      records:
        - number: PO-1
          supplier_id: { $ref: "alpha.supplier:code=SUP-1" }
    - entity: supplier
      records:
        - { code: SUP-1, name: "Pemasok Utama" }
`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSeedResolvesRefs pins the `$ref` contract: a relation field can name its
// target by a natural value in YAML, even when the target block appears LATER
// in the file, and an unresolvable ref fails loudly instead of writing NULL.
func TestSeedResolvesRefs(t *testing.T) {
	dir := t.TempDir()
	writeRefSeedSpec(t, dir)

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = database.Close() }()

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	for _, loadErr := range reg.LoadEntities() {
		t.Fatalf("load entity: %v", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}
	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}

	inserted, skipped, failed := seedAll(context.Background(), res, reg, "", "demo")
	if failed != 0 {
		t.Fatalf("expected 0 failed, got %d (inserted=%d skipped=%d)", failed, inserted, skipped)
	}
	if inserted != 2 {
		t.Fatalf("expected 2 inserted, got %d", inserted)
	}

	// The po row must hold the supplier's record ID, not the literal "SUP-1".
	supplierStore, err := reg.GetEntityStore("alpha", "supplier")
	if err != nil {
		t.Fatal(err)
	}
	sup, err := supplierStore.FindByField(context.Background(), "demo", "code", "SUP-1")
	if err != nil || sup == nil {
		t.Fatalf("supplier not seeded: %v", err)
	}
	poStore, err := reg.GetEntityStore("alpha", "po")
	if err != nil {
		t.Fatal(err)
	}
	po, err := poStore.FindByField(context.Background(), "demo", "number", "PO-1")
	if err != nil || po == nil {
		t.Fatalf("po not seeded: %v", err)
	}
	if got := po.Data["supplier_id"]; got != sup.ID {
		t.Fatalf("supplier_id = %v, want the supplier record id %s", got, sup.ID)
	}
}

// TestSeedUnresolvedRefFails makes the failure mode explicit: a ref that
// matches nothing is an error, because writing a NULL foreign key would produce
// an order that LOOKS seeded and is broken.
func TestSeedUnresolvedRefFails(t *testing.T) {
	dir := t.TempDir()
	writeRefSeedSpec(t, dir)

	// Rewrite the seed so the po points at a supplier that does not exist.
	seed := filepath.Join(dir, "seed.yaml")
	if err := os.WriteFile(seed, []byte(`apiVersion: formspec.dev/v1
kind: Seed
metadata: { name: demo, module: alpha }
spec:
  entities:
    - entity: po
      records:
        - number: PO-9
          supplier_id: { $ref: "alpha.supplier:code=TIDAK-ADA" }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = database.Close() }()

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	for _, loadErr := range reg.LoadEntities() {
		t.Fatalf("load entity: %v", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}
	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}

	if _, _, failed := seedAll(context.Background(), res, reg, "", "demo"); failed != 1 {
		t.Fatalf("expected 1 failed (unresolved $ref), got %d", failed)
	}
}
