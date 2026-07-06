// Command forma-dev-init initializes the development SQLite database
// with system tables, sample entities, and demo data.
//
// Usage:
//
//	go run cmd/forma-dev-init/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/forma/forma/internal/db"
	"github.com/forma/forma/pkg/spec"
)

func main() {
	ctx := context.Background()

	// Open SQLite file
	d, err := db.Open("sqlite:.forma/data.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Open failed: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()

	fmt.Println("📦 Forma Dev DB Initializer")
	fmt.Println("===========================")

	// Ensure system tables
	runner := db.NewMigrationRunner(d, db.DriverSQLite)
	if err := runner.EnsureSystemTables(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "EnsureSystemTables: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ System tables created")

	// Create sample entity: customer
	meta := spec.Metadata{Name: "customer", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "email", Type: spec.FieldString, Unique: true},
		},
	}

	if _, err := runner.ApplyMigrations(ctx, []db.EntityMigration{
		{Metadata: meta, EntitySpec: *entity},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "ApplyMigrations: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Entity table: billing_customers")

	// Insert sample data
	store := db.NewEntityStore(d, db.DriverSQLite, meta, entity)
	id, err := store.Insert(ctx, db.InsertParams{
		TenantID: "demo", CreatedBy: "system",
		Data: map[string]any{"name": "Alice", "email": "alice@example.com"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Insert: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Sample data: id=%s name=Alice\n", id)

	fmt.Println("\n✅ Done! Open .forma/data.db in DBeaver as SQLite connection.")
}
