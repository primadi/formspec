// Workspace CLI — register/list/remove named workspaces in the target
// database (plan docs_internal/plan/named-workspaces.md).
//
// Workspaces are the single multi-tenancy unit of FormSpec (platform/
// 02-workspace-app-module.md §1). This command writes the same registry the
// resource process consults at request time — the formspec.core/workspace
// entity — so a workspace created here is immediately routable:
//
//	formspec workspace create kopi --name "Kopi Kita" --dsn sqlite:.formspec/cafe.db
//	formspec workspace list --dsn sqlite:.formspec/cafe.db
//	formspec workspace delete kopi --dsn sqlite:.formspec/cafe.db --confirm
//
// Slug rules: kebab-case, unique, and must not collide with reserved first
// path segments (_ui, api, _admin, assets, health, login, register, _ws,
// print) — validated by pkg/spec.ValidateWorkspaceSpec.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

func runWorkspace(args []string) {
	if len(args) == 0 {
		usageWorkspace()
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		runWorkspaceCreate(args[1:])
	case "list":
		runWorkspaceList(args[1:])
	case "delete":
		runWorkspaceDelete(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown workspace subcommand %q\n\n", args[0])
		usageWorkspace()
		os.Exit(1)
	}
}

func usageWorkspace() {
	fmt.Fprintf(os.Stderr, "Usage: formspec workspace <subcommand> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  create <slug>            Register a workspace (--name display name)\n")
	fmt.Fprintf(os.Stderr, "  list                     List registered workspaces\n")
	fmt.Fprintf(os.Stderr, "  delete <slug>            Remove a workspace registration (--confirm)\n")
	fmt.Fprintf(os.Stderr, "\nFlags: --dsn (required) --spec (optional, entity registry root)\n")
}

// workspaceRegistry opens the DSN, registers the embedded formspec.core
// entities, syncs the schema, and returns the workspace registry.
func workspaceRegistry(dsn, specPath string) (*auth.WorkspaceRegistry, func()) {
	database, err := db.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: open database: %v\n", err)
		os.Exit(1)
	}
	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}
	reg := entity.NewRegistry(database, driver, specPath)
	if err := auth.RegisterCoreEntities(reg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: register core entities: %v\n", err)
		os.Exit(1)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: sync schema: %v\n", err)
		os.Exit(1)
	}
	store, err := reg.GetEntityStore(auth.CoreModule, auth.CoreWorkspaceEntity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: resolve workspace entity store: %v\n", err)
		os.Exit(1)
	}
	return auth.NewWorkspaceRegistry(store), func() { database.Close() }
}

// parseFlagsFirst parses flags that may appear after positional arguments
// (standard flag.Parse stops at the first non-flag arg). Positional args are
// extracted up-front and returned in order.
func parseFlagsFirst(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flags = append(flags, args[i])
			// Flag values: "--flag value" form consumes the next arg.
			if !strings.Contains(args[i], "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, args[i])
	}
	_ = fs.Parse(flags)
	return positional
}

func runWorkspaceCreate(args []string) {
	fs := flag.NewFlagSet("workspace create", flag.ExitOnError)
	dsn := fs.String("dsn", "", "database DSN (required), e.g. sqlite:.formspec/cafe.db")
	specPath := fs.String("spec", "", "spec root (optional — only needed to resolve spec-level entities)")
	name := fs.String("name", "", "human-readable display name (default: slug)")
	positional := parseFlagsFirst(fs, args)
	if len(positional) < 1 {
		fmt.Fprintln(os.Stderr, "Error: workspace slug is required")
		os.Exit(1)
	}
	slug := positional[0]
	display := slug
	if *name != "" {
		display = *name
	}
	if err := spec.ValidateWorkspaceSpec(&spec.WorkspaceSpec{}, slug); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "Error: --dsn is required")
		os.Exit(1)
	}
	reg, closeDB := workspaceRegistry(*dsn, *specPath)
	defer closeDB()
	created, err := reg.Ensure(context.Background(), auth.WorkspaceInfo{
		Slug:        slug,
		DisplayName: display,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if created {
		fmt.Printf("Workspace created: %s (%s)\n", slug, display)
	} else {
		fmt.Printf("Workspace already registered: %s (display name updated)\n", slug)
	}
}

func runWorkspaceList(args []string) {
	fs := flag.NewFlagSet("workspace list", flag.ExitOnError)
	dsn := fs.String("dsn", "", "database DSN (required)")
	specPath := fs.String("spec", "", "spec root (optional)")
	parseFlagsFirst(fs, args)
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "Error: --dsn is required")
		os.Exit(1)
	}
	reg, closeDB := workspaceRegistry(*dsn, *specPath)
	defer closeDB()
	list, err := reg.List(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%-24s %s\n", "SLUG", "NAME")
	for _, ws := range list {
		fmt.Printf("%-24s %s\n", ws.Slug, ws.DisplayName)
	}
	fmt.Printf("\n%d workspace(s)\n", len(list))
}

func runWorkspaceDelete(args []string) {
	fs := flag.NewFlagSet("workspace delete", flag.ExitOnError)
	dsn := fs.String("dsn", "", "database DSN (required)")
	specPath := fs.String("spec", "", "spec root (optional)")
	confirm := fs.Bool("confirm", false, "actually delete (required)")
	positional := parseFlagsFirst(fs, args)
	if len(positional) < 1 {
		fmt.Fprintln(os.Stderr, "Error: workspace slug is required")
		os.Exit(1)
	}
	slug := positional[0]
	if !*confirm {
		fmt.Fprintf(os.Stderr, "Error: deleting workspace %q requires --confirm\n", slug)
		os.Exit(1)
	}
	if slug == spec.DefaultWorkspaceSlug {
		fmt.Fprintln(os.Stderr, "Error: the default workspace cannot be deleted")
		os.Exit(1)
	}
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "Error: --dsn is required")
		os.Exit(1)
	}
	reg, closeDB := workspaceRegistry(*dsn, *specPath)
	defer closeDB()
	if err := reg.Delete(context.Background(), slug); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Workspace deleted: %s\n", slug)
}
