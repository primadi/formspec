package main

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// writeBackupSpec writes an entity + seed manifest for backup/restore tests.
func writeBackupSpec(t *testing.T, dir string) {
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

// seedRegistry loads the spec, syncs schema, and runs the seeders.
func seedRegistry(t *testing.T, dir string) (*entity.Registry, db.DB) {
	t.Helper()
	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
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
	if _, _, failed := seedAll(context.Background(), res, reg, ""); failed != 0 {
		t.Fatalf("seed failed")
	}
	return reg, database
}

func TestBackupCreateInspectRestore(t *testing.T) {
	dir := t.TempDir()
	writeBackupSpec(t, dir)

	// Source DB with 2 seeded records.
	reg, database := seedRegistry(t, dir)
	backupFile := filepath.Join(t.TempDir(), "backup.tar")

	// Create backup.
	manifest := createBackup(t, reg, backupFile)
	if len(manifest.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(manifest.Tables))
	}
	if manifest.Tables[0].Count != 2 {
		t.Fatalf("expected 2 records, got %d", manifest.Tables[0].Count)
	}
	database.Close()

	// Inspect should report the same.
	inspected := inspectBackup(t, backupFile)
	if inspected.CreatedAt == "" || len(inspected.Tables) != 1 {
		t.Fatalf("inspect failed: %+v", inspected)
	}

	// Restore into a fresh DB.
	reg2, database2 := loadRegistryForTest(t, dir)
	defer database2.Close()
	rpt := restoreFrom(context.Background(), reg2, backupFile, "skip", false)
	if rpt.Restored != 2 || rpt.Skipped != 0 || rpt.Failed != 0 {
		t.Fatalf("expected 2/0/0, got %d/%d/%d", rpt.Restored, rpt.Skipped, rpt.Failed)
	}

	// Restore again → all skipped (idempotent).
	rpt = restoreFrom(context.Background(), reg2, backupFile, "skip", false)
	if rpt.Restored != 0 || rpt.Skipped != 2 || rpt.Failed != 0 {
		t.Fatalf("expected 0/2/0 on second restore, got %d/%d/%d", rpt.Restored, rpt.Skipped, rpt.Failed)
	}
}

