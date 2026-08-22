// Command `formspec backup` — data lifecycle backup/restore
// (docs/cli-tools/02-formspec-cli.md §6, docs/spec/backend/04-persist-backend.md §3).
//
//	formspec backup create --full [--out <file>] [--spec <path>] [--dsn <dsn>]
//	formspec backup inspect <file>
//	formspec restore --from <file> [--conflict skip|overwrite] [--dry-run] [--spec <path>] [--dsn <dsn>]
//
// Backup format is an open tar archive:
//
//	manifest.json            → {created_at, driver, tables:[{module,entity,table,count}]}
//	<module>_<entity>.jsonl  → one flattened record per line (wire shape)
//
// File storage (ctx.storage) is not yet included — noted as a gap (4.8.1).
// Read/export operations are never license-gated (credible exit, 4.8.4).
package main

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	formspec "github.com/primadi/formspec/resource"
)

// BackupManifest describes the contents of a backup archive.
type BackupManifest struct {
	CreatedAt string        `json:"created_at"`
	Driver    string        `json:"driver"`
	Tables    []BackupTable `json:"tables"`
}

// BackupTable describes one entity table in the backup.
type BackupTable struct {
	Module string `json:"module"`
	Entity string `json:"entity"`
	Table  string `json:"table"`
	Count  int    `json:"count"`
}

func runBackup(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: formspec backup <create|inspect> [flags]\n")
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		runBackupCreate(args[1:])
	case "inspect":
		runBackupInspect(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "formspec backup: unknown action %q (want create|inspect)\n", args[0])
		os.Exit(2)
	}
}

func runBackupCreate(args []string) {
	specPath := "spec"
	dsn := "sqlite:.formspec/data.db"
	out := ""
	full := false
	filter := "" // "module" or "module/entity" (4.8.2)
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
		case "--out", "-out":
			if i+1 < len(args) {
				out = args[i+1]
				i++
			}
		case "--filter", "-filter":
			if i+1 < len(args) {
				filter = args[i+1]
				i++
			}
		case "--full":
			full = true
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec backup create --full [--out <file>] [--filter <module|module/entity>] [--spec <path>] [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "formspec backup create: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}
	if !full {
		fmt.Fprintf(os.Stderr, "formspec backup create: --full is required (incremental not yet implemented)\n")
		os.Exit(2)
	}
	if out == "" {
		out = fmt.Sprintf("backup-%s.tar", time.Now().Format("2006-01-02"))
	}

	reg, database, driver := loadRegistry(specPath, dsn)
	defer database.Close()

	ctx := context.Background()
	manifest := BackupManifest{CreatedAt: time.Now().UTC().Format(time.RFC3339), Driver: driver}

	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: create %s: %v\n", out, err)
		os.Exit(1)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	for _, info := range reg.ListEntities() {
		// Filterable backup (4.8.2): --filter <module> or <module/entity>.
		if filter != "" && !matchesFilter(info.Module, info.Name, filter) {
			continue
		}
		store, err := reg.GetEntityStore(info.Module, info.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: store %s.%s: %v\n", info.Module, info.Name, err)
			os.Exit(1)
		}
		records, err := listAll(ctx, store, "demo")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: list %s.%s: %v\n", info.Module, info.Name, err)
			os.Exit(1)
		}
		manifest.Tables = append(manifest.Tables, BackupTable{
			Module: info.Module, Entity: info.Name, Table: info.TableName, Count: len(records),
		})
		if len(records) == 0 {
			continue
		}
		if err := writeJSONL(tw, info.Module+"_"+info.Name+".jsonl", records); err != nil {
			fmt.Fprintf(os.Stderr, "Error: write %s: %v\n", info.Module+"_"+info.Name, err)
			os.Exit(1)
		}
	}

	mb, _ := json.MarshalIndent(manifest, "", "  ")
	if err := writeBytes(tw, "manifest.json", mb); err != nil {
		fmt.Fprintf(os.Stderr, "Error: write manifest: %v\n", err)
		os.Exit(1)
	}

	// File storage (ctx.storage) ikut ter-backup (4.8.1): files under
	// {state}/storage are added under storage/ in the archive.
	storageDir := filepath.Join(formspec.StateDirFromDSN(dsn), "storage")
	if err := writeDirToTar(tw, storageDir, "storage"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: write storage: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Backup written to %s (%d table(s), %d record(s)).\n", out, len(manifest.Tables), manifestRecordCount(manifest))
}

// writeDirToTar recursively adds a directory's files to the tar under prefix.
func writeDirToTar(tw *tar.Writer, dir, prefix string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // storage dir may not exist
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeBytes(tw, prefix+"/"+rel, data)
	})
}

