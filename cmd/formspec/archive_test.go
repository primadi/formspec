package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	formspec "github.com/primadi/formspec/resource"
)

func TestParseDuration(t *testing.T) {
	now := time.Now().UTC()
	// 1y → roughly 1 year ago (allow a small window for the time component).
	cutoff, err := parseDuration("1y")
	if err != nil {
		t.Fatalf("1y: %v", err)
	}
	if cutoff.After(now.AddDate(-1, 0, 0).Add(2 * time.Hour)) {
		t.Fatalf("1y cutoff too recent: %v", cutoff)
	}
	// 30d → roughly 30 days ago.
	cutoff, err = parseDuration("30d")
	if err != nil {
		t.Fatalf("30d: %v", err)
	}
	if cutoff.After(now.Add(-30 * 24 * time.Hour).Add(2 * time.Hour)) {
		t.Fatalf("30d cutoff too recent: %v", cutoff)
	}
	// Invalid.
	if _, err := parseDuration("abc"); err == nil {
		t.Fatal("expected error for invalid duration")
	}
	if _, err := parseDuration("5x"); err == nil {
		t.Fatal("expected error for invalid unit")
	}
}

// writeArchiveSpec writes a transaction entity with a transaction_date field.
func writeArchiveSpec(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "modules", "alpha", "transaction", "sale", "entity.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: sale, module: alpha }
spec:
  version: v1
  characteristic: transaction
  backdate_policy: { max_days_back: 10000 }
  fields:
    - { name: transaction_date, type: date, required: true, index: true }
    - { name: amount, type: number }
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveTransactions(t *testing.T) {
	dir := t.TempDir()
	writeArchiveSpec(t, dir)

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	for _, loadErr := range reg.LoadEntities() {
		t.Fatalf("load: %v", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	store, err := reg.GetEntityStore("alpha", "sale")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	ctx := context.Background()

	// Old transaction (2020) and recent transaction (today).
	oldID, err := store.Insert(ctx, db.InsertParams{WorkspaceID: "demo", Data: map[string]any{"transaction_date": "2020-01-01", "amount": 100}})
	if err != nil {
		t.Fatalf("insert old: %v", err)
	}
	if _, err := store.Insert(ctx, db.InsertParams{WorkspaceID: "demo", Data: map[string]any{"transaction_date": time.Now().UTC().Format("2006-01-02"), "amount": 200}}); err != nil {
		t.Fatalf("insert recent: %v", err)
	}

	// Dry-run: 1 would be archived.
	n, err := archiveTransactions(ctx, reg, database, time.Now().AddDate(-1, 0, 0), true)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if n != 1 {
		t.Fatalf("dry-run expected 1 archived, got %d", n)
	}

	// Real run: 1 archived, old row soft-deleted.
	n, err = archiveTransactions(ctx, reg, database, time.Now().AddDate(-1, 0, 0), false)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 archived, got %d", n)
	}

	// Old row should be soft-deleted (not in list).
	res, err := store.List(ctx, db.ListParams{WorkspaceID: "demo", Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, rec := range res.Data {
		if rec.ID == oldID {
			t.Fatal("expected old transaction to be archived/deleted")
		}
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 remaining transaction, got %d", res.Total)
	}

	// Archive file should exist under the batch subdirectory.
	stateDir := formspec.StateDirFromDSN(database.DSN())
	batchID := "archive-" + time.Now().UTC().Format("2006-01-02")
	archiveFile := filepath.Join(stateDir, "archive", batchID, "alpha_sale.jsonl")
	if _, err := os.Stat(archiveFile); err != nil {
		t.Fatalf("expected archive file %s: %v", archiveFile, err)
	}
}
