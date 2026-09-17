package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
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
		GenerateNaturalKey(ctx context.Context, workspaceID, module, name, fieldName, scope string) (string, error)
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
	GenerateNaturalKey(ctx context.Context, workspaceID, module, name, fieldName, scope string) (string, error)
}) {
	r.registry = reg
}

// SyncSchema implements PersistBackend.SyncSchema (4.1.1).
func (r *MigrationRunner) SyncSchema(ctx context.Context, entities []EntityMigration) (int, error) {
	return r.ApplySpecSet(ctx, entities)
}

// PlanSchema implements PersistBackend.PlanSchema (4.1.1).
func (r *MigrationRunner) PlanSchema(ctx context.Context, entities []EntityMigration) ([]DDLResult, error) {
	return r.PlanMigrations(ctx, entities)
}

// DriverName implements PersistBackend.DriverName (4.1.1).
func (r *MigrationRunner) DriverName() string { return string(r.driver) }

// NextKey implements PersistBackend.NextKey (4.1.1) — delegates to the
// registry's natural-key counter (gap-free, atomic).
func (r *MigrationRunner) NextKey(ctx context.Context, workspaceID, module, entity, field, scope string) (string, error) {
	if r.registry == nil {
		return "", fmt.Errorf("next_key: no registry wired — call SetRegistry")
	}
	return r.registry.GenerateNaturalKey(ctx, workspaceID, module, entity, field, scope)
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

		// formspec_schema_snapshot — the applied storage shape of each entity
		// (see EntitySnapshot). Records what the schema *was*, not only a
		// checksum of it: telling a removed field from one that never existed is
		// impossible from a checksum alone, and the migration gate has to know
		// which one it is looking at before it can refuse anything.
		createTableSQL(driver, SnapshotTable,
			"module      text NOT NULL",
			"entity      text NOT NULL",
			"driver      text NOT NULL DEFAULT ''",
			"checksum    text NOT NULL",
			"shape       text NOT NULL",
			fmt.Sprintf("updated_at  %s NOT NULL DEFAULT %s", ts, ts),
			"PRIMARY KEY (module, entity)",
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
		"formspec_schema_migrations", SnapshotTable, "formspec_natural_key_counters",
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
	defer func() { _ = rows.Close() }()

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

// MigrationPlan is the pending work for one entity, together with the rated
// differences that justify it (see DiffShapes).
type MigrationPlan struct {
	Result DDLResult
	// Changes are the rated differences. A plan with no changes is either a
	// brand-new table or a baseline adoption.
	Changes []Change
	// Snapshot is the shape to record once this plan has been applied — the
	// baseline the next plan will be diffed against.
	Snapshot *EntitySnapshot
	// Bootstrap marks a plan that adopts the current manifest as the baseline
	// because nothing was recorded for it (a database created before snapshots
	// existed). It must not refuse anything: there is no earlier shape to have
	// changed away from.
	Bootstrap bool
	// Module and Entity identify the manifest this plan belongs to, so the
	// snapshot can be written or forgotten without re-deriving them.
	Module, Entity string
	// ForgetOnly marks a plan whose whole job is to drop a stale snapshot: the
	// manifests no longer declare the entity and its table is gone too, so the
	// operator already did both steps by hand.
	ForgetOnly bool
}

// ApplyResult reports what an apply did.
type ApplyResult struct {
	Applied int
	// Forgotten counts stale snapshots dropped because the entity (and its
	// table) had already been removed by hand.
	Forgotten int
	// Changes lists every rated difference the run considered — the additive
	// ones it applied, the derived ones it rebuilt, and the declared lossy ones
	// it executed. An undeclared lossy change never reaches ApplyResult: it is
	// returned as an error instead.
	Changes []Change
}

// entityChecksum fingerprints everything about an entity that the storage layer
// cares about: the desired shape (fields, indexes, raw_ddl) *and* the manifest's
// declarations. The declarations belong in the fingerprint because adding a
// tombstone changes the work to do (payload cleanup) while leaving the generated
// DDL identical — a checksum over DDL alone would call that "unchanged" and skip
// the cleanup forever.
func entityChecksum(driver DriverType, desired *EntitySnapshot, decls Declarations) string {
	shape, _ := json.Marshal(desired)
	return checksumDDL(string(shape) + "|" + declarationSignature(decls) + "|" + string(driver))
}

// declarationSignature renders declarations in a stable order so the checksum
// depends on their content, never on map iteration order.
func declarationSignature(decls Declarations) string {
	var b strings.Builder
	for _, key := range []string{"removed", "accept_data_loss"} {
		set := decls.Removed
		if key == "accept_data_loss" {
			set = decls.AcceptDataLoss
		}
		names := make([]string, 0, len(set))
		for name := range set {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			b.WriteString(key)
			b.WriteString(":")
			b.WriteString(name)
			b.WriteString("=")
			b.WriteString(set[name])
			b.WriteString(";")
		}
	}
	return b.String()
}

// PlanDetailed compares the desired schema against what is actually applied and
// returns one plan per entity, each carrying the rated differences behind it.
//
// The entity list is treated as partial: a caller may legitimately apply a
// subset (one module, or a single extension on top of an entity applied in an
// earlier call), so an entity with a recorded snapshot that is absent from this
// list is not reported. Use PlanSpecSet when the list is the whole spec, where
// that absence means the entity was removed from the manifests.
//
// The classification is the point. A checksum can only say *that* something
// changed, and the additive-only diff silently ignored everything it could not
// add — which is how a removed field ended up neither migrated nor reported,
// while it kept breaking writes as an unknown field.
func (r *MigrationRunner) PlanDetailed(ctx context.Context, entities []EntityMigration) ([]MigrationPlan, error) {
	var plans []MigrationPlan

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

			plans = append(plans, MigrationPlan{
				Result: DDLResult{
					DDL:         extDDL,
					Checksum:    extChecksum,
					IsNew:       true,
					Description: extDesc,
				},
			})
			continue
		}

		ti, err := GenerateEntityDDL(em.Metadata, &em.EntitySpec, r.driver)
		if err != nil {
			return nil, fmt.Errorf("plan migrations: generate DDL for %s: %w", em.Metadata.Name, err)
		}

		desc := fmt.Sprintf("entity:%s/%s", em.Metadata.Module, em.Metadata.Name)

		// The desired shape and the manifest's declarations are what the diff
		// compares; the create DDL is only needed when the table is new.
		desired, decls := DesiredSnapshot(ti, &em.EntitySpec, r.driver)
		checksum := entityChecksum(r.driver, desired, decls)

		// Check if already applied with same checksum
		if existingChecksum, ok := applied[desc]; ok {
			if existingChecksum == checksum {
				// Unchanged. The snapshot should already exist; when it does not
				// (a database created before snapshots existed), adopt the manifest
				// as the baseline now rather than refusing over a shape nobody
				// recorded.
				_, _, found, err := r.LoadSnapshot(ctx, r.db, em.Metadata.Module, em.Metadata.Name)
				if err != nil {
					return nil, err
				}
				if !found {
					plans = append(plans, MigrationPlan{
						Result:    DDLResult{TableInfo: ti, Checksum: checksum, Description: desc},
						Snapshot:  desired,
						Bootstrap: true,
					})
				}
				continue
			}
			plan, err := r.planEntityChange(ctx, em, ti, desc, checksum, desired, decls)
			if err != nil {
				return nil, err
			}
			plans = append(plans, plan)
			continue
		}

		// Check if table already exists (prior version or manual creation)
		exists, err := r.db.HasTable(ctx, ti.Schema, ti.TableName)
		if err != nil {
			return nil, fmt.Errorf("plan migrations: check table %s: %w", ti.TableName, err)
		}

		if exists {
			plan, err := r.planEntityChange(ctx, em, ti, desc, checksum, desired, decls)
			if err != nil {
				return nil, err
			}
			plans = append(plans, plan)
			continue
		}

		// New table: everything it needs is additive.
		plans = append(plans, MigrationPlan{
			Result: DDLResult{
				TableInfo:   ti,
				DDL:         createDDLFor(ti, &em.EntitySpec, r.driver),
				Checksum:    checksum,
				IsNew:       true,
				Description: desc,
			},
			Changes: []Change{{
				Class:  ClassAdditive,
				Kind:   ChangeTableAdded,
				Entity: desc,
				Name:   ti.TableName,
				Detail: "table does not exist yet",
			}},
			Snapshot: desired,
		})
	}

	// Sort for deterministic order
	sort.Slice(plans, func(i, j int) bool {
		a, b := plans[i].Result, plans[j].Result
		// Extensions (nil TableInfo) go after regular entities
		if a.TableInfo == nil && b.TableInfo == nil {
			return a.Description < b.Description
		}
		if a.TableInfo == nil {
			return false
		}
		if b.TableInfo == nil {
			return true
		}
		return a.TableInfo.TableName < b.TableInfo.TableName
	})

	return plans, nil
}