func runBackupInspect(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: formspec backup inspect <file>\n")
		os.Exit(2)
	}
	file := args[0]

	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: open %s: %v\n", file, err)
		os.Exit(1)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	var manifest BackupManifest
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: read tar: %v\n", err)
			os.Exit(1)
		}
		if hdr.Name == "manifest.json" {
			b, _ := io.ReadAll(tr)
			if err := json.Unmarshal(b, &manifest); err != nil {
				fmt.Fprintf(os.Stderr, "Error: parse manifest: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if manifest.CreatedAt == "" {
		fmt.Fprintf(os.Stderr, "Error: %s is not a valid formspec backup (no manifest.json)\n", file)
		os.Exit(1)
	}
	fmt.Printf("Backup: %s\n", file)
	fmt.Printf("  Created: %s\n", manifest.CreatedAt)
	fmt.Printf("  Driver:  %s\n", manifest.Driver)
	fmt.Printf("  Tables:  %d\n", len(manifest.Tables))
	for _, t := range manifest.Tables {
		fmt.Printf("    %s.%s (%s): %d record(s)\n", t.Module, t.Entity, t.Table, t.Count)
	}
}

// matchesFilter reports whether a (module, entity) matches a backup filter
// (4.8.2): "<module>" matches the whole module, "<module>/<entity>" matches
// one entity. Empty filter matches everything.
func matchesFilter(module, entity, filter string) bool {
	if filter == "" {
		return true
	}
	if strings.Contains(filter, "/") {
		return filter == module+"/"+entity
	}
	return filter == module
}

// loadRegistry opens the database and loads the entity registry (shared by
// backup create and restore).
func loadRegistry(specPath, dsn string) (*entity.Registry, db.DB, string) {
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
	for _, loadErr := range reg.LoadEntities() {
		fmt.Fprintf(os.Stderr, "formspec: load warning: %v\n", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: sync schema: %v\n", err)
		os.Exit(1)
	}
	return reg, database, string(driver)
}

// listAll fetches every record of an entity (paginated).
func listAll(ctx context.Context, store *db.EntityStore, workspaceID string) ([]db.EntityRecord, error) {
	var all []db.EntityRecord
	page := 1
	for {
		res, err := store.List(ctx, db.ListParams{WorkspaceID: workspaceID, Page: page, PerPage: 1000})
		if err != nil {
			return nil, err
		}
		all = append(all, res.Data...)
		if page >= res.TotalPages {
			break
		}
		page++
	}
	return all, nil
}

// writeJSONL writes each record as one JSON line into a tar entry.
func writeJSONL(tw *tar.Writer, name string, records []db.EntityRecord) error {
	var buf []byte
	for _, r := range records {
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		buf = append(buf, b...)
		buf = append(buf, '\n')
	}
	return writeBytes(tw, name, buf)
}

func writeBytes(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), ModTime: time.Now()}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func manifestRecordCount(m BackupManifest) int {
	n := 0
	for _, t := range m.Tables {
		n += t.Count
	}
	return n
}

// ─── restore ───

func runRestore(args []string) {
	from := ""
	specPath := "spec"
	dsn := "sqlite:.formspec/data.db"
	conflict := "skip"
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from":
			if i+1 < len(args) {
				from = args[i+1]
				i++
			}
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
		case "--conflict":
			if i+1 < len(args) {
				conflict = args[i+1]
				i++
			}
		case "--dry-run":
			dryRun = true
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec restore --from <file> [--conflict skip|overwrite|remap] [--dry-run] [--spec <path>] [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "formspec restore: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}
	if from == "" {
		fmt.Fprintf(os.Stderr, "formspec restore: --from <file> is required\n")
		os.Exit(2)
	}
	if conflict != "skip" && conflict != "overwrite" && conflict != "remap" {
		fmt.Fprintf(os.Stderr, "formspec restore: --conflict must be skip|overwrite|remap\n")
		os.Exit(2)
	}

	reg, database, _ := loadRegistry(specPath, dsn)
	defer database.Close()

	ctx := context.Background()
	report := restoreFrom(ctx, reg, from, conflict, dryRun)

	if dryRun {
		// Compatibility report (4.8.3): per-entity breakdown of what would
		// happen, so the operator can decide the conflict mode before
		// committing.
		fmt.Printf("Dry-run compatibility report:\n")
		for _, e := range report.Entities {
			fmt.Printf("  %s/%s: %d restore, %d skip, %d remap, %d fail\n",
				e.Module, e.Entity, e.Restored, e.Skipped, e.Remapped, e.Failed)
		}
		fmt.Printf("Total: %d would be restored, %d skipped, %d remapped, %d failed.\n",
			report.Restored, report.Skipped, report.Remapped, report.Failed)
		return
	}
	fmt.Printf("Restore complete: %d restored, %d skipped, %d remapped, %d failed.\n",
		report.Restored, report.Skipped, report.Remapped, report.Failed)
	if report.Failed > 0 {
		os.Exit(1)
	}

	// Outbox reconciliation pass (4.8.5, MUST): after restore, pending outbox
	// entries must be replayed/verified against the restored state before the
	// workspace resumes serving. Here we report the pending count so the
	// operator can trigger the outbox worker; a full replay is the worker's
	// job (it drains pending entries on start).
	reconcileOutbox(ctx, database)
}

// reconcileOutbox reports pending outbox entries after a restore (4.8.5).
func reconcileOutbox(ctx context.Context, database db.DB) {
	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}
	store := db.NewOutboxStore(database, driver)
	counts, err := store.CountByStatus(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec restore: warning: outbox reconciliation failed: %v\n", err)
		return
	}
	pending := counts["pending"]
	if pending > 0 {
		fmt.Printf("Outbox reconciliation: %d pending event(s) — replay via the outbox worker before resuming service (4.8.5).\n", pending)
	} else {
		fmt.Println("Outbox reconciliation: no pending events.")
	}
}

// RestoreReport summarizes the outcome of a restore, per entity and in total.
type RestoreReport struct {
	Restored int
	Skipped  int
	Remapped int
	Failed   int
	Entities []RestoreEntityReport
}

// RestoreEntityReport is the per-entity breakdown used by the dry-run
// compatibility report (4.8.3).
type RestoreEntityReport struct {
	Module   string
	Entity   string
	Restored int
	Skipped  int
	Remapped int
	Failed   int
}

// restoreFrom reads a backup archive and inserts its records into the target
// database. conflict=skip skips records whose natural key already exists;
// conflict=overwrite updates them; conflict=remap assigns a fresh natural key
// and inserts as a new record. dryRun only reports what would happen.
func restoreFrom(ctx context.Context, reg *entity.Registry, from, conflict string, dryRun bool) RestoreReport {
	var report RestoreReport
	f, err := os.Open(from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: open %s: %v\n", from, err)
		os.Exit(1)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: read tar: %v\n", err)
			report.Failed++
			continue
		}
		if hdr.Name == "manifest.json" || filepath.Ext(hdr.Name) != ".jsonl" {
			continue
		}
		// hdr.Name is "<module>_<entity>.jsonl"
		base := hdr.Name[:len(hdr.Name)-len(".jsonl")]
		module, entityName, ok := splitModuleEntity(base)
		if !ok {
			continue
		}
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: store %s.%s: %v\n", module, entityName, err)
			report.Failed++
			continue
		}
		info, ok := reg.GetEntity(module, entityName)
		if !ok || info.EntitySpec == nil {
			continue
		}
		nkField := info.EntitySpec.NaturalKeyField

		entityReport := &RestoreEntityReport{Module: module, Entity: entityName}

		sc := bufio.NewScanner(tr)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 {
				continue
			}
			var rec map[string]any
			if err := json.Unmarshal(line, &rec); err != nil {
				fmt.Fprintf(os.Stderr, "Error: parse record in %s: %v\n", hdr.Name, err)
				report.Failed++
				entityReport.Failed++
				continue
			}
			// Drop framework-owned columns; keep only business data.
			data := stripReserved(rec)
			if nkField != "" && data[nkField] != nil {
				exists, err := naturalKeyExists(ctx, store, "demo", nkField, data[nkField])
				if err == nil && exists {
					switch conflict {
					case "skip":
						report.Skipped++
						entityReport.Skipped++
						continue
					case "overwrite":
						// overwrite: update the existing record's data
						if !dryRun {
							if err := updateByNaturalKey(ctx, store, "demo", nkField, data[nkField], data); err != nil {
								fmt.Fprintf(os.Stderr, "Error: overwrite %s.%s %s=%v: %v\n", module, entityName, nkField, data[nkField], err)
								report.Failed++
								entityReport.Failed++
								continue
							}
						}
						report.Restored++
						entityReport.Restored++
						continue
					case "remap":
						// remap: assign a fresh natural key and insert as a
						// new record, preserving the existing one.
						if !dryRun {
							newKey, err := remapNaturalKey(ctx, store, "demo", nkField, data[nkField])
							if err != nil {
								fmt.Fprintf(os.Stderr, "Error: remap %s.%s %s=%v: %v\n", module, entityName, nkField, data[nkField], err)
								report.Failed++
								entityReport.Failed++
								continue
							}
							data[nkField] = newKey
							if _, err := store.Insert(ctx, db.InsertParams{WorkspaceID: "demo", CreatedBy: "restore", Data: data}); err != nil {
								fmt.Fprintf(os.Stderr, "Error: insert (remap) %s.%s: %v\n", module, entityName, err)
								report.Failed++
								entityReport.Failed++
								continue
							}
						}
						report.Remapped++
						entityReport.Remapped++
						continue
					}
				}
			}
			if dryRun {
				report.Restored++
				entityReport.Restored++
				continue
			}
			if _, err := store.Insert(ctx, db.InsertParams{WorkspaceID: "demo", CreatedBy: "restore", Data: data}); err != nil {
				fmt.Fprintf(os.Stderr, "Error: insert %s.%s: %v\n", module, entityName, err)
				report.Failed++
				entityReport.Failed++
				continue
			}
			report.Restored++
			entityReport.Restored++
		}
		if err := sc.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: read %s: %v\n", hdr.Name, err)
			report.Failed++
			entityReport.Failed++
		}
		report.Entities = append(report.Entities, *entityReport)
	}
	return report
}

