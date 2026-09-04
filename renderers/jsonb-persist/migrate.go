package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// MigrationRecord tracks a completed migration.
type MigrationRecord struct {
	Version     int
	Description string
	Checksum    string // SHA256 of the DDL
	AppliedAt   string
}

// MigrationRunner applies schema migrations for FormSpec entities.
type MigrationRunner struct {
	db     DB
	driver DriverType
	// registry is used by EntityStore/NextKey to satisfy the PersistBackend
	// contract (4.1.3). Set via SetRegistry.
	registry interface {
		GetEntityStore(module, name string) (*EntityStore, error)
		GenerateNaturalKey(ctx context.Context, workspaceID, module, name, fieldName string) (string, error)
	}
}

// NewMigrationRunner creates a new migration runner.
func NewMigrationRunner(db DB, driver DriverType) *MigrationRunner {
	return &MigrationRunner{db: db, driver: driver}
}

// SetRegistry wires an entity registry so the runner can satisfy the
// EntityStore/NextKey parts of the PersistBackend contract (4.1.3).
func (r *MigrationRunner) SetRegistry(reg interface {
	GetEntityStore(module, name string) (*EntityStore, error)
	GenerateNaturalKey(ctx context.Context, workspaceID, module, name, fieldName string) (string, error)
}) {
	r.registry = reg
}

// SyncSchema implements PersistBackend.SyncSchema (4.1.1).
func (r *MigrationRunner) SyncSchema(ctx context.Context, entities []EntityMigration) (int, error) {
	return r.ApplyMigrations(ctx, entities)
}

// PlanSchema implements PersistBackend.PlanSchema (4.1.1).
func (r *MigrationRunner) PlanSchema(ctx context.Context, entities []EntityMigration) ([]DDLResult, error) {
	return r.PlanMigrations(ctx, entities)
}

// DriverName implements PersistBackend.DriverName (4.1.1).
func (r *MigrationRunner) DriverName() string { return string(r.driver) }

// NextKey implements PersistBackend.NextKey (4.1.1) — delegates to the
// registry's natural-key counter (gap-free, atomic).
func (r *MigrationRunner) NextKey(ctx context.Context, workspaceID, module, entity, field string) (string, error) {
	if r.registry == nil {
		return "", fmt.Errorf("next_key: no registry wired — call SetRegistry")
	}
	return r.registry.GenerateNaturalKey(ctx, workspaceID, module, entity, field)
}

// EntityStore implements PersistBackend.EntityStore (4.1.1).
func (r *MigrationRunner) EntityStore(module, entity string) (*EntityStore, error) {
	if r.registry == nil {
		return nil, fmt.Errorf("entity store: no registry wired — call SetRegistry")
	}
	return r.registry.GetEntityStore(module, entity)
}

// Compile-time check: MigrationRunner satisfies the PersistBackend contract.
var _ PersistBackend = (*MigrationRunner)(nil)

