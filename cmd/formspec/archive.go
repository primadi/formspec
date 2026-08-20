// Command `formspec archive` — data archival (docs/cli-tools/02-formspec-cli.md §7).
//
//	formspec archive run --max-age 3y [--dry-run] [--spec <path>] [--dsn <dsn>]
//
// Only transactions (characteristic: transaction) are archived fully; masters
// referenced by archived transactions are flagged locked_for_deletion=true
// (4.9.3). Archive output is an open JSONL format (one record per line) under
// the state dir — Parquet is a future enhancement.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	formspec "github.com/primadi/formspec/resource"
)

func runArchive(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: formspec archive <run|view|restore-batch> [flags]\n")
		os.Exit(2)
	}
	switch args[0] {
	case "run":
		runArchiveRun(args[1:])
	case "view":
		runArchiveView(args[1:])
	case "restore-batch":
		fmt.Fprintf(os.Stderr, "formspec archive restore-batch: not implemented yet\n")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "formspec archive: unknown action %q (want run|view|restore-batch)\n", args[0])
		os.Exit(2)
	}
}

// runArchiveView prints the contents of an archive batch (4.9.5).
//
//	formspec archive view --batch-id <id> [--spec <path>] [--dsn <dsn>]
func runArchiveView(args []string) {
	dsn := "sqlite:.formspec/data.db"
	batchID := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dsn", "-dsn":
			if i+1 < len(args) {
				dsn = args[i+1]
				i++
			}
		case "--batch-id":
			if i+1 < len(args) {
				batchID = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec archive view --batch-id <id> [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "formspec archive view: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}
	if batchID == "" {
		fmt.Fprintf(os.Stderr, "formspec archive view: --batch-id is required\n")
		os.Exit(2)
	}

	database, err := db.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	archiveDir := filepath.Join(formspec.StateDirFromDSN(dsn), "archive", batchID)
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: read archive batch %q: %v\n", batchID, err)
		os.Exit(1)
	}
	total := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		path := filepath.Join(archiveDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: read %s: %v\n", path, err)
			os.Exit(1)
		}
		lines := 0
		for _, b := range data {
			if b == '\n' {
				lines++
			}
		}
		fmt.Printf("%s: %d record(s)\n", e.Name(), lines)
		total += lines
	}
	fmt.Printf("Batch %q: %d total record(s).\n", batchID, total)
}

