// Command `formspec diff` — compare local spec against deployed state
// (docs/cli-tools/02-formspec-cli.md §2). In single-server scope (no Control
// Plane), "deployed" = the schema already materialized in the database. This
// is a pure dry-run: it reads the pending structural migration (field
// add/remove/type-change) and prints it without changing anything.
//
//	formspec diff -f <path> [--dsn <dsn>]
//
// Exit 0 when local spec matches deployed state; exit 1 when there are
// differences (so it can gate CI).
package main

import (
	"context"
	"fmt"
	"os"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

func runDiff(args []string) {
	specPath := "spec"
	dsn := "sqlite:.formspec/data.db"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--spec", "-spec":
			if i+1 < len(args) {
				specPath = args[i+1]
				i++
			}
		case "--dsn", "-dsn":
			if i+1 < len(args) {
				dsn = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec diff -f <path> [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "formspec diff: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}

	entities := loadEntityMigrations(specPath)

	database, err := db.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}
	runner := db.NewMigrationRunner(database, driver)

	ctx := context.Background()
	if err := runner.EnsureSystemTables(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: ensure system tables: %v\n", err)
		os.Exit(1)
	}

	results, err := computeDiff(ctx, runner, entities)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: plan migrations: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Println("No differences: local spec matches deployed state.")
		return
	}

	fmt.Printf("%d difference(s) between local spec and deployed state:\n\n", len(results))
	for _, r := range results {
		fmt.Printf("── %s ──\n%s\n\n", r.Description, r.DDL)
	}
	os.Exit(1)
}

// computeDiff returns the pending structural migrations between the local
// entity specs and the deployed schema. Extracted for testability.
func computeDiff(ctx context.Context, runner *db.MigrationRunner, entities []db.EntityMigration) ([]db.DDLResult, error) {
	return runner.PlanMigrations(ctx, entities)
}
