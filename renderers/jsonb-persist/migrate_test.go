package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestMigrationRunner_EnsureSystemTables(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_sys.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	// Verify system tables exist
	systemTables := []string{
		"formspec_schema_migrations",
		"formspec_natural_key_counters",
		"formspec_idempotency_keys",
		"formspec_outbox",
		"formspec_extensions",
		"formspec_audit_log",
		"formspec_event_log",
	}
	for _, tbl := range systemTables {
		exists, err := d.HasTable(ctx, "", tbl)
		if err != nil {
			t.Fatalf("HasTable(%s) failed: %v", tbl, err)
		}
		if !exists {
			t.Errorf("expected system table %s to exist", tbl)
		}
	}
}

func TestMigrationRunner_ApplyMigrations_NewEntity(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_new.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	entities := []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "customer", Module: "billing"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "name", Type: spec.FieldString, Required: true},
					{Name: "email", Type: spec.FieldString, Unique: true},
				},
			},
		},
	}

	applied, err := r.ApplyMigrations(ctx, entities)
	if err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}
	if applied != 1 {
		t.Errorf("expected 1 migration applied, got %d", applied)
	}

	// Verify table was created
	exists, err := d.HasTable(ctx, "", "billing_customers")
	if err != nil {
		t.Fatalf("HasTable failed: %v", err)
	}
	if !exists {
		t.Error("expected billing_customers table to exist")
	}

	// Verify migration was recorded
	var count int
	err = d.QueryRowContext(ctx, "SELECT COUNT(*) FROM formspec_schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("count migrations failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 migration record, got %d", count)
	}
}

func TestMigrationRunner_ApplyMigrations_Idempotent(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_idem.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	entities := []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "product", Module: "inventory"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "sku", Type: spec.FieldString, Required: true, Unique: true},
					{Name: "name", Type: spec.FieldString},
				},
			},
		},
	}

	// First run — should apply 1 migration
	applied1, err := r.ApplyMigrations(ctx, entities)
	if err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	if applied1 != 1 {
		t.Errorf("expected 1 migration on first run, got %d", applied1)
	}

	// Second run — should apply 0 (idempotent)
	applied2, err := r.ApplyMigrations(ctx, entities)
	if err != nil {
		t.Fatalf("second apply failed: %v", err)
	}
	if applied2 != 0 {
		t.Errorf("expected 0 migrations on second run, got %d", applied2)
	}
}

func TestMigrationRunner_MultipleEntities(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_multi.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	entities := []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "customer", Module: "billing"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "name", Type: spec.FieldString},
				},
			},
		},
		{
			Metadata: spec.Metadata{Name: "order", Module: "billing"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "total", Type: spec.FieldNumber, Required: true},
					{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"draft", "paid"}},
				},
			},
		},
		{
			Metadata: spec.Metadata{Name: "product", Module: "inventory"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "sku", Type: spec.FieldString, Unique: true},
				},
			},
		},
	}

	applied, err := r.ApplyMigrations(ctx, entities)
	if err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}
	if applied != 3 {
		t.Errorf("expected 3 migrations applied, got %d", applied)
	}

	// Verify all tables exist
	tables := []string{"billing_customers", "billing_orders", "inventory_products"}
	for _, tbl := range tables {
		exists, err := d.HasTable(ctx, "", tbl)
		if err != nil {
			t.Fatalf("HasTable(%s) failed: %v", tbl, err)
		}
		if !exists {
			t.Errorf("expected table %s to exist", tbl)
		}
	}
}

func TestChecksumDDL(t *testing.T) {
	ddl1 := "CREATE TABLE test (id int);"
	ddl2 := "CREATE TABLE test (id int);"
	ddl3 := "CREATE TABLE test (id bigint);"

	c1 := checksumDDL(ddl1)
	c2 := checksumDDL(ddl2)
	c3 := checksumDDL(ddl3)

	if c1 != c2 {
		t.Error("same DDL should produce same checksum")
	}
	if c1 == c3 {
		t.Error("different DDL should produce different checksum")
	}

	if len(c1) != 64 {
		t.Errorf("expected 64-char hex SHA256, got %d", len(c1))
	}
}