// SystemTableDDLs returns the DDL statements for FormSpec system tables.
// Uses dialect-aware SQL that works on both SQLite and PostgreSQL.
func SystemTableDDLs(driver DriverType) []string {
	ts := currentTimestamp(driver)

	return []string{
		// formspec_schema_migrations
		createTableSQL(driver, "formspec_schema_migrations",
			"version     integer     PRIMARY KEY",
			"description text        NOT NULL",
			"checksum    text        NOT NULL",
			fmt.Sprintf("applied_at  %s NOT NULL DEFAULT %s", ts, ts),
		),

		// formspec_natural_key_counters
		`CREATE TABLE IF NOT EXISTS formspec_natural_key_counters (
			tenant_id   text    NOT NULL,
			resource    text    NOT NULL,
			field       text    NOT NULL,
			scope       text    NOT NULL DEFAULT '',
			period      text    NOT NULL DEFAULT '',
			counter     integer NOT NULL DEFAULT 0,
			PRIMARY KEY (tenant_id, resource, field, scope, period)
		);`,

		// formspec_idempotency_keys
		createTableSQL(driver, "formspec_idempotency_keys",
			"tenant_id   text    NOT NULL",
			"action      text    NOT NULL",
			"key         text    NOT NULL",
			"status      text    NOT NULL DEFAULT 'pending'",
			"response    text",
			fmt.Sprintf("expires_at  %s NOT NULL", ts),
			fmt.Sprintf("created_at  %s NOT NULL DEFAULT %s", ts, ts),
			"PRIMARY KEY (tenant_id, action, key)",
		),

		// formspec_outbox — enhanced with backoff strategy + initial delay (2.4.4)
		createTableSQL(driver, "formspec_outbox",
			idColumn(driver),
			"tenant_id       text    NOT NULL",
			"event_name      text    NOT NULL",
			"resource        text    NOT NULL",
			"payload         text    NOT NULL DEFAULT '{}'",
			"status          text    NOT NULL DEFAULT 'pending'",
			"retry_count     integer NOT NULL DEFAULT 0",
			"max_retries     integer NOT NULL DEFAULT 10",
			"backoff         text    NOT NULL DEFAULT 'exponential'", // exponential | linear | fixed (2.4.4)
			"initial_delay_ms integer NOT NULL DEFAULT 1000",         // ms before first retry (2.4.4)
			fmt.Sprintf("created_at      %s NOT NULL DEFAULT %s", ts, ts),
			fmt.Sprintf("next_retry_at   %s NOT NULL DEFAULT %s", ts, ts),
		),

		// formspec_extensions — namespace reservation for entity extensions
		`CREATE TABLE IF NOT EXISTS formspec_extensions (
			resource    text    NOT NULL,
			namespace   text    NOT NULL,
			module      text    NOT NULL,
			status      text    NOT NULL DEFAULT 'active',
			created_at  text    NOT NULL DEFAULT '2026-01-01T00:00:00Z',
			PRIMARY KEY (resource, namespace)
		);`,

		// formspec_audit_log — immutable audit trail for entity operations
		createTableSQL(driver, "formspec_audit_log",
			idColumn(driver),
			"tenant_id   text    NOT NULL",
			"entity      text    NOT NULL",
			"entity_id   text    NOT NULL",
			"action      text    NOT NULL",
			"actor       text    NOT NULL DEFAULT 'system'",
			"changes     text    NOT NULL DEFAULT '{}'",
			"request_id  text    NOT NULL DEFAULT ''",
			fmt.Sprintf("created_at  %s NOT NULL DEFAULT %s", ts, ts),
		),

		// formspec_event_log — durable record of delivered declared business
		// events (deliver: {channel: audit_log}), distinct from
		// formspec_audit_log's CRUD-diff change history.
		createTableSQL(driver, "formspec_event_log",
			idColumn(driver),
			"tenant_id    text    NOT NULL",
			"event_name   text    NOT NULL",
			"resource     text    NOT NULL",
			"payload      text    NOT NULL DEFAULT '{}'",
			fmt.Sprintf("delivered_at %s NOT NULL DEFAULT %s", ts, ts),
		),

		// formspec_workflow_approval — pending/active approval requests for
		// kind: Workflow interception (02-core-extended.md §2). One row per
		// (tenant, entity, record, workflow) while approval is in flight.
		createTableSQL(driver, "formspec_workflow_approval",
			idColumn(driver),
			"tenant_id       text    NOT NULL",
			"entity          text    NOT NULL", // "module.entity"
			"record_id       text    NOT NULL",
			"workflow_module text    NOT NULL",
			"workflow_name   text    NOT NULL",
			"from_state      text    NOT NULL",
			"to_state        text    NOT NULL",
			"requester_id    text    NOT NULL DEFAULT ''",
			"status          text    NOT NULL DEFAULT 'pending'",
			"active_step     integer NOT NULL DEFAULT 0",
			"approvals       text    NOT NULL DEFAULT '{}'",
			"rejected_by     text    NOT NULL DEFAULT ''",
			"reject_step     integer NOT NULL DEFAULT -1",
			"escalated_steps text    NOT NULL DEFAULT '{}'", // stepIdx -> reassign_roles (7.4.4)
			fmt.Sprintf("created_at      %s NOT NULL DEFAULT %s", ts, ts),
			fmt.Sprintf("updated_at      %s NOT NULL DEFAULT %s", ts, ts),
		),

		// formspec_saga_log — records cross-boundary integrator calls and
		// their compensate actions (02-core-extended.md §5, todo 7.7.4). A
		// saga entry is registered when an integrator dispatches a
		// cross-boundary call; on failure the compensate action is invoked.
		createTableSQL(driver, "formspec_saga_log",
			idColumn(driver),
			"tenant_id    text    NOT NULL",
			"source       text    NOT NULL",                   // originating event, e.g. "billing.invoice.on_submit"
			"target       text    NOT NULL",                   // target action, e.g. "gl.journal-entry.create"
			"compensate   text    NOT NULL DEFAULT ''",        // compensate action ref
			"status       text    NOT NULL DEFAULT 'pending'", // pending | compensated | completed
			"error        text    NOT NULL DEFAULT ''",
			fmt.Sprintf("created_at   %s NOT NULL DEFAULT %s", ts, ts),
			fmt.Sprintf("updated_at   %s NOT NULL DEFAULT %s", ts, ts),
		),

		// formspec_job — async job tracking (02-core-extended.md §13, todo
		// 7.13). A tracked async action (`call: async` + `track: true`)
		// creates a row, reports progress via ctx.job.progress, and ends
		// completed/failed — pushed to the `jobs` websocket channel.
		createTableSQL(driver, "formspec_job",
			idColumn(driver),
			"tenant_id     text    NOT NULL",
			"module        text    NOT NULL",
			"entity        text    NOT NULL",
			"action        text    NOT NULL",
			"status        text    NOT NULL DEFAULT 'pending'", // pending | running | completed | failed
			"progress      integer NOT NULL DEFAULT 0",
			"message       text    NOT NULL DEFAULT ''",
			"result        text    NOT NULL DEFAULT '{}'",
			"error         text    NOT NULL DEFAULT ''",
			"callback_url  text    NOT NULL DEFAULT ''", // optional callback webhook (7.13.4)
			fmt.Sprintf("created_at    %s NOT NULL DEFAULT %s", ts, ts),
			fmt.Sprintf("updated_at    %s NOT NULL DEFAULT %s", ts, ts),
		),

		// formspec_storage_link — download-link tokens for file fields
		// (plan: storage-links-plan.md Fase 2, todo 7.17.6). Backs the
		// 1x-download (one_time) and TTL flows: the link route issues a
		// token, the consume route validates/increments atomically, and
		// the sweeper deletes objects whose ttl passed untouched.
		createTableSQL(driver, "formspec_storage_link",
			"token               text    PRIMARY KEY",
			"tenant_id           text    NOT NULL",
			"path                text    NOT NULL", // object key in the storage backend
			fmt.Sprintf("expires_at         %s NOT NULL", ts),
			"max_downloads       integer NOT NULL DEFAULT 0", // 0 = unlimited
			"download_count      integer NOT NULL DEFAULT 0",
			"status              text    NOT NULL DEFAULT 'active'", // active | consumed
			"delete_on_download  integer NOT NULL DEFAULT 0",        // one_time
			"delete_if_untouched integer NOT NULL DEFAULT 0",        // ttl sweeper
			"downloaded_at       text",
			fmt.Sprintf("created_at         %s NOT NULL DEFAULT %s", ts, ts),
		),
	}
}

