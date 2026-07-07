// Command forma-entity-sync loads Forma entity manifests from a spec directory,
// registers them with the entity registry, syncs schemas to the database,
// and prints a summary.
//
// Usage:
//
//	go run ./cmd/forma-entity-sync/ [--dsn sqlite:.forma/data.db] [--spec ./examples/Customer/spec]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/forma/forma/internal/db"
	"github.com/forma/forma/internal/entity"
)

func main() {
	ctx := context.Background()

	dsn := flag.String("dsn", "sqlite:.forma/data.db", "Database DSN")
	specPath := flag.String("spec", "./examples/Customer/spec", "Path to spec directory")
	flag.Parse()

	fmt.Println("📦 Forma Entity Sync")
	fmt.Println("=====================")

	// 1. Open database
	database, err := db.Open(*dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}
	fmt.Printf("✓ Database: %s (%s)\n", *dsn, driver)

	// 2. Create registry
	reg := entity.NewRegistry(database, driver, *specPath)
	fmt.Printf("✓ Registry created for spec path: %s\n", *specPath)

	// 3. Load entities
	errs := reg.LoadEntities()
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "  ⚠ %v\n", e)
	}
	fmt.Printf("✓ Entities loaded: %d registered\n", reg.Count())

	// 4. List entities
	for _, info := range reg.ListEntities() {
		fmt.Printf("  • %s/%s", info.Module, info.Name)
		if info.Characteristic != "" {
			fmt.Printf(" [%s]", info.Characteristic)
		}
		fmt.Printf(" (%d fields)", info.FieldCount)
		if info.Description != "" {
			fmt.Printf(" — %s", info.Description)
		}
		fmt.Println()
	}

	// 5. Sync schema
	applied, err := reg.SyncSchema(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ SyncSchema: %v\n", err)
		os.Exit(1)
	}
	if applied > 0 {
		fmt.Printf("✓ Schema synced: %d migration(s) applied\n", applied)
	} else {
		fmt.Println("✓ Schema synced: up to date")
	}

	// 6. Verify: get store and insert sample data
	customerStore, err := reg.GetEntityStore("billing", "customer")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ GetEntityStore(customer): %v\n", err)
		os.Exit(1)
	}

	addressStore, err := reg.GetEntityStore("billing", "address")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ GetEntityStore(address): %v\n", err)
		os.Exit(1)
	}

	// Insert sample customer
	custID, err := customerStore.Insert(ctx, db.InsertParams{
		TenantID: "demo", CreatedBy: "system",
		Data: map[string]any{"name": "Alice", "email": "alice@example.com", "member_tier": "gold"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Insert customer: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Sample customer inserted: id=%s\n", custID)

	// Insert sample address (relation to customer)
	_, err = addressStore.Insert(ctx, db.InsertParams{
		TenantID: "demo", CreatedBy: "system",
		Data: map[string]any{
			"customer_id": custID,
			"type":        "billing",
			"label":       "Rumah",
			"street":      "Jl. Merdeka No. 1",
			"city":        "Jakarta",
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Insert address: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Sample address inserted")

	// Query back the customer
	rec, err := customerStore.GetByID(ctx, db.GetByIDParams{TenantID: "demo", ID: custID})
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ GetByID customer: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Verified: customer name=%s email=%s\n", rec.Data["name"], rec.Data["email"])

	fmt.Println("\n✅ Done! Database ready.")
	fmt.Printf("   DB file: %s\n", *dsn)
	fmt.Printf("   Tables:  %d entity tables created\n", reg.Count())
}