func TestBackupRestoreDryRun(t *testing.T) {
	dir := t.TempDir()
	writeBackupSpec(t, dir)

	reg, database := seedRegistry(t, dir)
	backupFile := filepath.Join(t.TempDir(), "backup.tar")
	createBackup(t, reg, backupFile)
	database.Close()

	reg2, database2 := loadRegistryForTest(t, dir)
	defer database2.Close()

	// Dry-run should report restored but not actually insert.
	rpt := restoreFrom(context.Background(), reg2, backupFile, "skip", true)
	if rpt.Restored != 2 || rpt.Skipped != 0 || rpt.Failed != 0 {
		t.Fatalf("dry-run expected 2/0/0, got %d/%d/%d", rpt.Restored, rpt.Skipped, rpt.Failed)
	}
	// Nothing actually inserted.
	store, _ := reg2.GetEntityStore("alpha", "customer")
	res, err := store.List(context.Background(), db.ListParams{WorkspaceID: "demo", Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("dry-run should not insert; found %d records", res.Total)
	}
}

// TestBackupRestoreRemap verifies the remap conflict mode (4.8.3): when a
// natural key already exists, remap assigns a fresh key and inserts a new
// record, preserving the existing one.
func TestBackupRestoreRemap(t *testing.T) {
	dir := t.TempDir()
	writeBackupSpec(t, dir)

	reg, database := seedRegistry(t, dir)
	backupFile := filepath.Join(t.TempDir(), "backup.tar")
	createBackup(t, reg, backupFile)
	database.Close()

	reg2, database2 := loadRegistryForTest(t, dir)
	defer database2.Close()

	// First restore inserts both records.
	rpt := restoreFrom(context.Background(), reg2, backupFile, "skip", false)
	if rpt.Restored != 2 || rpt.Skipped != 0 || rpt.Failed != 0 {
		t.Fatalf("first restore expected 2/0/0, got %d/%d/%d", rpt.Restored, rpt.Skipped, rpt.Failed)
	}

	// Second restore with remap: both natural keys collide → both remapped.
	rpt = restoreFrom(context.Background(), reg2, backupFile, "remap", false)
	if rpt.Remapped != 2 || rpt.Restored != 0 || rpt.Failed != 0 {
		t.Fatalf("remap restore expected 0/0/2 remapped, got restored=%d remapped=%d failed=%d",
			rpt.Restored, rpt.Remapped, rpt.Failed)
	}

	// All four records present, with distinct natural keys.
	store, _ := reg2.GetEntityStore("alpha", "customer")
	res, err := store.List(context.Background(), db.ListParams{WorkspaceID: "demo", Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if res.Total != 4 {
		t.Fatalf("expected 4 records after remap, got %d", res.Total)
	}
	seen := map[string]bool{}
	for _, rec := range res.Data {
		code, _ := rec.Data["code"].(string)
		if seen[code] {
			t.Fatalf("duplicate natural key after remap: %s", code)
		}
		seen[code] = true
	}
	if !seen["C-001"] || !seen["C-002"] {
		t.Fatal("original natural keys should be preserved")
	}
	remapped := 0
	for code := range seen {
		if code != "C-001" && code != "C-002" {
			remapped++
		}
	}
	if remapped != 2 {
		t.Fatalf("expected 2 remapped keys, got %d", remapped)
	}
}

func TestMatchesFilter(t *testing.T) {
	// Empty filter matches everything.
	if !matchesFilter("alpha", "customer", "") {
		t.Fatal("empty filter should match")
	}
	// Module filter.
	if !matchesFilter("alpha", "customer", "alpha") {
		t.Fatal("module filter should match")
	}
	if matchesFilter("beta", "customer", "alpha") {
		t.Fatal("module filter should not match other module")
	}
	// Module/entity filter.
	if !matchesFilter("alpha", "customer", "alpha/customer") {
		t.Fatal("module/entity filter should match")
	}
	if matchesFilter("alpha", "order", "alpha/customer") {
		t.Fatal("module/entity filter should not match other entity")
	}
}

// createBackup runs the backup-create logic against a registry and returns the manifest.
func createBackup(t *testing.T, reg *entity.Registry, out string) BackupManifest {
	t.Helper()
	ctx := context.Background()
	manifest := BackupManifest{CreatedAt: "test", Driver: "sqlite"}
	f, err := os.Create(out)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	tw := tar.NewWriter(f)
	defer tw.Close()
	for _, info := range reg.ListEntities() {
		store, err := reg.GetEntityStore(info.Module, info.Name)
		if err != nil {
			t.Fatalf("store: %v", err)
		}
		records, err := listAll(ctx, store, "demo")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		manifest.Tables = append(manifest.Tables, BackupTable{Module: info.Module, Entity: info.Name, Table: info.TableName, Count: len(records)})
		if len(records) > 0 {
			if err := writeJSONL(tw, info.Module+"_"+info.Name+".jsonl", records); err != nil {
				t.Fatalf("writeJSONL: %v", err)
			}
		}
	}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	if err := writeBytes(tw, "manifest.json", mb); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return manifest
}

// inspectBackup reads the manifest from a backup archive.
func inspectBackup(t *testing.T, file string) BackupManifest {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	tr := tar.NewReader(f)
	var m BackupManifest
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if hdr.Name == "manifest.json" {
			b, _ := io.ReadAll(tr)
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatalf("parse manifest: %v", err)
			}
		}
	}
	return m
}

// loadRegistryForTest loads a registry without seeding.
func loadRegistryForTest(t *testing.T, dir string) (*entity.Registry, db.DB) {
	t.Helper()
	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	for _, loadErr := range reg.LoadEntities() {
		t.Fatalf("load entity: %v", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}
	return reg, database
}