// createTableSQL builds a CREATE TABLE IF NOT EXISTS statement.
func createTableSQL(_ DriverType, name string, columns ...string) string {
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n\t%s\n);", name, strings.Join(columns, ",\n\t"))
}

// currentTimestamp returns the dialect-appropriate current timestamp expression.
func currentTimestamp(driver DriverType) string {
	if driver == DriverSQLite {
		return "text"
	}
	return "timestamptz"
}

// idColumn returns the dialect-appropriate primary key column.
func idColumn(driver DriverType) string {
	if driver == DriverSQLite {
		return "id  integer PRIMARY KEY AUTOINCREMENT"
	}
	return "id  uuid PRIMARY KEY DEFAULT gen_uuid_v7()"
}

// EnsureSystemTables creates all FormSpec system tables needed for runtime.
func (r *MigrationRunner) EnsureSystemTables(ctx context.Context) error {
	ddls := SystemTableDDLs(r.driver)
	ddlNames := []string{
		"formspec_schema_migrations", "formspec_natural_key_counters",
		"formspec_idempotency_keys", "formspec_outbox",
		"formspec_extensions", "formspec_audit_log", "formspec_event_log",
		"formspec_workflow_approval", "formspec_saga_log", "formspec_job",
		"formspec_storage_link",
	}
	for i, ddl := range ddls {
		if _, err := r.db.ExecContext(ctx, ddl); err != nil {
			name := "system_table"
			if i < len(ddlNames) {
				name = ddlNames[i]
			}
			return fmt.Errorf("create system table %s: %w", name, err)
		}
	}

	// Ensure the escalated_steps column exists on formspec_workflow_approval
	// (todo 7.4.4) — added after the table's initial creation, so existing
	// databases need an ALTER TABLE ADD COLUMN.
	if err := r.ensureWorkflowApprovalColumn(ctx); err != nil {
		return err
	}
	return nil
}

