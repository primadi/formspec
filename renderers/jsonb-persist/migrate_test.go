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
	// `pragma_table_xinfo`, not `table_info`: SQLite hides generated columns
	// from table_info, and the ALTER path materializes `_code` as a generated
	// (VIRTUAL) column so it is populated for existing and future rows alike.
	if err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_xinfo('test_items') WHERE name = '_code'",
	).Scan(&colCount); err != nil {
		t.Fatalf("query column: %v", err)
	}
	if colCount != 1 {
		rows, _ := d.QueryContext(ctx, "SELECT name FROM pragma_table_xinfo('test_items')")
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

// TestMigrationRunner_ChangedIndexDefinitionIsRebuilt pins the other half of the
// index diff (kafe TODO 3.9, found while verifying 3.8): an index that *exists*
// can still be the wrong index.
//
// The diff used to reconcile by name alone, so when a manifest changed what an
// index covers — 3.6 grew the natural-key index from `(tenant_id, _number)` to
// `(tenant_id, _branch_id, _number)` so each branch gets its own order sequence
// — every database already in use kept the old index and rejected rows the
// manifest says are valid (`UNIQUE constraint failed: … tenant_id, _number`).
// A fresh database was fine, which is exactly why it went unnoticed.
func TestMigrationRunner_ChangedIndexDefinitionIsRebuilt(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_index_shape.db"), nil)
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

	// Table with the *old shape* of the rule, then the stale index under the
	// name the new manifest will use: a database created before the manifest
	// changed looks like this — the index exists, so the old diff skipped it,
	// while it enforces less than the manifest now says.
	if _, err := r.ApplyMigrations(ctx, mk([]spec.IndexDecl{{
		Fields: []string{"branch_id"},
		Unique: true,
	}})); err != nil {
		t.Fatalf("initial apply: %v", err)
	}
	if _, err := d.ExecContext(ctx,
		"CREATE UNIQUE INDEX idx_cafe_order_shifts_branch_id_cashier_id ON cafe_order_shifts (_branch_id)"); err != nil {
		t.Fatalf("seed stale index: %v", err)
	}

	// The manifest now covers (branch_id, cashier_id) and only `open` rows.
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
		t.Fatal("expected the changed index definition to produce a migration, got none")
	}
	ddl := results[0].DDL
	if !strings.Contains(ddl, "DROP INDEX IF EXISTS idx_cafe_order_shifts_branch_id_cashier_id") {
		t.Errorf("plan must drop the stale index before rebuilding it\ngot: %s", ddl)
	}
	if !strings.Contains(ddl, "CREATE UNIQUE INDEX idx_cafe_order_shifts_branch_id_cashier_id ON cafe_order_shifts (_branch_id, _cashier_id) WHERE _status = 'open';") {
		t.Errorf("plan must recreate the index with its new shape\ngot: %s", ddl)
	}

	if _, err := r.ApplyMigrations(ctx, withIndex); err != nil {
		t.Fatalf("apply with changed index: %v", err)
	}

	var rebuilt string
	if err := d.QueryRowContext(ctx,
		"SELECT COALESCE(sql, '') FROM sqlite_master WHERE type = 'index' AND name = 'idx_cafe_order_shifts_branch_id_cashier_id'").
		Scan(&rebuilt); err != nil {
		t.Fatalf("read rebuilt index: %v", err)
	}
	if !strings.Contains(rebuilt, "_cashier_id") {
		t.Errorf("index still has its old definition: %s", rebuilt)
	}

	// Converged: a rebuild that repeats on every plan would make `migrate plan`
	// useless as a gate.
	results, err = r.PlanMigrations(ctx, withIndex)
	if err != nil {
		t.Fatalf("plan after rebuild: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 migrations once rebuilt, got %d: %q", len(results), results[0].DDL)
	}
}