// PlanSpecSet plans against the complete spec set: entities recorded in the
// database whose manifest is gone are reported too. That is the only place a
// table drop can be detected, and a table drop is the one change no declaration
// can make acceptable.
func (r *MigrationRunner) PlanSpecSet(ctx context.Context, entities []EntityMigration) ([]MigrationPlan, error) {
	plans, err := r.PlanDetailed(ctx, entities)
	if err != nil {
		return nil, err
	}
	forgotten, err := r.planForgotten(ctx, entities)
	if err != nil {
		return nil, err
	}
	return append(plans, forgotten...), nil
}

// PlanMigrations returns only the statements to run, without the classification
// that justifies them. It is the compatibility shape for callers that just want
// DDL text (`formspec diff` and the older CLI path).
func (r *MigrationRunner) PlanMigrations(ctx context.Context, entities []EntityMigration) ([]DDLResult, error) {
	plans, err := r.PlanSpecSet(ctx, entities)
	if err != nil {
		return nil, err
	}
	results := make([]DDLResult, 0, len(plans))
	for _, p := range plans {
		results = append(results, p.Result)
	}
	return results, nil
}

// ApplyMigrations plans and applies all pending migrations.
// Returns the number of migrations applied.
func (r *MigrationRunner) ApplyMigrations(ctx context.Context, entities []EntityMigration) (int, error) {
	res, err := r.ApplyDetailed(ctx, entities)
	return res.Applied, err
}