// ensureWorkflowApprovalColumn adds the escalated_steps column to
// formspec_workflow_approval if it is missing (todo 7.4.4).
func (r *MigrationRunner) ensureWorkflowApprovalColumn(ctx context.Context) error {
	existing, err := r.existingColumns(ctx, "", "formspec_workflow_approval")
	if err != nil {
		return fmt.Errorf("ensure escalated_steps: list columns: %w", err)
	}
	if existing["escalated_steps"] {
		return nil
	}
	if _, err := r.db.ExecContext(ctx,
		"ALTER TABLE formspec_workflow_approval ADD COLUMN escalated_steps text NOT NULL DEFAULT '{}'"); err != nil {
		return fmt.Errorf("ensure escalated_steps: add column: %w", err)
	}
	return nil
}

// UninstallExtension drops an entity extension column (ext_{namespace}) and
// marks its namespace as locked so it is never reused (4.3.3). The DROP and
// the namespace-lock update commit in one transaction.
func (r *MigrationRunner) UninstallExtension(ctx context.Context, tableName, namespace string) error {
	if tableName == "" || namespace == "" {
		return fmt.Errorf("uninstall extension: tableName and namespace required")
	}
	col := "ext_" + namespace

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("uninstall extension: begin tx: %w", err)
	}

	// Drop the extension column.
	dropSQL := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, col)
	if _, err := tx.ExecContext(ctx, dropSQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("uninstall extension: drop column: %w", err)
	}

	// Mark the namespace as locked (never reused).
	if _, err := tx.ExecContext(ctx,
		"UPDATE formspec_extensions SET status = 'locked' WHERE namespace = ?", namespace); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("uninstall extension: lock namespace: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("uninstall extension: commit: %w", err)
	}
	return nil
}

// AppliedMigrations returns the list of already-applied migrations.
func (r *MigrationRunner) AppliedMigrations(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT description, checksum FROM formspec_schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var desc, checksum string
		if err := rows.Scan(&desc, &checksum); err != nil {
			return nil, fmt.Errorf("scan migration: %w", err)
		}
		result[desc] = checksum
	}
	return result, nil
}