// TestMigrationRunner_DriftedIndexIsRepairedWithoutManifestChange covers the
// path the applied checksum cannot see: the manifest is unchanged, but the
// index in storage is not the one the manifest describes. That is the state the
// kafe database is in (TODO 3.9) — the checksum fingerprints the manifest, so
// before this check `migrate plan` answered "nothing to do" while the database
// kept the old index.
func TestMigrationRunner_DriftedIndexIsRepairedWithoutManifestChange(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_index_drift.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	decl := []EntityMigration{{
		Metadata: spec.Metadata{Name: "shift", Module: "cafe-order"},
		EntitySpec: spec.EntitySpec{
			Version: "v1",
			Plural:  "shifts",
			Fields: []spec.Field{
				{Name: "branch_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.branch"}},
				{Name: "cashier_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.employee"}},
			},
			Indexes: []spec.IndexDecl{{Fields: []string{"branch_id", "cashier_id"}, Unique: true}},
		},
	}}

	// Converged: apply the manifest, so its checksum is recorded.
	if _, err := r.ApplyMigrations(ctx, decl); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if results, err := r.PlanMigrations(ctx, decl); err != nil || len(results) != 0 {
		t.Fatalf("expected a converged plan, got %d (err %v)", len(results), err)
	}

	// Storage drifts behind the manifest's back: same index name, narrower
	// definition (what every database created before 3.6 looks like).
	if _, err := d.ExecContext(ctx, "DROP INDEX idx_cafe_order_shifts_branch_id_cashier_id"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := d.ExecContext(ctx,
		"CREATE UNIQUE INDEX idx_cafe_order_shifts_branch_id_cashier_id ON cafe_order_shifts (_branch_id)"); err != nil {
		t.Fatalf("seed drifted index: %v", err)
	}

	// The manifest has not changed — only storage did. The plan must still
	// notice, or the database never returns to the declared shape.
	results, err := r.PlanMigrations(ctx, decl)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected a repair plan for the drifted index, got none")
	}
	if !strings.Contains(results[0].DDL, "DROP INDEX IF EXISTS idx_cafe_order_shifts_branch_id_cashier_id") {
		t.Errorf("repair plan must drop the drifted index\ngot: %s", results[0].DDL)
	}

	if _, err := r.ApplyMigrations(ctx, decl); err != nil {
		t.Fatalf("apply repair: %v", err)
	}
	if results, err := r.PlanMigrations(ctx, decl); err != nil || len(results) != 0 {
		t.Errorf("expected convergence after the repair, got %d (err %v)", len(results), err)
	}
}

// TestMigrationRunner_DriftedIndexIsRepairedOnSnapshotDiffPath reproduces the
// exact state the kafe database was stuck in (kafe TODO 3.9): a recorded
// snapshot that matches the manifest — so the rated diff (`DiffShapes`) finds
// nothing to change — while storage holds an older index.
//
// The DDL used to be built *only* from the rated changes, so this path returned
// a plan with no statements and `formspec migrate plan` answered "No pending
// migrations." forever. The snapshot records what was intended, not what the
// database has; the declared indexes must be checked against storage on this
// path too.
func TestMigrationRunner_DriftedIndexIsRepairedOnSnapshotDiffPath(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_index_snapshot.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	decl := []EntityMigration{{
		Metadata: spec.Metadata{Name: "shift", Module: "cafe-order"},
		EntitySpec: spec.EntitySpec{
			Version: "v1",
			Plural:  "shifts",
			Fields: []spec.Field{
				{Name: "branch_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.branch"}},
				{Name: "cashier_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.employee"}},
			},
			Indexes: []spec.IndexDecl{{Fields: []string{"branch_id", "cashier_id"}, Unique: true}},
		},
	}}

	if _, err := r.ApplyMigrations(ctx, decl); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Force the snapshot-diff path: the recorded checksum no longer matches the
	// manifest (the newest record wins), while the snapshot still describes the
	// manifest's shape — exactly the kafe database's situation.
	if err := r.RecordMigration(ctx, 9999, "entity:cafe-order/shift", "poisoned-checksum"); err != nil {
		t.Fatalf("record poisoned migration: %v", err)
	}

	// Storage drifts: same name, narrower definition.
	if _, err := d.ExecContext(ctx, "DROP INDEX idx_cafe_order_shifts_branch_id_cashier_id"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := d.ExecContext(ctx,
		"CREATE UNIQUE INDEX idx_cafe_order_shifts_branch_id_cashier_id ON cafe_order_shifts (_branch_id)"); err != nil {
		t.Fatalf("seed drifted index: %v", err)
	}

	results, err := r.PlanMigrations(ctx, decl)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected a repair plan on the snapshot-diff path, got none")
	}
	if !strings.Contains(results[0].DDL, "DROP INDEX IF EXISTS idx_cafe_order_shifts_branch_id_cashier_id") {
		t.Errorf("repair plan must drop the drifted index\ngot: %s", results[0].DDL)
	}

	if _, err := r.ApplyMigrations(ctx, decl); err != nil {
		t.Fatalf("apply repair: %v", err)
	}
	if results, err := r.PlanMigrations(ctx, decl); err != nil || len(results) != 0 {
		t.Errorf("expected convergence after the repair, got %d (err %v)", len(results), err)
	}
}

