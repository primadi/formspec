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
	defer database.Close()

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
	inserted, skipped, failed := seedAll(ctx, res, reg, "")
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
	inserted, skipped, failed = seedAll(ctx, res, reg, "")
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
	defer database.Close()

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
	inserted, skipped, failed := seedAll(context.Background(), res, reg, "other")
	if inserted != 0 || skipped != 0 || failed != 0 {
		t.Fatalf("expected 0/0/0, got %d/%d/%d", inserted, skipped, failed)
	}
}