// ApplySpecSet applies a plan over the complete spec set — the entry point for
// the normal paths (dev startup, `formspec migrate`), where an entity whose
// manifest disappeared must be refused rather than ignored. The entity list is
// authoritative here, so a missing manifest is a removed entity.
func (r *MigrationRunner) ApplySpecSet(ctx context.Context, entities []EntityMigration) (int, error) {
	res, err := r.ApplySpecSetDetailed(ctx, entities)
	return res.Applied, err
}

// ApplySpecSetDetailed is ApplySpecSet with the rated changes it considered, so
// a CLI can report what it did instead of only how many records it wrote.
func (r *MigrationRunner) ApplySpecSetDetailed(ctx context.Context, entities []EntityMigration) (ApplyResult, error) {
	return r.applyPlans(ctx, entities, true)
}

// ApplyDetailed plans, refuses, and applies — in that order.
//
// The refusal happens before any statement runs, over the whole plan: a run that
// applies the safe half of a change set and then stops has left the database in
// a state no manifest describes, which is worse than refusing up front.
func (r *MigrationRunner) ApplyDetailed(ctx context.Context, entities []EntityMigration) (ApplyResult, error) {
	return r.applyPlans(ctx, entities, false)
}

func (r *MigrationRunner) applyPlans(ctx context.Context, entities []EntityMigration, completeSpecSet bool) (ApplyResult, error) {
	// First ensure system tables
	if err := r.EnsureSystemTables(ctx); err != nil {
		return ApplyResult{}, fmt.Errorf("apply migrations: ensure system tables: %w", err)
	}

	plan := r.PlanDetailed
	if completeSpecSet {
		plan = r.PlanSpecSet
	}
	plans, err := plan(ctx, entities)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("apply migrations: plan: %w", err)
	}

	// Gate: refuse undeclared lossy changes (field removals, unverifiable type
	// changes, unique indexes blocked by duplicates) and table drops. This is
	// the same in dev and in production — a rule that bends under convenience is
	// the rule that silently eats data.
	var allChanges []Change
	for _, plan := range plans {
		allChanges = append(allChanges, plan.Changes...)
	}
	if err := RefuseUndeclared(allChanges); err != nil {
		return ApplyResult{Changes: allChanges}, fmt.Errorf("apply migrations: %w", err)
	}

	// Get current migration count for versioning
	var currentVersion int
	if err := r.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM formspec_schema_migrations").Scan(&currentVersion); err != nil {
		return ApplyResult{}, fmt.Errorf("apply migrations: get current version: %w", err)
	}

	applied := 0
	forgotten := 0
	for _, plan := range plans {
		res := plan.Result
		desc := res.Description
		if desc == "" && res.TableInfo != nil {
			desc = fmt.Sprintf("entity:%s/%s", res.TableInfo.Module, res.TableInfo.Entity)
		}

		// A stale snapshot with no table left behind it: the operator removed
		// the manifest and dropped the table by hand, so there is nothing to
		// migrate — only bookkeeping to clear so the next plan is not refused
		// over work that no longer exists.
		if plan.ForgetOnly {
			if err := r.ForgetSnapshot(ctx, r.db, plan.Module, plan.Entity); err != nil {
				return ApplyResult{Applied: applied, Forgotten: forgotten, Changes: allChanges}, err
			}
			forgotten++
			continue
		}

		// Per-entity migration in one transaction (4.2.3): the DDL, its
		// migration record, and the new snapshot commit together or not at all
		// — a failure rolls back the whole entity migration. A snapshot that
		// survived a rolled-back migration would claim a shape the database
		// never had, and the next run would trust it.
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return ApplyResult{Applied: applied, Changes: allChanges}, fmt.Errorf("apply migrations: begin tx for %s: %w", desc, err)
		}

		// Execute DDL. A plan can carry no statement at all — a bootstrap that
		// only adopts a baseline, or a change whose only effect is a notice.
		if strings.TrimSpace(res.DDL) != "" {
			if _, err := tx.ExecContext(ctx, res.DDL); err != nil {
				_ = tx.Rollback()
				return ApplyResult{Applied: applied, Changes: allChanges},
					fmt.Errorf("apply migrations: execute DDL for %s: %w\nDDL: %s", desc, err, res.DDL)
			}
		}

		// Record migration
		currentVersion++
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO formspec_schema_migrations (version, description, checksum) VALUES (?, ?, ?)",
			currentVersion, desc, res.Checksum); err != nil {
			_ = tx.Rollback()
			return ApplyResult{Applied: applied, Changes: allChanges}, fmt.Errorf("apply migrations: record %s: %w", desc, err)
		}

		// Record the shape this entity now has — the baseline the next plan
		// diffs against.
		if plan.Snapshot != nil && plan.Result.TableInfo != nil {
			module := plan.Result.TableInfo.Module
			entity := plan.Result.TableInfo.Entity
			if err := r.SaveSnapshot(ctx, tx, module, entity, res.Checksum, plan.Snapshot); err != nil {
				_ = tx.Rollback()
				return ApplyResult{Applied: applied, Changes: allChanges}, fmt.Errorf("apply migrations: %w", err)
			}
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
			return ApplyResult{Applied: applied, Changes: allChanges}, fmt.Errorf("apply migrations: commit %s: %w", desc, err)
		}

		applied++
	}

	return ApplyResult{Applied: applied, Forgotten: forgotten, Changes: allChanges}, nil
}

