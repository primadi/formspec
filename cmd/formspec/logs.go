// Command `formspec logs` — tail structured logs
// (docs/cli-tools/02-formspec-cli.md §6, docs/spec/platform/09-observability.md §7).
//
//	formspec logs [--workspace <ws>] [--module <m>] [--entity <e>]
//	             [--limit <n>] [--output pretty|json] [--spec <path>] [--dsn <dsn>]
//
// Reads the durable event log (formspec_event_log — the audit_log delivery
// channel) and prints it with optional filters. This is the structured-log
// surface available in single-server mode today; full 12-field request
// logging (09-observability §3) is a Fase 8.2 item.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

func runLogs(args []string) {
	workspace := "demo"
	module := ""
	entityName := ""
	limit := 50
	output := "pretty"
	dsn := "sqlite:.formspec/data.db"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workspace", "-workspace":
			if i+1 < len(args) {
				workspace = args[i+1]
				i++
			}
		case "--module", "-module":
			if i+1 < len(args) {
				module = args[i+1]
				i++
			}
		case "--entity", "-entity":
			if i+1 < len(args) {
				entityName = args[i+1]
				i++
			}
		case "--limit", "-limit":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &limit)
				i++
			}
		case "--output", "-output":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "--dsn", "-dsn":
			if i+1 < len(args) {
				dsn = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec logs [--workspace <ws>] [--module <m>] [--entity <e>] [--limit <n>] [--output pretty|json] [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "formspec logs: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}
	if output != "pretty" && output != "json" {
		fmt.Fprintf(os.Stderr, "formspec logs: --output must be pretty|json\n")
		os.Exit(2)
	}

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

	// Ensure the event log system table exists.
	runner := db.NewMigrationRunner(database, driver)
	if err := runner.EnsureSystemTables(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: ensure system tables: %v\n", err)
		os.Exit(1)
	}

	store := db.NewEventLogStore(database, driver)

	// Build the resource filter "module/entity" from --module/--entity.
	resource := ""
	if module != "" {
		resource = module
		if entityName != "" {
			resource = module + "/" + entityName
		}
	}

	records, err := store.ListByWorkspace(context.Background(), workspace, resource, limit, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: list logs: %v\n", err)
		os.Exit(1)
	}

	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		for _, r := range records {
			_ = enc.Encode(r)
		}
		return
	}

	if len(records) == 0 {
		fmt.Println("No log entries.")
		return
	}
	for _, r := range records {
		fmt.Printf("%s  %s  %s  %s\n", r.DeliveredAt, r.WorkspaceID, r.Resource, r.EventName)
		if r.Payload != "" && r.Payload != "null" {
			fmt.Printf("    %s\n", r.Payload)
		}
	}
}