func TestMigrationRunner_ExtensionMigration(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_ext.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	// First, create base entity
	baseEntities := []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "customer", Module: "billing"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "name", Type: spec.FieldString},
					{Name: "email", Type: spec.FieldString, Unique: true},
				},
			},
		},
	}

	appliedBase, err := r.ApplyMigrations(ctx, baseEntities)
	if err != nil {
		t.Fatalf("ApplyMigrations base failed: %v", err)
	}
	if appliedBase != 1 {
		t.Errorf("expected 1 base migration, got %d", appliedBase)
	}

	// Now apply extension
	extEntities := []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "custext", Module: "billing"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "loyalty_tier", Type: spec.FieldString, Default: "bronze"},
					{Name: "referral_code", Type: spec.FieldString, Unique: true},
				},
				ExtendStorage: &spec.ExtendStorage{
					Target:    "billing/customer",
					Namespace: "custext",
				},
			},
		},
	}

	appliedExt, err := r.ApplyMigrations(ctx, extEntities)
	if err != nil {
		t.Fatalf("ApplyMigrations extension failed: %v", err)
	}
	if appliedExt != 1 {
		t.Errorf("expected 1 extension migration, got %d", appliedExt)
	}

	// Verify extension column exists
	var colCount int
	err = d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info('billing_customers') WHERE name = 'ext_custext'",
	).Scan(&colCount)
	if err != nil {
		t.Fatalf("query extension column failed: %v", err)
	}
	if colCount != 1 {
		t.Error("expected ext_custext column to exist on billing_customers")
	}

	// Verify formspec_extensions records
	var extCount int
	err = d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM formspec_extensions WHERE resource = 'billing/customer' AND namespace = 'custext'",
	).Scan(&extCount)
	if err != nil {
		t.Fatalf("query formspec_extensions failed: %v", err)
	}
	if extCount != 1 {
		t.Errorf("expected 1 extension record, got %d", extCount)
	}

	// Verify ExtensionStore works
	extStore := NewExtensionStore(d, DriverSQLite, "billing_customers", "custext")
	if extStore.ColumnName() != "ext_custext" {
		t.Errorf("expected column 'ext_custext', got %q", extStore.ColumnName())
	}
}

func TestMigrationRunner_UninstallExtension(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_uninst.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	// Base entity.
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "customer", Module: "billing"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields:  []spec.Field{{Name: "name", Type: spec.FieldString}},
			},
		},
	}); err != nil {
		t.Fatalf("apply base: %v", err)
	}

	// Extension with a plain (non-unique) field so no generated column
	// depends on ext_custext (SQLite DROP COLUMN limitation).
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "custext", Module: "billing"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields:  []spec.Field{{Name: "loyalty_tier", Type: spec.FieldString, Default: "bronze"}},
				ExtendStorage: &spec.ExtendStorage{
					Target:    "billing/customer",
					Namespace: "custext",
				},
			},
		},
	}); err != nil {
		t.Fatalf("apply extension: %v", err)
	}

	// Verify column exists.
	var colCount int
	if err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info('billing_customers') WHERE name = 'ext_custext'",
	).Scan(&colCount); err != nil {
		t.Fatalf("query column: %v", err)
	}
	if colCount != 1 {
		t.Fatalf("expected ext_custext column, got %d", colCount)
	}

	// Uninstall (4.3.3).
	if err := r.UninstallExtension(ctx, "billing_customers", "custext"); err != nil {
		t.Fatalf("UninstallExtension: %v", err)
	}

	// Column dropped.
	if err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info('billing_customers') WHERE name = 'ext_custext'",
	).Scan(&colCount); err != nil {
		t.Fatalf("query after uninstall: %v", err)
	}
	if colCount != 0 {
		t.Fatalf("expected column dropped, got %d", colCount)
	}

	// Namespace locked.
	var status string
	if err := d.QueryRowContext(ctx,
		"SELECT status FROM formspec_extensions WHERE namespace = 'custext'",
	).Scan(&status); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != "locked" {
		t.Fatalf("expected status 'locked', got %q", status)
	}
}