// diffExistingTable compares an existing table's columns against the desired
// entity spec and returns ALTER TABLE DDL for missing generated columns
// (indexed/unique/natural-key fields) plus CREATE INDEX for declared indexes
// that may not exist yet. Returns the number of schema changes emitted.
func (r *MigrationRunner) diffExistingTable(ctx context.Context, ti *TableInfo, entity spec.EntitySpec) (string, int, error) {
	existing, err := r.existingColumns(ctx, ti.Schema, ti.TableName)
	if err != nil {
		return "", 0, err
	}

	// Columns that must exist: every field the DDL generation projects into a
	// real column. `derivedColumnFields` is the single definition of that rule —
	// the snapshot uses it too, and two copies would eventually disagree about
	// which fields have a column.
	needed := make(map[string]bool)
	var ordered []spec.Field
	byName := make(map[string]spec.Field, len(entity.Fields))
	for _, f := range entity.Fields {
		byName[f.Name] = f
	}
	for _, name := range derivedColumnFields(&entity) {
		f, ok := byName[name]
		if !ok || needed[name] {
			continue
		}
		needed[name] = true
		ordered = append(ordered, f)
	}

	var alters []string
	added := 0
	for _, f := range ordered {
		col := generatedColumnName(f.Name)
		if existing[col] {
			continue
		}
		// Note: the modernc SQLite driver cannot ALTER TABLE ADD COLUMN with
		// a GENERATED ALWAYS AS column (it silently no-ops), so SQLite gets a
		// plain column and PostgreSQL a generated one — see addDerivedColumnSQL.
		alters = append(alters, addDerivedColumnSQL(ti, f, r.driver))
		added++
	}

	// Declared indexes on an existing table. Without this, an `indexes:` entry
	// added to a manifest after the table was created was never actually
	// created — the diff only ever reconciled *columns*, so composite
	// uniqueness stayed unenforced on any database already in use.
	//
	// Only indexes that are genuinely absent are emitted, so a converged schema
	// still plans zero migrations (the plan is the gate CI uses). `IF NOT
	// EXISTS` covers the race where the index appears between the check and the
	// apply; both SQLite (≥3.8) and PostgreSQL (≥9.5) support it.
	existingIdx, err := r.existingIndexes(ctx, ti.Schema, ti.TableName)
	if err != nil {
		return "", 0, err
	}
	for _, idx := range ti.CreateIndexSQL {
		if name := indexNameOf(idx); name != "" && existingIdx[name] {
			continue
		}
		alters = append(alters, withIfNotExists(idx))
		added++
	}

	return strings.Join(alters, "\n"), added, nil
}