// RecordMigration records a migration as applied.
func (r *MigrationRunner) RecordMigration(ctx context.Context, version int, desc, checksum string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO formspec_schema_migrations (version, description, checksum) VALUES (?, ?, ?)",
		version, desc, checksum)
	return err
}

// DDLResult holds the result of DDL generation for migration.
type DDLResult struct {
	TableInfo   *TableInfo // nil for extension DDLs
	DDL         string     // Full DDL to execute
	Checksum    string     // SHA256 of DDL
	IsNew       bool       // Whether this is a new table (not an alter)
	Description string     // Migration description (e.g. "entity:billing/invoice" or "extension:billing/invoice-ext->custext")
}

// PlanMigrations compares desired schema against existing tables and returns DDL to apply.
func (r *MigrationRunner) PlanMigrations(ctx context.Context, entities []EntityMigration) ([]DDLResult, error) {
	var results []DDLResult

	applied, err := r.AppliedMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("plan migrations: get applied: %w", err)
	}

	for _, em := range entities {
		// If this is an extension entity, generate ALTER TABLE DDL instead
		if em.EntitySpec.ExtendStorage != nil {
			extInfo, err := GenerateExtensionDDL(em.Metadata, &em.EntitySpec, r.driver)
			if err != nil {
				return nil, fmt.Errorf("plan extensions: %w", err)
			}

			// Validate namespace reservation
			resource := em.EntitySpec.ExtendStorage.Target
			ns := em.EntitySpec.ExtendStorage.Namespace
			var count int
			if err := r.db.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM formspec_extensions WHERE resource = ? AND namespace = ? AND status = 'active'",
				resource, ns).Scan(&count); err == nil && count > 0 {
				return nil, fmt.Errorf("extension namespace %q already reserved for resource %q", ns, resource)
			}

			extDesc := fmt.Sprintf("extension:%s/%s->%s", em.Metadata.Module, em.Metadata.Name, extInfo.Namespace)
			extDDL := extInfo.AlterTableSQL
			for _, idx := range extInfo.CreateIndexSQL {
				extDDL += "\n" + idx
			}

			extChecksum := checksumDDL(extDDL)

			// Check if already applied
			if existingChecksum, ok := applied[extDesc]; ok {
				if existingChecksum == extChecksum {
					continue
				}
				// Checksum mismatch — v1 is add-only, skip
				continue
			}

			results = append(results, DDLResult{
				DDL:         extDDL,
				Checksum:    extChecksum,
				IsNew:       true,
				Description: extDesc,
			})
			continue
		}

		ti, err := GenerateEntityDDL(em.Metadata, &em.EntitySpec, r.driver)
		if err != nil {
			return nil, fmt.Errorf("plan migrations: generate DDL for %s: %w", em.Metadata.Name, err)
		}

		desc := fmt.Sprintf("entity:%s/%s", em.Metadata.Module, em.Metadata.Name)
		ddl := ti.CreateTableSQL

		// Include child table DDLs
		for _, ct := range ti.ChildTables {
			ddl += "\n\n" + ct.CreateTableSQL
		}

		// Include index DDLs
		for _, idx := range ti.CreateIndexSQL {
			ddl += "\n" + idx
		}

		checksum := checksumDDL(ddl)

		// Check if already applied with same checksum
		if existingChecksum, ok := applied[desc]; ok {
			if existingChecksum == checksum {
				continue // Already applied, unchanged
			}
			// Checksum mismatch — the entity changed. Diff the existing table
			// to add generated columns for new indexed/unique/natural-key
			// fields (4.2.1). Field removal/type-change is two-phase (4.2.2)
			// and not auto-applied here.
			alterDDL, added, err := r.diffExistingTable(ctx, ti, em.EntitySpec)
			if err != nil {
				return nil, fmt.Errorf("plan migrations: diff %s: %w", ti.TableName, err)
			}
			if added > 0 {
				results = append(results, DDLResult{
					TableInfo:   ti,
					DDL:         alterDDL,
					Checksum:    checksumDDL(alterDDL),
					IsNew:       false,
					Description: desc,
				})
			}
			continue
		}

		// Check if table already exists (prior version or manual creation)
		exists, err := r.db.HasTable(ctx, ti.Schema, ti.TableName)
		if err != nil {
			return nil, fmt.Errorf("plan migrations: check table %s: %w", ti.TableName, err)
		}

		if exists {
			// Table exists but migration not recorded — diff columns (4.2.1):
			// add generated columns for indexed/unique/natural-key fields that
			// are missing. Field removal/type-change is two-phase (4.2.2) and
			// not auto-applied here.
			alterDDL, added, err := r.diffExistingTable(ctx, ti, em.EntitySpec)
			if err != nil {
				return nil, fmt.Errorf("plan migrations: diff %s: %w", ti.TableName, err)
			}
			if added > 0 {
				results = append(results, DDLResult{
					TableInfo:   ti,
					DDL:         alterDDL,
					Checksum:    checksumDDL(alterDDL),
					IsNew:       false,
					Description: desc,
				})
			}
			continue
		}

		results = append(results, DDLResult{
			TableInfo:   ti,
			DDL:         ddl,
			Checksum:    checksum,
			IsNew:       true,
			Description: desc,
		})
	}

	// Sort for deterministic order
	sort.Slice(results, func(i, j int) bool {
		// Extensions (nil TableInfo) go after regular entities
		if results[i].TableInfo == nil && results[j].TableInfo == nil {
			return results[i].Description < results[j].Description
		}
		if results[i].TableInfo == nil {
			return false
		}
		if results[j].TableInfo == nil {
			return true
		}
		return results[i].TableInfo.TableName < results[j].TableInfo.TableName
	})

	return results, nil
}