// remapNaturalKey generates a fresh natural key value for a conflicting
// record by appending a numeric suffix ("-r1", "-r2", ...) until it no longer
// collides with an existing record. This lets a restore keep both the
// existing and the incoming record under distinct keys (4.8.3).
func remapNaturalKey(ctx context.Context, store *db.EntityStore, workspaceID, field string, value any) (any, error) {
	base := fmt.Sprintf("%v", value)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-r%d", base, i)
		exists, err := naturalKeyExists(ctx, store, workspaceID, field, candidate)
		if err != nil {
			return nil, err
		}
		if !exists {
			return candidate, nil
		}
	}
}

// splitModuleEntity splits "<module>_<entity>" back into module and entity.
// Entity names are kebab-case; the separator is the last underscore.
func splitModuleEntity(s string) (string, string, bool) {
	idx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '_' {
			idx = i
			break
		}
	}
	if idx <= 0 || idx == len(s)-1 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}

// stripReserved removes framework-owned columns from a restored record,
// keeping only business data for Insert/Update.
func stripReserved(rec map[string]any) map[string]any {
	out := make(map[string]any, len(rec))
	reserved := map[string]bool{
		"id": true, "tenant_id": true, "version": true, "created_at": true,
		"updated_at": true, "created_by": true, "updated_by": true, "doc_status": true,
	}
	for k, v := range rec {
		if !reserved[k] {
			out[k] = v
		}
	}
	return out
}

// updateByNaturalKey finds a record by its natural key and overwrites its data.
func updateByNaturalKey(ctx context.Context, store *db.EntityStore, workspaceID, field string, value any, data map[string]any) error {
	res, err := store.List(ctx, db.ListParams{
		WorkspaceID: workspaceID, Page: 1, PerPage: 1,
		Filters: map[string]db.FilterOp{field: {Op: "eq", Value: value}},
	})
	if err != nil {
		return err
	}
	if len(res.Data) == 0 {
		return fmt.Errorf("record with %s=%v not found", field, value)
	}
	rec := res.Data[0]
	_, err = store.Update(ctx, db.UpdateParams{
		WorkspaceID: workspaceID, ID: rec.ID, Version: rec.Version,
		UpdatedBy: "restore", Data: data,
	})
	return err
}