// withIfNotExists rewrites a CREATE INDEX statement into its idempotent form.
func withIfNotExists(stmt string) string {
	trimmed := strings.TrimSpace(stmt)
	upper := strings.ToUpper(trimmed)
	switch {
	case strings.HasPrefix(upper, "CREATE UNIQUE INDEX "):
		return "CREATE UNIQUE INDEX IF NOT EXISTS " + trimmed[len("CREATE UNIQUE INDEX "):]
	case strings.HasPrefix(upper, "CREATE INDEX "):
		return "CREATE INDEX IF NOT EXISTS " + trimmed[len("CREATE INDEX "):]
	}
	return trimmed
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
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// existingIndexes returns the set of index names present in a table.
func (r *MigrationRunner) existingIndexes(ctx context.Context, schema, table string) (map[string]bool, error) {
	names := make(map[string]bool)
	var rows *sql.Rows
	var err error

	if r.driver == DriverPostgres {
		rows, err = r.db.QueryContext(ctx,
			"SELECT indexname FROM pg_indexes WHERE schemaname = $1 AND tablename = $2",
			schema, table)
	} else {
		rows, err = r.db.QueryContext(ctx,
			"SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = ?", table)
	}
	if err != nil {
		return nil, fmt.Errorf("list indexes %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan index: %w", err)
		}
		names[name] = true
	}
	return names, rows.Err()
}

// indexNameOf extracts the index name from a CREATE [UNIQUE] INDEX statement.
// Returns "" when the statement is not one (in which case the caller emits it
// unchanged — better an idempotent re-apply than a silently skipped statement).
func indexNameOf(stmt string) string {
	trimmed := strings.TrimSpace(stmt)
	trimmed = strings.TrimSuffix(trimmed, ";")
	fields := strings.Fields(trimmed)
	for i, f := range fields {
		if strings.EqualFold(f, "INDEX") && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
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