func TestMigrationRunner_ChecksumChange(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_ck.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	entities := []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "item", Module: "test"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "name", Type: spec.FieldString},
				},
			},
		},
	}

	applied, err := r.ApplyMigrations(ctx, entities)
	if err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	if applied != 1 {
		t.Errorf("expected 1 migration, got %d", applied)
	}

	// Now simulate a modified entity (different fields)
	entities2 := []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "item", Module: "test"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "name", Type: spec.FieldString},
					{Name: "description", Type: spec.FieldString}, // new field
				},
			},
		},
	}

	// In v1, modified entities are skipped (add-only migration)
	applied2, err := r.ApplyMigrations(ctx, entities2)
	if err != nil {
		t.Fatalf("second apply failed: %v", err)
	}
	if applied2 != 0 {
		t.Errorf("expected 0 migrations (add-only), got %d", applied2)
	}
}

func TestMigrationRunner_FieldAddDiff(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_fieldadd.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	// Initial entity with a plain field.
	entities := []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "item", Module: "test"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields:  []spec.Field{{Name: "name", Type: spec.FieldString}},
			},
		},
	}
	if _, err := r.ApplyMigrations(ctx, entities); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Add a new indexed field → should generate ALTER TABLE ADD COLUMN.
	entities2 := []EntityMigration{
		{
			Metadata: spec.Metadata{Name: "item", Module: "test"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Fields: []spec.Field{
					{Name: "name", Type: spec.FieldString},
					{Name: "code", Type: spec.FieldString, Index: true},
				},
			},
		},
	}

	results, err := r.PlanMigrations(ctx, entities2)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 diff migration, got %d", len(results))
	}
	if !strings.Contains(results[0].DDL, "ADD COLUMN") {
		t.Fatalf("expected ALTER TABLE ADD COLUMN, got %q", results[0].DDL)
	}
	t.Logf("diff DDL: %s", results[0].DDL)

	// Apply → column added.
	appliedDiff, err := r.ApplyMigrations(ctx, entities2)
	if err != nil {
		t.Fatalf("apply diff: %v", err)
	}
	if appliedDiff != 1 {
		t.Fatalf("expected 1 diff migration applied, got %d", appliedDiff)
	}
	var colCount int
	if err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info('test_items') WHERE name = '_code'",
	).Scan(&colCount); err != nil {
		t.Fatalf("query column: %v", err)
	}
	if colCount != 1 {
		rows, _ := d.QueryContext(ctx, "SELECT name FROM pragma_table_info('test_items')")
		var names []string
		for rows.Next() {
			var n string
			rows.Scan(&n)
			names = append(names, n)
		}
		rows.Close()
		t.Fatalf("expected _code column added, got %d; columns: %v", colCount, names)
	}
}

func TestMigrationRunner_OutputDDL(t *testing.T) {
	// Verify generated DDL is valid SQL by checking structure
	meta := spec.Metadata{Name: "invoice", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "number", Type: spec.FieldString, Required: true, NaturalKey: true},
			{Name: "total", Type: spec.FieldNumber, Required: true},
			{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"draft", "sent", "paid"}},
		},
	}

	ti, err := GenerateEntityDDL(meta, entity, DriverSQLite)
	if err != nil {
		t.Fatalf("GenerateEntityDDL failed: %v", err)
	}

	// Verify DDL structure
	if !strings.HasPrefix(ti.CreateTableSQL, "CREATE TABLE") {
		t.Error("DDL should start with CREATE TABLE")
	}
	if !strings.Contains(ti.CreateTableSQL, "PRIMARY KEY") {
		t.Error("DDL should have PRIMARY KEY")
	}
	if !strings.Contains(ti.CreateTableSQL, "tenant_id") {
		t.Error("DDL should have tenant_id")
	}
}