func runArchiveRun(args []string) {
	specPath := "spec"
	dsn := "sqlite:.formspec/data.db"
	maxAge := ""
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--spec", "-spec":
			if i+1 < len(args) {
				specPath = args[i+1]
				i++
			}
		case "--dsn", "-dsn":
			if i+1 < len(args) {
				dsn = args[i+1]
				i++
			}
		case "--max-age":
			if i+1 < len(args) {
				maxAge = args[i+1]
				i++
			}
		case "--dry-run":
			dryRun = true
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec archive run --max-age <dur> [--dry-run] [--spec <path>] [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "formspec archive run: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}
	if maxAge == "" {
		fmt.Fprintf(os.Stderr, "formspec archive run: --max-age is required (e.g. 3y, 180d)\n")
		os.Exit(2)
	}
	cutoff, err := parseDuration(maxAge)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec archive run: invalid --max-age %q: %v\n", maxAge, err)
		os.Exit(2)
	}

	reg, database, _ := loadRegistry(specPath, dsn)
	defer database.Close()

	ctx := context.Background()
	archived, err := archiveTransactions(ctx, reg, database, cutoff, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if dryRun {
		fmt.Printf("Dry-run: %d transaction(s) would be archived.\n", archived)
		return
	}
	fmt.Printf("Archive complete: %d transaction(s) archived.\n", archived)
}

// archiveTransactions finds transactions older than cutoff, writes them to an
// open JSONL archive, flags referenced masters locked_for_deletion, and (unless
// dryRun) deletes the transaction rows.
func archiveTransactions(ctx context.Context, reg *entity.Registry, database db.DB, cutoff time.Time, dryRun bool) (int, error) {
	archived := 0
	stateDir := formspec.StateDirFromDSN(database.DSN())
	batchID := "archive-" + time.Now().UTC().Format("2006-01-02")
	archiveDir := filepath.Join(stateDir, "archive", batchID)
	if !dryRun {
		if err := os.MkdirAll(archiveDir, 0o755); err != nil {
			return 0, fmt.Errorf("create archive dir: %w", err)
		}
	}

	for _, info := range reg.GetEntitiesByCharacteristic("transaction") {
		store, err := reg.GetEntityStore(info.Module, info.Name)
		if err != nil {
			return archived, fmt.Errorf("store %s.%s: %w", info.Module, info.Name, err)
		}
		specInfo, _ := reg.GetEntity(info.Module, info.Name)
		var entitySpec *spec.EntitySpec
		if specInfo != nil {
			entitySpec = specInfo.EntitySpec
		}
		// Find transactions with transaction_date before cutoff.
		res, err := store.List(ctx, db.ListParams{
			WorkspaceID: "demo", Page: 1, PerPage: 1000,
			Filters: map[string]db.FilterOp{
				"transaction_date": {Op: "lt", Value: cutoff.Format("2006-01-02")},
			},
		})
		if err != nil {
			return archived, fmt.Errorf("list %s.%s: %w", info.Module, info.Name, err)
		}
		if len(res.Data) == 0 {
			continue
		}

		if dryRun {
			archived += len(res.Data)
			continue
		}

		// Write archive file.
		archiveFile := filepath.Join(archiveDir, fmt.Sprintf("%s_%s.jsonl", info.Module, info.Name))
		f, err := os.Create(archiveFile)
		if err != nil {
			return archived, fmt.Errorf("create archive %s: %w", archiveFile, err)
		}
		for _, rec := range res.Data {
			b, err := json.Marshal(rec)
			if err != nil {
				f.Close()
				return archived, err
			}
			if _, err := f.Write(append(b, '\n')); err != nil {
				f.Close()
				return archived, err
			}
		}
		f.Close()

		// Delete the archived transaction rows.
		for _, rec := range res.Data {
			if err := store.SoftDelete(ctx, "demo", rec.ID); err != nil {
				// Best-effort: a locked/guarded row is left in place.
				continue
			}
			archived++
		}

		// Master snapshot as-of (4.9.2): snapshot referenced masters and flag
		// them locked_for_deletion so they cannot be deleted while referenced
		// by archived transactions.
		if err := snapshotMasters(ctx, reg, archiveDir, entitySpec, res.Data); err != nil {
			return archived, err
		}
	}
	return archived, nil
}

// snapshotMasters snapshots masters referenced by archived transactions
// (belongs_to relations) into the archive and flags them locked_for_deletion.
func snapshotMasters(ctx context.Context, reg *entity.Registry, archiveDir string, entitySpec *spec.EntitySpec, transactions []db.EntityRecord) error {
	if entitySpec == nil {
		return nil
	}

	// Collect referenced master ids per (module, entity) from belongs_to fields.
	refs := map[string]map[string]bool{} // "module/entity" → set of ids
	for _, f := range entitySpec.Fields {
		if f.Relation == nil || f.Relation.Type != "belongs_to" {
			continue
		}
		targetModule := ""
		targetEntity := f.Relation.Resource
		if dotIdx := strings.Index(targetEntity, "."); dotIdx >= 0 {
			targetModule = targetEntity[:dotIdx]
			targetEntity = targetEntity[dotIdx+1:]
		}
		key := targetModule + "/" + targetEntity
		if refs[key] == nil {
			refs[key] = map[string]bool{}
		}
		for _, tx := range transactions {
			if id, ok := tx.Data[f.Name].(string); ok && id != "" {
				refs[key][id] = true
			}
		}
	}

	for key, ids := range refs {
		module, entityName, _ := strings.Cut(key, "/")
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			continue
		}
		// Snapshot each referenced master.
		masterFile := filepath.Join(archiveDir, "masters_"+module+"_"+entityName+".jsonl")
		f, err := os.Create(masterFile)
		if err != nil {
			return fmt.Errorf("create master archive %s: %w", masterFile, err)
		}
		for id := range ids {
			rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: "demo", ID: id})
			if err != nil || rec == nil {
				continue
			}
			b, err := json.Marshal(rec)
			if err != nil {
				f.Close()
				return err
			}
			if _, err := f.Write(append(b, '\n')); err != nil {
				f.Close()
				return err
			}
			// Flag locked_for_deletion (4.9.3).
			data := rec.Data
			if data == nil {
				data = map[string]any{}
			}
			data["locked_for_deletion"] = true
			_, _ = store.Update(ctx, db.UpdateParams{
				WorkspaceID: "demo", ID: id, Version: rec.Version,
				UpdatedBy: "archive", Data: data,
			})
		}
		f.Close()
	}
	return nil
}

// parseDuration parses a human duration like "3y", "180d", "6mo".
func parseDuration(s string) (time.Time, error) {
	now := time.Now().UTC()
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid duration %q", s)
	}
	n := 0
	for _, c := range s[:len(s)-1] {
		if c < '0' || c > '9' {
			return time.Time{}, fmt.Errorf("invalid duration %q", s)
		}
		n = n*10 + int(c-'0')
	}
	switch s[len(s)-1] {
	case 'y':
		return now.AddDate(-n, 0, 0), nil
	case 'm':
		return now.AddDate(0, -n, 0), nil
	case 'd':
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	default:
		return time.Time{}, fmt.Errorf("invalid duration unit %q (want y|m|d)", s[len(s)-1])
	}
}
