// Command `formspec backup` — data lifecycle backup/restore
// (docs/cli-tools/02-formspec-cli.md §6, docs/spec/backend/04-persist-backend.md §3).
//
//	formspec backup create --full [--out <file>] [--spec <path>] [--dsn <dsn>]
//	formspec backup inspect <file>
//	formspec restore --from <file> [--conflict skip|overwrite] [--dry-run] [--spec <path>] [--dsn <dsn>]
//
// Backup format is an open tar archive:
//
//	manifest.json            → {created_at, driver, tables:[{module,entity,table,count}], storage_objects}
//	<module>_<entity>.jsonl  → one flattened record per line (wire shape)
//	storage/<object key>     → ctx.storage bytes, under the key the records reference
//
// File storage (ctx.storage) IS included (gap 10.16): objects are read and
// written through the storage service resolved from the manifests, not from a
// hardcoded `{state}/storage` path, so a `kind: Datastore` backed by
// garage/minio/s3 is covered too.
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
	"sort"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	spec "github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	formspec "github.com/primadi/formspec/resource"
)

// BackupManifest describes the contents of a backup archive.
type BackupManifest struct {
	CreatedAt string        `json:"created_at"`
	Driver    string        `json:"driver"`
	Tables    []BackupTable `json:"tables"`
	// StorageObjects counts the ctx.storage objects carried in this archive.
	// Zero with a non-zero file-field count means the app had no uploads — not
	// that files were skipped (gap 10.16).
	StorageObjects int `json:"storage_objects"`
}

// storageEntryPrefix is the archive path prefix under which ctx.storage objects
// are stored. The object key follows it verbatim
// (`{workspace}/{module}/{entity}/{id}/{field}/{uuid}-{name}`), so restore can
// upload the bytes straight back under the key the records already reference —
// no key remapping, and a record never points at a missing object.
const storageEntryPrefix = "storage/"

// backupStorageService resolves the SAME storage the server reads from: the
// datastore registry built from the manifests, falling back to the filesystem
// under the project-root-anchored state dir. Shared with `formspec seed`
// (gap 10.16 — a second implementation that only works on filesystem is how
// seed assets used to break in prod).
func backupStorageService(specPath, dsn string) (formspec.ObjectStoreStorage, error) {
	res, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		return nil, err
	}
	database, err := db.Open(dsn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = database.Close() }()
	dsReg, err := formspec.NewDatastoreRegistryFromManifests(res.Manifests, database, formspec.StateDirFor(dsn, specPath))
	if err != nil {
		return nil, err
	}
	return formspec.ResolveStorage(dsReg, formspec.StateDirFor(dsn, specPath))
}

