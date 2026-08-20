package main

import (
	"context"
	"testing"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// TestLogsEventLogRoundTrip verifies the event-log store the `formspec logs`
// command reads from: write + list with a resource filter.
func TestLogsEventLogRoundTrip(t *testing.T) {
	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	runner := db.NewMigrationRunner(database, db.DriverSQLite)
	if err := runner.EnsureSystemTables(context.Background()); err != nil {
		t.Fatalf("ensure system tables: %v", err)
	}

	store := db.NewEventLogStore(database, db.DriverSQLite)
	ctx := context.Background()

	if err := store.Write(ctx, "demo", "created", "alpha/customer", []byte(`{"code":"C-001"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := store.Write(ctx, "demo", "submitted", "alpha/order", []byte(`{"number":"INV-1"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Filter by resource "alpha/customer" → 1 record.
	records, err := store.ListByWorkspace(ctx, "demo", "alpha/customer", 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record for alpha/customer, got %d", len(records))
	}
	if records[0].EventName != "created" {
		t.Fatalf("expected event created, got %s", records[0].EventName)
	}

	// No filter → 2 records.
	records, err = store.ListByWorkspace(ctx, "demo", "", 10, 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}