// ApplyMigrations plans and applies all pending migrations.
// Returns the number of migrations applied.
func (r *MigrationRunner) ApplyMigrations(ctx context.Context, entities []EntityMigration) (int, error) {
	// First ensure system tables
	if err := r.EnsureSystemTables(ctx); err != nil {
		return 0, fmt.Errorf("apply migrations: ensure system tables: %w", err)
	}

	// Get current migration count for versioning
	var currentVersion int
	err := r.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM formspec_schema_migrations").Scan(&currentVersion)
	if err != nil {
		return 0, fmt.Errorf("apply migrations: get current version: %w", err)
	}

	plans, err := r.PlanMigrations(ctx, entities)
	if err != nil {
		return 0, fmt.Errorf("apply migrations: plan: %w", err)
	}

	applied := 0
	for _, plan := range plans {
		desc := plan.Description
		if desc == "" && plan.TableInfo != nil {
			desc = fmt.Sprintf("entity:%s/%s", plan.TableInfo.Module, plan.TableInfo.Entity)
		}

		// Per-entity migration in one transaction (4.2.3): the DDL and its
		// migration record commit together or not at all — a failure rolls
		// back the whole entity migration.
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return applied, fmt.Errorf("apply migrations: begin tx for %s: %w", desc, err)
		}

		// Execute DDL
		if _, err := tx.ExecContext(ctx, plan.DDL); err != nil {
			_ = tx.Rollback()
			return applied, fmt.Errorf("apply migrations: execute DDL for %s: %w\nDDL: %s", desc, err, plan.DDL)
		}

		// Record migration
		currentVersion++
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO formspec_schema_migrations (version, description, checksum) VALUES (?, ?, ?)",
			currentVersion, desc, plan.Checksum); err != nil {
			_ = tx.Rollback()
			return applied, fmt.Errorf("apply migrations: record %s: %w", desc, err)
		}

		// If this is an extension migration, record namespace reservation
		if strings.HasPrefix(desc, "extension:") {
			// Parse extension info from description: extension:module/name->namespace
			parts := strings.Split(desc, "->")
			if len(parts) == 2 {
				ns := parts[1]
				// module/name part
				extParts := strings.Split(strings.TrimPrefix(parts[0], "extension:"), "/")
				if len(extParts) == 2 {
					module := extParts[0]
					entityName := extParts[1]
					// Find target from the original entity migration
					var target string
					for _, em := range entities {
						if em.Metadata.Module == module && em.Metadata.Name == entityName {
							if em.EntitySpec.ExtendStorage != nil {
								target = em.EntitySpec.ExtendStorage.Target
							}
							break
						}
					}
					if target != "" {
						_, _ = tx.ExecContext(ctx,
							"INSERT OR IGNORE INTO formspec_extensions (resource, namespace, module) VALUES (?, ?, ?)",
							target, ns, module)
					}
				}
			}
		}

		if err := tx.Commit(); err != nil {
			return applied, fmt.Errorf("apply migrations: commit %s: %w", desc, err)
		}

		applied++
	}

	return applied, nil
}

