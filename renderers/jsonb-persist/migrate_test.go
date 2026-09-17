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
	defer func() { _ = d.Close() }()

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
	defer func() { _ = d.Close() }()

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
	defer func() { _ = d.Close() }()

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
	defer func() { _ = d.Close() }()

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
					{Name: "total", Type: spec.FieldDecimal, Required: true},
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
	defer func() { _ = d.Close() }()

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
	defer func() { _ = d.Close() }()

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
	defer func() { _ = d.Close() }()

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

	// Now simulate a modified entity (a new payload field).
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

	// The change is recorded even though it needs no DDL: a payload-only field
	// lives inside `data`, so the storage statement set is identical — but the
	// entity's *contract* changed, and the migration record is what makes
	// `formspec diff` able to say so. The old behaviour (silently skipping any
	// change that needed no DDL) is exactly what hid removed fields.
	applied2, err := r.ApplyMigrations(ctx, entities2)
	if err != nil {
		t.Fatalf("second apply failed: %v", err)
	}
	if applied2 != 1 {
		t.Errorf("expected 1 migration for the changed contract, got %d", applied2)
	}

	// Applying the same spec again is a no-op.
	applied3, err := r.ApplyMigrations(ctx, entities2)
	if err != nil {
		t.Fatalf("third apply failed: %v", err)
	}
	if applied3 != 0 {
		t.Errorf("expected 0 migrations after convergence, got %d", applied3)
	}
}

func TestMigrationRunner_FieldAddDiff(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_fieldadd.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

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
			_ = rows.Scan(&n)
			names = append(names, n)
		}
		_ = rows.Close()
		t.Fatalf("expected _code column added, got %d; columns: %v", colCount, names)
	}
}

// TestMigrationRunner_NewDeclaredIndexReachesExistingTable covers the S8 half
// that is easy to miss: adding `indexes:` to a manifest whose table already
// exists must actually create the index.
//
// The diff path only ever reconciled *columns*, so a uniqueness rule declared
// after the table was created was silently never enforced on any database in
// use — the manifest looked correct and `formspec validate` passed, while the
// business rule did not exist. The test also pins convergence: a second plan
// must be empty, or every `formspec migrate plan` would emit the same index
// forever.
func TestMigrationRunner_NewDeclaredIndexReachesExistingTable(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_index.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	mk := func(indexes []spec.IndexDecl) []EntityMigration {
		return []EntityMigration{{
			Metadata: spec.Metadata{Name: "shift", Module: "cafe-order"},
			EntitySpec: spec.EntitySpec{
				Version: "v1",
				Plural:  "shifts",
				Fields: []spec.Field{
					{Name: "branch_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.branch"}},
					{Name: "cashier_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.employee"}},
					{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"open", "closed"}},
				},
				Indexes: indexes,
			},
		}}
	}

	// Table created without the rule.
	if _, err := r.ApplyMigrations(ctx, mk(nil)); err != nil {
		t.Fatalf("initial apply: %v", err)
	}
	if indexExists(t, d, "idx_cafe_order_shifts_branch_id_cashier_id") {
		t.Fatal("index should not exist before it is declared")
	}

	// The rule is declared: partial unique index, exactly as the partial-index
	// construct renders it.
	withIndex := mk([]spec.IndexDecl{{
		Fields: []string{"branch_id", "cashier_id"},
		Unique: true,
		Where:  "status = 'open'",
	}})

	results, err := r.PlanMigrations(ctx, withIndex)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected a migration for the newly declared index, got none")
	}
	const want = "CREATE UNIQUE INDEX IF NOT EXISTS idx_cafe_order_shifts_branch_id_cashier_id ON cafe_order_shifts (_branch_id, _cashier_id) WHERE _status = 'open';"
	if !strings.Contains(results[0].DDL, want) {
		t.Errorf("plan DDL should contain %q\ngot: %s", want, results[0].DDL)
	}

	if _, err := r.ApplyMigrations(ctx, withIndex); err != nil {
		t.Fatalf("apply with index: %v", err)
	}
	if !indexExists(t, d, "idx_cafe_order_shifts_branch_id_cashier_id") {
		t.Fatal("index was not created on the existing table")
	}

	// Converged: the index now exists, so nothing more to plan.
	results, err = r.PlanMigrations(ctx, withIndex)
	if err != nil {
		t.Fatalf("plan after apply: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 migrations once converged, got %d: %q", len(results), results[0].DDL)
	}
}

// indexExists reports whether the SQLite table has an index with the given name.
func indexExists(t *testing.T, d DB, name string) bool {
	t.Helper()
	var found int
	if err := d.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", name).Scan(&found); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	return found > 0
}

func TestMigrationRunner_EnumChangeNoDuplicateColumn(t *testing.T) { // Regression: changing an enum value list changes the DDL checksum (the
	// CHECK constraint), which triggers diffExistingTable. The indexed enum
	// field's generated column (_status) must be detected as already present
	// via table_xinfo; otherwise the diff tries to ADD COLUMN it again and
	// SQLite fails with "duplicate column name".
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_enum.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	mk := func(vals []string) []EntityMigration {
		return []EntityMigration{
			{
				Metadata: spec.Metadata{Name: "table", Module: "cafe-master"},
				EntitySpec: spec.EntitySpec{
					Version: "v1",
					Fields: []spec.Field{
						{Name: "code", Type: spec.FieldString, Unique: true},
						{Name: "status", Type: spec.FieldEnum, Index: true, EnumValues: vals},
					},
				},
			},
		}
	}

	// Initial apply with generated columns for code + status.
	if _, err := r.ApplyMigrations(ctx, mk([]string{"available", "occupied", "reserved"})); err != nil {
		t.Fatalf("initial apply: %v", err)
	}

	// Add an enum value → checksum changes → diff path runs.
	entities2 := mk([]string{"available", "occupied", "reserved", "not_available"})
	results, err := r.PlanMigrations(ctx, entities2)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 diff migrations (generated columns already exist), got %d: %q",
			len(results), results[0].DDL)
	}

	// Applying must not error with duplicate column.
	applied, err := r.ApplyMigrations(ctx, entities2)
	if err != nil {
		t.Fatalf("apply after enum change: %v", err)
	}
	if applied != 0 {
		t.Fatalf("expected 0 applied, got %d", applied)
	}
}

func TestMigrationRunner_OutputDDL(t *testing.T) {
	// Verify generated DDL is valid SQL by checking structure
	meta := spec.Metadata{Name: "invoice", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "number", Type: spec.FieldString, Required: true, NaturalKey: true},
			{Name: "total", Type: spec.FieldDecimal, Required: true},
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