// TestMigrationRunner_UniqueIndexRejectsDuplicates pins GAP-32 (TODO 4.4): the
// canonical answer to "one row per key" is a database unique index, not a script
// guard. This proves the constraint actually rejects duplicates at runtime —
// including the partial form (only `open` shifts are constrained) — so a guard
// script is a second layer for a friendly message, never the sole enforcer.
func TestMigrationRunner_UniqueIndexRejectsDuplicates(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_unique.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	migrations := []EntityMigration{{
		Metadata: spec.Metadata{Name: "shift", Module: "cafe-order"},
		EntitySpec: spec.EntitySpec{
			Version: "v1",
			Plural:  "shifts",
			Fields: []spec.Field{
				{Name: "branch_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.branch"}},
				{Name: "cashier_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.employee"}},
				{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"open", "closed"}},
			},
			Indexes: []spec.IndexDecl{{
				Fields: []string{"branch_id", "cashier_id"},
				Unique: true,
				Where:  "status = 'open'",
			}},
		},
	}}
	if _, err := r.ApplyMigrations(ctx, migrations); err != nil {
		t.Fatalf("apply: %v", err)
	}

	insert := func(id, branch, cashier, status string) error {
		_, err := d.ExecContext(ctx,
			`INSERT INTO cafe_order_shifts (id, tenant_id, version, doc_status, data) VALUES (?, 'kafe', 1, '', ?)`,
			id, `{"branch_id":"`+branch+`","cashier_id":"`+cashier+`","status":"`+status+`"}`)
		return err
	}

	if err := insert("s1", "B1", "C1", "open"); err != nil {
		t.Fatalf("first open shift: %v", err)
	}
	if err := insert("s2", "B1", "C1", "open"); err == nil {
		t.Error("second open shift for same (branch, cashier): expected UNIQUE violation, got none")
	}
	// Partial index: closed shifts are not constrained.
	if err := insert("s3", "B1", "C1", "closed"); err != nil {
		t.Errorf("closed shift should be allowed alongside an open one: %v", err)
	}
	if err := insert("s4", "B1", "C1", "closed"); err != nil {
		t.Errorf("second closed shift should be allowed (partial index): %v", err)
	}
	// Different branch is a different key.
	if err := insert("s5", "B2", "C1", "open"); err != nil {
		t.Errorf("open shift in another branch should be allowed: %v", err)
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

// TestMigrationRunner_AlteredDerivedColumnEnforcesUnique pins kafe TODO 3.11: a
// derived column materialized by ALTER TABLE (not at table creation) must still
// be populated, or any unique index built over it enforces nothing.
//
// SQLite cannot add a STORED generated column with ALTER TABLE, so the column
// used to be added as a plain column that nothing ever wrote: it stayed NULL on
// every row, and because NULLs never collide in a unique index the second open
// shift for the same (branch, cashier) was accepted — a business rule that
// looked declared while enforcing nothing.
func TestMigrationRunner_AlteredDerivedColumnEnforcesUnique(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_alter_derived.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	// v1 declares the shift entity without cashier_id: `_cashier_id` therefore
	// does not exist when the table is created.
	v1 := []EntityMigration{{
		Metadata: spec.Metadata{Name: "shift", Module: "cafe-order"},
		EntitySpec: spec.EntitySpec{
			Version: "v1",
			Plural:  "shifts",
			Fields: []spec.Field{
				{Name: "branch_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.branch"}},
				{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"open", "closed"}},
			},
		},
	}}
	if _, err := r.ApplyMigrations(ctx, v1); err != nil {
		t.Fatalf("apply v1: %v", err)
	}

	// v2 adds the field and the partial unique rule on an existing table, so
	// `_cashier_id` (and `_branch_id`, `_status`) are materialized by ALTER.
	v2 := []EntityMigration{{
		Metadata: spec.Metadata{Name: "shift", Module: "cafe-order"},
		EntitySpec: spec.EntitySpec{
			Version: "v1",
			Plural:  "shifts",
			Fields: []spec.Field{
				{Name: "branch_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.branch"}},
				{Name: "cashier_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.employee"}},
				{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"open", "closed"}},
			},
			Indexes: []spec.IndexDecl{{
				Fields: []string{"branch_id", "cashier_id"},
				Unique: true,
				Where:  "status = 'open'",
			}},
		},
	}}
	if _, err := r.ApplyMigrations(ctx, v2); err != nil {
		t.Fatalf("apply v2: %v", err)
	}

	insert := func(id, branch, cashier, status string) error {
		_, err := d.ExecContext(ctx,
			`INSERT INTO cafe_order_shifts (id, tenant_id, version, doc_status, data) VALUES (?, 'kafe', 1, '', ?)`,
			id, `{"branch_id":"`+branch+`","cashier_id":"`+cashier+`","status":"`+status+`"}`)
		return err
	}

	if err := insert("s1", "B1", "C1", "open"); err != nil {
		t.Fatalf("first open shift: %v", err)
	}
	if err := insert("s2", "B1", "C1", "open"); err == nil {
		t.Error("second open shift for same (branch, cashier): expected UNIQUE violation, got none")
	}
	if err := insert("s3", "B2", "C1", "open"); err != nil {
		t.Errorf("open shift in another branch should be allowed: %v", err)
	}
	if err := insert("s4", "B1", "C1", "closed"); err != nil {
		t.Errorf("closed shift should be allowed (partial index): %v", err)
	}
}