// diffExistingTable compares an existing table's columns against the desired
// entity spec and returns ALTER TABLE DDL for missing generated columns
// (indexed/unique/natural-key fields). Returns the number of columns added.
func (r *MigrationRunner) diffExistingTable(ctx context.Context, ti *TableInfo, entity spec.EntitySpec) (string, int, error) {
	existing, err := r.existingColumns(ctx, ti.Schema, ti.TableName)
	if err != nil {
		return "", 0, err
	}

	var alters []string
	added := 0
	for _, f := range entity.Fields {
		if f.Type == spec.FieldChild || f.Type == spec.FieldRelation {
			continue
		}
		if !(f.Index || f.Unique || f.NaturalKey) {
			continue
		}
		col := generatedColumnName(f.Name)
		if existing[col] {
			continue
		}
		sqlType := fieldTypeToSQL(f.Type, f.EnumValues)
		// Note: the modernc SQLite driver cannot ALTER TABLE ADD COLUMN with
		// a GENERATED ALWAYS AS column (it silently no-ops), so the diff adds
		// a plain column. On Postgres a generated column is used.
		colDef := fmt.Sprintf("%s %s", col, sqlType)
		if r.driver == DriverPostgres {
			colDef = generateGeneratedColumn(f.Name, sqlType, r.driver)
		}
		alters = append(alters, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;",
			qualifiedName(ti.Schema, ti.TableName, r.driver), colDef))
		added++
	}
	return strings.Join(alters, "\n"), added, nil
}

// existingColumns returns the set of column names present in a table.
func (r *MigrationRunner) existingColumns(ctx context.Context, schema, table string) (map[string]bool, error) {
	cols := make(map[string]bool)
	var rows *sql.Rows
	var err error

	if r.driver == DriverPostgres {
		rows, err = r.db.QueryContext(ctx,
			"SELECT column_name FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2",
			schema, table)
	} else {
		// Use table_xinfo, not table_info: SQLite's table_info hides generated
		// columns (GENERATED ALWAYS AS ... STORED), which would make the diff
		// think indexed/unique generated columns are missing and try to ADD
		// them again → "duplicate column name" error. table_xinfo includes them.
		rows, err = r.db.QueryContext(ctx,
			"SELECT name FROM pragma_table_xinfo(?)", table)
	}
	if err != nil {
		return nil, fmt.Errorf("list columns %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// EntityMigration holds an entity manifest for migration.
type EntityMigration struct {
	Metadata   spec.Metadata
	EntitySpec spec.EntitySpec
}

// checksumDDL computes a SHA256 checksum of DDL string.
func checksumDDL(ddl string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(ddl)))
	return fmt.Sprintf("%x", h)
}
