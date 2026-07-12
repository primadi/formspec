package db

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/forma/pkg/spec"
)

// MigrationRecord tracks a completed migration.
type MigrationRecord struct {
	Version     int
	Description string
	Checksum    string // SHA256 of the DDL
	AppliedAt   string
}

// MigrationRunner applies schema migrations for Forma entities.
type MigrationRunner struct {
	db     DB
	driver DriverType
}

// NewMigrationRunner creates a new migration runner.
func NewMigrationRunner(db DB, driver DriverType) *MigrationRunner {
	return &MigrationRunner{db: db, driver: driver}
}

// SystemTableDDLs returns the DDL statements for Forma system tables.
// Uses dialect-aware SQL that works on both SQLite and PostgreSQL.
func SystemTableDDLs(driver DriverType) []string {
	ts := currentTimestamp(driver)

	return []string{
		// forma_schema_migrations
		createTableSQL(driver, "forma_schema_migrations",
			"version     integer     PRIMARY KEY",
			"description text        NOT NULL",
			"checksum    text        NOT NULL",
			fmt.Sprintf("applied_at  %s NOT NULL DEFAULT %s", ts, ts),
		),

		// forma_natural_key_counters
		`CREATE TABLE IF NOT EXISTS forma_natural_key_counters (
			tenant_id   text    NOT NULL,
			resource    text    NOT NULL,
			field       text    NOT NULL,
			scope       text    NOT NULL DEFAULT '',
			period      text    NOT NULL DEFAULT '',
			counter     integer NOT NULL DEFAULT 0,
			PRIMARY KEY (tenant_id, resource, field, scope, period)
		);`,

		// forma_idempotency_keys
		createTableSQL(driver, "forma_idempotency_keys",
			"tenant_id   text    NOT NULL",
			"action      text    NOT NULL",
			"key         text    NOT NULL",
			"status      text    NOT NULL DEFAULT 'pending'",
			"response    text",
			fmt.Sprintf("expires_at  %s NOT NULL", ts),
			fmt.Sprintf("created_at  %s NOT NULL DEFAULT %s", ts, ts),
			"PRIMARY KEY (tenant_id, action, key)",
		),

		// forma_outbox
		createTableSQL(driver, "forma_outbox",
			idColumn(driver),
			"tenant_id   text    NOT NULL",
			"event_name  text    NOT NULL",
			"resource    text    NOT NULL",
			"payload     text    NOT NULL DEFAULT '{}'",
			"status      text    NOT NULL DEFAULT 'pending'",
			"retry_count integer NOT NULL DEFAULT 0",
			"max_retries integer NOT NULL DEFAULT 10",
			fmt.Sprintf("created_at      %s NOT NULL DEFAULT %s", ts, ts),
			fmt.Sprintf("next_retry_at   %s NOT NULL DEFAULT %s", ts, ts),
		),

		// forma_extensions — namespace reservation for entity extensions
		`CREATE TABLE IF NOT EXISTS forma_extensions (
			resource    text    NOT NULL,
			namespace   text    NOT NULL,
			module      text    NOT NULL,
			status      text    NOT NULL DEFAULT 'active',
			created_at  text    NOT NULL DEFAULT '2026-01-01T00:00:00Z',
			PRIMARY KEY (resource, namespace)
		);`,

		// forma_audit_log — immutable audit trail for entity operations
		createTableSQL(driver, "forma_audit_log",
			idColumn(driver),
			"tenant_id   text    NOT NULL",
			"entity      text    NOT NULL",
			"entity_id   text    NOT NULL",
			"action      text    NOT NULL",
			"actor       text    NOT NULL DEFAULT 'system'",
			"changes     text    NOT NULL DEFAULT '{}'",
			fmt.Sprintf("created_at  %s NOT NULL DEFAULT %s", ts, ts),
		),

		// forma_event_log — durable record of delivered declared business
		// events (deliver: {channel: audit_log}), distinct from
		// forma_audit_log's CRUD-diff change history.
		createTableSQL(driver, "forma_event_log",
			idColumn(driver),
			"tenant_id    text    NOT NULL",
			"event_name   text    NOT NULL",
			"resource     text    NOT NULL",
			"payload      text    NOT NULL DEFAULT '{}'",
			fmt.Sprintf("delivered_at %s NOT NULL DEFAULT %s", ts, ts),
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

// EnsureSystemTables creates all Forma system tables needed for runtime.
func (r *MigrationRunner) EnsureSystemTables(ctx context.Context) error {
	ddls := SystemTableDDLs(r.driver)
	ddlNames := []string{
		"forma_schema_migrations", "forma_natural_key_counters",
		"forma_idempotency_keys", "forma_outbox",
		"forma_extensions", "forma_audit_log", "forma_event_log",
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
	return nil
}

// AppliedMigrations returns the list of already-applied migrations.
func (r *MigrationRunner) AppliedMigrations(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT description, checksum FROM forma_schema_migrations ORDER BY version")
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
		"INSERT INTO forma_schema_migrations (version, description, checksum) VALUES (?, ?, ?)",
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
				"SELECT COUNT(*) FROM forma_extensions WHERE resource = ? AND namespace = ? AND status = 'active'",
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
			// Checksum mismatch — would need alter, but v1 is add-only
			// Just skip for now
			continue
		}

		// Check if table already exists (prior version or manual creation)
		exists, err := r.db.HasTable(ctx, ti.Schema, ti.TableName)
		if err != nil {
			return nil, fmt.Errorf("plan migrations: check table %s: %w", ti.TableName, err)
		}

		if exists {
			// Table exists but migration not recorded — skip (assume manual)
			// In v1 we're add-only; future versions will diff columns
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
	err := r.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM forma_schema_migrations").Scan(&currentVersion)
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

		// Execute DDL
		if _, err := r.db.ExecContext(ctx, plan.DDL); err != nil {
			return applied, fmt.Errorf("apply migrations: execute DDL for %s: %w\nDDL: %s", desc, err, plan.DDL)
		}

		// Record migration
		currentVersion++
		if err := r.RecordMigration(ctx, currentVersion, desc, plan.Checksum); err != nil {
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
						_, _ = r.db.ExecContext(ctx,
							"INSERT OR IGNORE INTO forma_extensions (resource, namespace, module) VALUES (?, ?, ?)",
							target, ns, module)
					}
				}
			}
		}

		applied++
	}

	return applied, nil
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