// TestMigrationRunner_StaleDerivedColumnIsRepaired covers the other half of kafe
// TODO 3.11: a database that already holds a derived column as a *plain* column
// (the shape the old ALTER path produced) must be repaired, and the repair has to
// happen even though the manifest never changed.
//
// This is the case the snapshot diff cannot see: the checksum fingerprints the
// manifest, the manifest is identical, the index definition is identical — only
// the storage disagrees. Without looking at the database the plan answers
// "nothing to do" and the check keeps enforcing nothing.
func TestMigrationRunner_StaleDerivedColumnIsRepaired(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_stale_column.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	meta := spec.Metadata{Name: "shift", Module: "cafe-order"}
	entity := spec.EntitySpec{
		Version: "v1",
		Plural:  "shifts",
		Fields: []spec.Field{
			{Name: "branch_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.branch"}},
			{Name: "cashier_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "cafe-master.employee"}},
			{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"open", "closed"}},
		},
		Indexes: []spec.IndexDecl{{
			Fields: []string{"branch_id", "cashier_id"},
			Unique: true,
			Where:  "status = 'open'",
		}},
	}
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: entity}}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	ti, err := GenerateEntityDDL(meta, &entity, DriverSQLite)
	if err != nil {
		t.Fatalf("GenerateEntityDDL: %v", err)
	}
	indexSQL := ""
	for _, stmt := range ti.CreateIndexSQL {
		if strings.Contains(stmt, "_cashier_id") {
			indexSQL = stmt
		}
	}
	if indexSQL == "" {
		t.Fatal("test setup: no index over _cashier_id was generated")
	}
	indexName := indexNameOf(indexSQL)

	// Rewind the database to the shape the old ALTER path produced: `_cashier_id`
	// as a plain column (nothing writes it) with the same index on top.
	for _, stmt := range []string{
		"DROP INDEX IF EXISTS " + indexName,
		"ALTER TABLE cafe_order_shifts DROP COLUMN _cashier_id",
		"ALTER TABLE cafe_order_shifts ADD COLUMN _cashier_id text",
		indexSQL,
	} {
		if _, err := d.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("simulate stale column (%s): %v", stmt, err)
		}
	}

	insert := func(id, branch, cashier, status string) error {
		_, err := d.ExecContext(ctx,
			`INSERT INTO cafe_order_shifts (id, tenant_id, version, doc_status, data) VALUES (?, 'kafe', 1, '', ?)`,
			id, `{"branch_id":"`+branch+`","cashier_id":"`+cashier+`","status":"`+status+`"}`)
		return err
	}

	// The stale column holds NULL, so the unique index waves duplicates through —
	// that is the bug, and the precondition for calling the storage drifted.
	if err := insert("s1", "B1", "C1", "open"); err != nil {
		t.Fatalf("first open shift: %v", err)
	}
	if err := insert("s2", "B1", "C1", "open"); err != nil {
		t.Fatalf("precondition: a plain NULL column must accept the duplicate, got %v", err)
	}
	// Clear the duplicate so the repair has a satisfiable starting point: the
	// rebuilt column is correct immediately, which would make CREATE UNIQUE INDEX
	// fail on rows the old shape wrongly allowed.
	if _, err := d.ExecContext(ctx, "DELETE FROM cafe_order_shifts WHERE id = 's2'"); err != nil {
		t.Fatalf("clean up duplicate: %v", err)
	}

	// The plan must notice the storage drift even though nothing in the manifest
	// changed.
	results, err := r.PlanMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: entity}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected a repair plan for the stale derived column, got none")
	}
	if !strings.Contains(results[0].DDL, "DROP COLUMN _cashier_id") ||
		!strings.Contains(results[0].DDL, "GENERATED ALWAYS") {
		t.Errorf("repair plan must rebuild _cashier_id as a generated column\ngot: %s", results[0].DDL)
	}

	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: entity}}); err != nil {
		t.Fatalf("apply repair: %v", err)
	}

	// Repaired: the existing row now carries a value, so the duplicate collides.
	if err := insert("s3", "B1", "C1", "open"); err == nil {
		t.Error("after the repair the duplicate open shift must be rejected")
	}
	if err := insert("s4", "B2", "C1", "open"); err != nil {
		t.Errorf("open shift in another branch should be allowed: %v", err)
	}

	if results, err := r.PlanMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: entity}}); err != nil || len(results) != 0 {
		t.Errorf("expected convergence after the repair, got %d (err %v)", len(results), err)
	}
}