// collectStorageKeys returns the ctx.storage object keys referenced by the
// records of every backed-up entity whose fields include a `file`/`attachment`.
//
// Keys come from the record data, not from listing the backend: the Storage
// contract is Upload/Download/Stat only, so no portable listing exists. That is
// also why this is sufficient — a `file` field stores the canonical key, so
// every object a record can reference is nameable.
func collectStorageKeys(ctx context.Context, reg *entity.Registry, tables []BackupTable, filter string) ([]string, error) {
	seen := map[string]bool{}
	var keys []string
	for _, t := range tables {
		if filter != "" && !matchesFilter(t.Module, t.Entity, filter) {
			continue
		}
		info, ok := reg.GetEntity(t.Module, t.Entity)
		if !ok || info.EntitySpec == nil {
			continue
		}
		// Child fields are not walked: a child `file` column is stored in the
		// child table's own rows, which `listAll` (above) already flattened in.
		fileFields := fileFieldNames(info.EntitySpec.Fields)
		if len(fileFields) == 0 {
			continue
		}
		store, err := reg.GetEntityStore(t.Module, t.Entity)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", t.Module, t.Entity, err)
		}
		records, err := listAll(ctx, store, "demo")
		if err != nil {
			return nil, fmt.Errorf("list %s.%s: %w", t.Module, t.Entity, err)
		}
		for _, rec := range records {
			for _, f := range fileFields {
				for _, key := range objectKeysIn(rec.Data[f]) {
					if !seen[key] {
						seen[key] = true
						keys = append(keys, key)
					}
				}
			}
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// fileFieldNames returns the names of `file`/`attachment` fields.
func fileFieldNames(fields []spec.Field) []string {
	var out []string
	for _, f := range fields {
		if f.Type == spec.FieldFile || f.Type == spec.FieldAttachment {
			out = append(out, f.Name)
		}
	}
	return out
}

// objectKeysIn extracts object keys from a file-field value. The stored value
// is one key (string) or several when `max_count > 1` (array). Anything else
// (a stale `{key, filename}` object, null) yields nothing — restore must not
// fail over a shape it does not recognize.
func objectKeysIn(value any) []string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
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
		_, _ = fmt.Fprintf(os.Stderr, "Usage: formspec backup <create|inspect> [flags]\n")
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		runBackupCreate(args[1:])
	case "inspect":
		runBackupInspect(args[1:])
	default:
		_, _ = fmt.Fprintf(os.Stderr, "formspec backup: unknown action %q (want create|inspect)\n", args[0])
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
			_, _ = fmt.Fprintf(os.Stderr, "Usage: formspec backup create --full [--out <file>] [--filter <module|module/entity>] [--spec <path>] [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			_, _ = fmt.Fprintf(os.Stderr, "formspec backup create: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}

	// Anchor relative SQLite DSN ke lokasi spec (plan dsn-spec-anchored.md).
	dsn = resolveDSN(dsn, specPath)

	if !full {
		_, _ = fmt.Fprintf(os.Stderr, "formspec backup create: --full is required (incremental not yet implemented)\n")
		os.Exit(2)
	}
	if out == "" {
		out = fmt.Sprintf("backup-%s.tar", time.Now().Format("2006-01-02"))
	}

	reg, database, driver := loadRegistry(specPath, dsn)
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	manifest := BackupManifest{CreatedAt: time.Now().UTC().Format(time.RFC3339), Driver: driver}

	f, err := os.Create(out)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: create %s: %v\n", out, err)
		os.Exit(1)
	}

	// The writers are closed explicitly on the success path, not deferred: every
	// failure below exits via os.Exit, which skips defers anyway, so a deferred
	// close only ever ran when there was nothing left to do — and then discarded
	// its error. The archive is not on disk until both return, and the "written"
	// message below must not be printed before that.
	tw := tar.NewWriter(f)
	closeAll := func() error {
		return closeInto(closeInto(nil, tw), f)
	}

	for _, info := range reg.ListEntities() {
		// Filterable backup (4.8.2): --filter <module> or <module/entity>.
		if filter != "" && !matchesFilter(info.Module, info.Name, filter) {
			continue
		}
		store, err := reg.GetEntityStore(info.Module, info.Name)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: store %s.%s: %v\n", info.Module, info.Name, err)
			os.Exit(1)
		}
		records, err := listAll(ctx, store, "demo")
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: list %s.%s: %v\n", info.Module, info.Name, err)
			os.Exit(1)
		}
		manifest.Tables = append(manifest.Tables, BackupTable{
			Module: info.Module, Entity: info.Name, Table: info.TableName, Count: len(records),
		})
		if len(records) == 0 {
			continue
		}
		if err := writeJSONL(tw, info.Module+"_"+info.Name+".jsonl", records); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: write %s: %v\n", info.Module+"_"+info.Name, err)
			os.Exit(1)
		}
	}

	mb, _ := json.MarshalIndent(manifest, "", "  ")
	if err := writeBytes(tw, "manifest.json", mb); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: write manifest: %v\n", err)
		os.Exit(1)
	}

	// File storage (ctx.storage) ikut ter-backup (4.8.1, gap 10.16): objek
	// diambil lewat **storage service** (datastore registry), bukan dengan
	// membaca `{state}/storage` langsung. Membaca hardcoded hanya bekerja di
	// dev (filesystem); begitu sebuah `kind: Datastore` menyajikan `storage`
	// dengan driver garage/minio/s3 (jalur prod), seluruh objek `file` tidak
	// ikut ter-backup — dan tidak ada peringatan apa pun.
	//
	// Kunci objek di-enumerasi dari RECORD yang baru saja ditulis, bukan dengan
	// meminta daftar ke backend: kontrak `Storage` hanya Upload/Download/Stat,
	// jadi tidak ada cara portable untuk melisting. Field `file`/`attachment`
	// menyimpan kunci kanoniknya, sehingga data itu memang sumber kebenarannya.
	keys, err := collectStorageKeys(ctx, reg, manifest.Tables, filter)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: enumerate storage objects: %v\n", err)
		os.Exit(1)
	}
	if len(keys) > 0 {
		store, err := backupStorageService(specPath, dsn)
		if err != nil {
			// A backup that silently drops files is worse than one that fails:
			// the operator would only discover it at restore time, when the
			// originals may be gone.
			_, _ = fmt.Fprintf(os.Stderr, "Error: resolve storage service: %v\n", err)
			os.Exit(1)
		}
		for _, key := range keys {
			data, err := store.Download(ctx, key)
			if err != nil {
				// Report and continue: one unreadable object must not abort the
				// whole backup, but it must be visible.
				_, _ = fmt.Fprintf(os.Stderr, "formspec backup: warning: read object %q: %v\n", key, err)
				continue
			}
			if err := writeBytes(tw, storageEntryPrefix+key, data); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: write object %q: %v\n", key, err)
				os.Exit(1)
			}
			manifest.StorageObjects++
		}
	}

	if err := closeAll(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: finalize %s: %v\n", out, err)
		os.Exit(1)
	}

	fmt.Printf("Backup written to %s (%d table(s), %d record(s), %d storage object(s)).\n",
		out, len(manifest.Tables), manifestRecordCount(manifest), manifest.StorageObjects)
}