// TestPlanSpecSet_IgnoresFrameworkModules pins kafe TODO 3.10's other half: the
// server registers framework-owned entities (formspec.core.*) at runtime, so a
// database it migrated has their tables and snapshots. A caller that loads only
// the user's spec tree (the CLI) must not report them as a removed entity —
// that refusal is impossible to satisfy, because no manifest ever declared them.
func TestPlanSpecSet_IgnoresFrameworkModules(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "migrate_core_module.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer func() { _ = d.Close() }()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("ensure system tables: %v", err)
	}

	user := []EntityMigration{{
		Metadata: spec.Metadata{Name: "item", Module: "alpha"},
		EntitySpec: spec.EntitySpec{
			Version: "v1",
			Fields:  []spec.Field{{Name: "code", Type: spec.FieldString}},
		},
	}}
	if _, err := r.ApplyMigrations(ctx, user); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// The footprint of a server-migrated database: a formspec.core table plus
	// its snapshot, invisible to the user's spec tree.
	if _, err := d.ExecContext(ctx, "CREATE TABLE formspec_core_users (id text)"); err != nil {
		t.Fatalf("seed core table: %v", err)
	}
	if err := r.SaveSnapshot(ctx, d, "formspec.core", "user", "checksum", &EntitySnapshot{
		Table: "formspec_core_users",
	}); err != nil {
		t.Fatalf("seed core snapshot: %v", err)
	}

	// Without the exclusion the plan refuses — that is the bug this test guards.
	r0 := NewMigrationRunner(d, DriverSQLite)
	plans0, err := r0.PlanSpecSet(ctx, user)
	if err != nil {
		t.Fatalf("plan without ignore: %v", err)
	}
	refused := false
	for _, p := range plans0 {
		for _, c := range p.Changes {
			if c.Kind == ChangeTableRemoved && c.Name == "formspec_core_users" {
				refused = true
			}
		}
	}
	if !refused {
		t.Fatal("precondition: the core table must be refused when the module is not excluded")
	}

	// With the exclusion the core module is simply not the caller's business.
	r.IgnoreModules("formspec.core")
	plans, err := r.PlanSpecSet(ctx, user)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, p := range plans {
		for _, c := range p.Changes {
			if c.Kind == ChangeTableRemoved && c.Name == "formspec_core_users" {
				t.Errorf("framework-owned table must not be reported as removed: %v", c)
			}
		}
	}
}

// TestIndexShapeOf_PostgreSQLIndexDef pins the shape comparison against the
// indexdef PostgreSQL actually reports back. The first real PG run compared
// the generated predicate `_status = 'open'` against PG's rewritten
// `((_status)::text = 'open'::text)` and reported every enum-predicate index
// as drifted on every run — a plan that never empties cannot gate anything
// (master todo 15.8).
func TestIndexShapeOf_PostgreSQLIndexDef(t *testing.T) {
	generated := `CREATE UNIQUE INDEX idx_cafe_order_shifts_branch_id_cashier_id ON cafe_order_shifts (_branch_id, _cashier_id) WHERE _status = 'open';`
	// What pg_get_indexdef returns for that index on PG 17.
	reported := `CREATE UNIQUE INDEX idx_cafe_order_shifts_branch_id_cashier_id ON operational.cafe_order_shifts USING btree (_branch_id, _cashier_id) WHERE ((_status)::text = 'open'::text)`

	if !indexShapesEqual(indexShapeOf(generated), indexShapeOf(reported)) {
		t.Errorf("the same index must compare equal across the rewrite:\n generated: %s\n reported:  %s", generated, reported)
	}
}