func runBackupInspect(args []string) {
	if len(args) < 1 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: formspec backup inspect <file>\n")
		os.Exit(2)
	}
	file := args[0]

	f, err := os.Open(file)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: open %s: %v\n", file, err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	tr := tar.NewReader(f)
	var manifest BackupManifest
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: read tar: %v\n", err)
			os.Exit(1)
		}
		if hdr.Name == "manifest.json" {
			b, _ := io.ReadAll(tr)
			if err := json.Unmarshal(b, &manifest); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: parse manifest: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if manifest.CreatedAt == "" {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s is not a valid formspec backup (no manifest.json)\n", file)
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
		_, _ = fmt.Fprintf(os.Stderr, "Error: open database: %v\n", err)
		os.Exit(1)
	}
	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}
	reg := entity.NewRegistry(database, driver, specPath)
	for _, loadErr := range reg.LoadEntities() {
		_, _ = fmt.Fprintf(os.Stderr, "formspec: load warning: %v\n", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: sync schema: %v\n", err)
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
			_, _ = fmt.Fprintf(os.Stderr, "Usage: formspec restore --from <file> [--conflict skip|overwrite|remap] [--dry-run] [--spec <path>] [--dsn <dsn>]\n")
			os.Exit(0)
		default:
			_, _ = fmt.Fprintf(os.Stderr, "formspec restore: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}

	// Anchor relative SQLite DSN ke lokasi spec (plan dsn-spec-anchored.md).
	dsn = resolveDSN(dsn, specPath)

	if from == "" {
		_, _ = fmt.Fprintf(os.Stderr, "formspec restore: --from <file> is required\n")
		os.Exit(2)
	}
	if conflict != "skip" && conflict != "overwrite" && conflict != "remap" {
		_, _ = fmt.Fprintf(os.Stderr, "formspec restore: --conflict must be skip|overwrite|remap\n")
		os.Exit(2)
	}

	reg, database, _ := loadRegistry(specPath, dsn)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	// ctx.storage objects ride along in the archive (gap 10.16). Resolve the
	// same service the server reads from; a restore that writes records but
	// skips their files would leave every `file` field pointing at nothing.
	// A dry run needs no service — it only counts.
	var store formspec.ObjectStoreStorage
	if !dryRun {
		var err error
		store, err = backupStorageService(specPath, dsn)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: resolve storage service: %v\n", err)
			os.Exit(1)
		}
	}

	report, stored, _ := restoreFromWithStorage(ctx, reg, from, conflict, dryRun, store)

	if dryRun {
		// Compatibility report (4.8.3): per-entity breakdown of what would
		// happen, so the operator can decide the conflict mode before
		// committing.
		fmt.Printf("Dry-run compatibility report:\n")
		for _, e := range report.Entities {
			fmt.Printf("  %s/%s: %d restore, %d skip, %d remap, %d fail\n",
				e.Module, e.Entity, e.Restored, e.Skipped, e.Remapped, e.Failed)
		}
		fmt.Printf("Objects: %d would be restored.\n", stored)
		fmt.Printf("Total: %d would be restored, %d skipped, %d remapped, %d failed.\n",
			report.Restored, report.Skipped, report.Remapped, report.Failed)
		return
	}
	fmt.Printf("Restore complete: %d restored, %d skipped, %d remapped, %d failed, %d storage object(s).\n",
		report.Restored, report.Skipped, report.Remapped, report.Failed, stored)
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
		_, _ = fmt.Fprintf(os.Stderr, "formspec restore: warning: outbox reconciliation failed: %v\n", err)
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
	report, _, _ := restoreFromWithStorage(ctx, reg, from, conflict, dryRun, nil)
	return report
}

// RestoreReport mirrors the counts restoreFromWithStorage produces; the storage
// tally is a separate return value because it is not per-entity.
//
// `store` may be nil (dry run, or a caller that only wants records). When it is
// non-nil, `storage/*` entries in the archive are uploaded through the SAME
// storage service the server reads from — previously they were dropped by the
// `.jsonl` filter, so a restore produced records whose `file` fields pointed at
// objects that were never written (gap 10.16).
func restoreFromWithStorage(
	ctx context.Context,
	reg *entity.Registry,
	from, conflict string,
	dryRun bool,
	store formspec.ObjectStoreStorage,
) (RestoreReport, int, error) {
	var report RestoreReport
	stored, failedStored := 0, 0
	f, err := os.Open(from)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: open %s: %v\n", from, err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: read tar: %v\n", err)
			report.Failed++
			continue
		}
		// ctx.storage objects (gap 10.16): the key is the tar path minus the
		// prefix, and it is uploaded verbatim — the records in this same archive
		// already reference exactly that key, so remapping would break them.
		if strings.HasPrefix(hdr.Name, storageEntryPrefix) && store != nil {
			key := strings.TrimPrefix(hdr.Name, storageEntryPrefix)
			if key == "" {
				continue
			}
			if dryRun {
				// Dry run reports what WOULD happen without writing objects.
				stored++
				continue
			}
			body, err := io.ReadAll(tr)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: read object %q: %v\n", key, err)
				failedStored++
				continue
			}
			if err := store.Upload(ctx, key, body); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: upload object %q: %v\n", key, err)
				failedStored++
				continue
			}
			stored++
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
			_, _ = fmt.Fprintf(os.Stderr, "Error: store %s.%s: %v\n", module, entityName, err)
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
				_, _ = fmt.Fprintf(os.Stderr, "Error: parse record in %s: %v\n", hdr.Name, err)
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
								_, _ = fmt.Fprintf(os.Stderr, "Error: overwrite %s.%s %s=%v: %v\n", module, entityName, nkField, data[nkField], err)
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
								_, _ = fmt.Fprintf(os.Stderr, "Error: remap %s.%s %s=%v: %v\n", module, entityName, nkField, data[nkField], err)
								report.Failed++
								entityReport.Failed++
								continue
							}
							data[nkField] = newKey
							if _, err := store.Insert(ctx, db.InsertParams{WorkspaceID: "demo", CreatedBy: "restore", Data: data}); err != nil {
								_, _ = fmt.Fprintf(os.Stderr, "Error: insert (remap) %s.%s: %v\n", module, entityName, err)
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
				_, _ = fmt.Fprintf(os.Stderr, "Error: insert %s.%s: %v\n", module, entityName, err)
				report.Failed++
				entityReport.Failed++
				continue
			}
			report.Restored++
			entityReport.Restored++
		}
		if err := sc.Err(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: read %s: %v\n", hdr.Name, err)
			report.Failed++
			entityReport.Failed++
		}
		report.Entities = append(report.Entities, *entityReport)
	}
	if failedStored > 0 {
		report.Failed += failedStored
	}
	return report, stored, nil
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
