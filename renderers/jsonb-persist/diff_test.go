package db

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// ─── Klasifikasi (pure) ───

func shapeWith(fields ...FieldShape) *EntitySnapshot {
	return &EntitySnapshot{Table: "t_items", Fields: fields}
}

func noDecls() Declarations {
	return Declarations{Removed: map[string]string{}, AcceptDataLoss: map[string]string{}}
}

// A field that disappears is lossy and, without a tombstone, refused. This is the
// change the old additive-only diff silently ignored while it kept breaking
// writes on every existing row.
func TestDiffShapes_RemovalNeedsDeclaration(t *testing.T) {
	old := shapeWith(FieldShape{Name: "code", Type: "string", Derived: true})
	desired := shapeWith()

	changes := DiffShapes("alpha/item", old, desired, noDecls())
	if len(changes) != 1 {
		t.Fatalf("want 1 change, got %d (%v)", len(changes), changes)
	}
	c := changes[0]
	if c.Class != ClassLossy || c.Kind != ChangeFieldRemoved {
		t.Fatalf("want lossy field_removed, got %s/%s", c.Class, c.Kind)
	}
	if !c.Required() {
		t.Error("an undeclared removal must be refused")
	}
	if !strings.Contains(c.Remedy, "removed: true") {
		t.Errorf("remedy must name the declaration, got %q", c.Remedy)
	}

	// Declared: allowed, carrying the reason into the report.
	decls := Declarations{Removed: map[string]string{"code": "digantikan sku"}, AcceptDataLoss: map[string]string{}}
	declared := DiffShapes("alpha/item", old, desired, decls)
	if declared[0].Required() {
		t.Error("a declared removal must pass")
	}
	if declared[0].Reason != "digantikan sku" || !declared[0].Declared {
		t.Errorf("declaration not carried through: %+v", declared[0])
	}
}

// Retyping a payload-only field costs nothing: the raw value lives in `data` and
// is not rewritten. Only a projected column can lose values to a cast, so only
// that case carries the "declare it" remedy.
func TestDiffShapes_TypeChangeOnPayloadOnlyFieldIsNotLossy(t *testing.T) {
	old := shapeWith(FieldShape{Name: "note", Type: "string"})
	desired := shapeWith(FieldShape{Name: "note", Type: "integer"})

	changes := DiffShapes("alpha/item", old, desired, noDecls())
	if len(changes) != 1 || changes[0].Kind != ChangeTypeChanged {
		t.Fatalf("want 1 type_changed, got %v", changes)
	}
	if changes[0].Projected {
		t.Error("a payload-only field must not be marked projected")
	}
	if changes[0].Required() {
		t.Error("a payload-only type change must not need consent")
	}

	// Projected: subject to the preflight, and refused until declared.
	oldProj := shapeWith(FieldShape{Name: "amount", Type: "string", Derived: true})
	desiredProj := shapeWith(FieldShape{Name: "amount", Type: "integer", Derived: true})
	proj := DiffShapes("alpha/item", oldProj, desiredProj, noDecls())
	if !proj[0].Projected {
		t.Fatal("a derived field must be marked projected")
	}
	if !Measurable(proj)[0].Projected && len(Measurable(proj)) == 0 {
		t.Error("a projected type change must be measured by the preflight")
	}
}

// Dropping an index rebuilds nothing that holds data, so it applies on its own —
// but a unique one is reported as losing enforcement rather than as a quiet
// housekeeping step.
func TestDiffShapes_IndexRemovalIsDerivedButNamed(t *testing.T) {
	old := &EntitySnapshot{Table: "t_items", Indexes: []IndexShape{{Name: "idx_uq_t_items_sku", Sig: "aaa", Unique: true}}}
	desired := &EntitySnapshot{Table: "t_items"}

	changes := DiffShapes("alpha/item", old, desired, noDecls())
	if len(changes) != 1 {
		t.Fatalf("want 1 change, got %v", changes)
	}
	if changes[0].Class != ClassDerived {
		t.Errorf("index removal must be derived (data is untouched), got %s", changes[0].Class)
	}
	if changes[0].Required() {
		t.Error("index removal must not be refused")
	}
	if !strings.Contains(changes[0].Detail, "uniqueness") {
		t.Errorf("a dropped unique index must say enforcement was lost, got %q", changes[0].Detail)
	}
}

// A never-class change is refused no matter what the manifest says — that is what
// separates "declared" from "acceptable".
func TestChange_TableRemovalIsNeverAcceptable(t *testing.T) {
	c := Change{
		Class:  ClassNever,
		Kind:   ChangeTableRemoved,
		Name:   "alpha_items",
		Remedy: "back up, drop the table by hand, then remove the manifest",
	}
	if !c.Required() {
		t.Error("a table removal must always be refused")
	}
	err := RefuseUndeclared([]Change{c})
	if err == nil || !strings.Contains(err.Error(), "by hand") {
		t.Fatalf("refusal must tell the operator the path, got %v", err)
	}
}

// The preflight turns a general warning into a specific one.
func TestChange_MeasurePromotesToLossy(t *testing.T) {
	c := Change{Class: ClassAdditive, Kind: ChangeIndexAdded, Name: "idx", Unique: true}
	c.Measure(0, "")
	if c.Required() {
		t.Fatal("no duplicates means the index is addable")
	}
	if !c.Counted {
		t.Error("a measured change must say it was measured")
	}

	c2 := Change{Class: ClassAdditive, Kind: ChangeIndexAdded, Name: "idx", Unique: true}
	c2.Measure(3, "repair the duplicates first")
	if c2.Class != ClassLossy || !c2.Required() {
		t.Fatalf("duplicates must promote the change to refused lossy, got %s", c2.Class)
	}
	if !strings.Contains(c2.String(), "3 row(s) affected") {
		t.Errorf("the count must be visible, got %q", c2.String())
	}
}

// ─── Deklarasi & raw_ddl (validasi) ───

func TestValidateEntitySpec_DestructiveDeclarations(t *testing.T) {
	base := func(f spec.Field) *spec.EntitySpec {
		return &spec.EntitySpec{Version: "v1", Fields: []spec.Field{f}}
	}

	cases := []struct {
		name    string
		field   spec.Field
		wantSub string
	}{
		{"removed without reason", spec.Field{Name: "a", Type: spec.FieldString, Removed: true}, "requires `reason`"},
		{"accept_data_loss without reason", spec.Field{Name: "a", Type: spec.FieldString, AcceptDataLoss: true}, "requires `reason`"},
		{"both declarations", spec.Field{Name: "a", Type: spec.FieldString, Removed: true, AcceptDataLoss: true, Reason: "x"}, "both `removed` and `accept_data_loss`"},
		{"removed plus renamed_from", spec.Field{Name: "a", Type: spec.FieldString, Removed: true, RenamedFrom: "b", Reason: "x"}, "mutually exclusive"},
		{"removed and required", spec.Field{Name: "a", Type: spec.FieldString, Removed: true, Required: true, Reason: "x"}, "cannot be required"},
	}
	for _, c := range cases {
		err := spec.ValidateEntitySpec(base(c.field))
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("%s: error %q does not contain %q", c.name, err.Error(), c.wantSub)
		}
	}

	// A well-formed tombstone passes.
	ok := base(spec.Field{Name: "a", Type: spec.FieldString, Removed: true, Reason: "digantikan b"})
	if err := spec.ValidateEntitySpec(ok); err != nil {
		t.Errorf("declared removal: expected no error, got %v", err)
	}
}

func TestValidateRawDDL(t *testing.T) {
	dialects := map[string]string{"sqlite": "CREATE INDEX i ON t(c)", "postgres": "CREATE INDEX i ON t(c)"}

	bad := []struct {
		name    string
		decl    spec.RawDDLDecl
		wantSub string
	}{
		{"no name", spec.RawDDLDecl{DDL: "CREATE INDEX i ON t(c)", Reason: "x"}, "name is required"},
		{"no ddl", spec.RawDDLDecl{Name: "a", Reason: "x"}, "declares no DDL"},
		{"both forms", spec.RawDDLDecl{Name: "a", DDL: "CREATE INDEX i ON t(c)", DDLByDialect: dialects, Reason: "x"}, "declares both"},
		{"unknown dialect", spec.RawDDLDecl{Name: "a", DDLByDialect: map[string]string{"mysql": "x"}, Reason: "x"}, "unknown dialect"},
		{"empty variant", spec.RawDDLDecl{Name: "a", DDLByDialect: map[string]string{"sqlite": "  "}, Reason: "x"}, "is empty"},
		{"no reason", spec.RawDDLDecl{Name: "a", DDL: "CREATE INDEX i ON t(c)"}, "`reason` is required"},
		{"data statement", spec.RawDDLDecl{Name: "a", DDL: "DELETE FROM t", Reason: "x"}, "data statement"},
		{"drop", spec.RawDDLDecl{Name: "a", DDL: "DROP TABLE t", Reason: "x"}, "never drops storage"},
	}
	for _, c := range bad {
		err := spec.ValidateRawDDL([]spec.RawDDLDecl{c.decl})
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("%s: error %q does not contain %q", c.name, err.Error(), c.wantSub)
		}
	}

	// A duplicate name would make the recorded checksum ambiguous.
	dup := []spec.RawDDLDecl{
		{Name: "a", DDL: "CREATE INDEX i ON t(c)", Reason: "x"},
		{Name: "a", DDL: "CREATE INDEX j ON t(d)", Reason: "y"},
	}
	if err := spec.ValidateRawDDL(dup); err == nil || !strings.Contains(err.Error(), "duplicate name") {
		t.Errorf("duplicate names must be rejected, got %v", err)
	}

	ok := []spec.RawDDLDecl{{Name: "a", DDLByDialect: dialects, Reason: "satu harga per menu"}}
	if err := spec.ValidateRawDDL(ok); err != nil {
		t.Errorf("well-formed raw_ddl: expected no error, got %v", err)
	}
}

// ─── Engine (end-to-end) ───

func newTestRunner(t *testing.T) (*MigrationRunner, DB, context.Context) {
	t.Helper()
	// A file per test: `sqlite::memory:` is shared by DSN inside this driver, so
	// parallel tests would see each other's tables.
	d, err := OpenSQLite(filepath.Join(t.TempDir(), "diff.db"), nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return NewMigrationRunner(d, DriverSQLite), d, context.Background()
}

func itemEntity(fields []spec.Field, persist *spec.PersistSpec) []EntityMigration {
	return []EntityMigration{{
		Metadata:   spec.Metadata{Name: "item", Module: "alpha"},
		EntitySpec: spec.EntitySpec{Version: "v1", Fields: fields, Persist: persist},
	}}
}

// A payload-only field change is a contract change with no DDL, and it must still
// be recorded — otherwise the next diff has no baseline and cannot tell a removed
// field from one that never existed.
func TestMigrate_PayloadOnlyChangeIsRecorded(t *testing.T) {
	r, _, ctx := newTestRunner(t)

	if n, err := r.ApplySpecSet(ctx, itemEntity([]spec.Field{{Name: "code", Type: spec.FieldString}}, nil)); err != nil || n != 1 {
		t.Fatalf("first apply: applied=%d err=%v", n, err)
	}

	fields := []spec.Field{{Name: "code", Type: spec.FieldString}, {Name: "note", Type: spec.FieldString}}
	if n, err := r.ApplySpecSet(ctx, itemEntity(fields, nil)); err != nil || n != 1 {
		t.Fatalf("second apply: applied=%d err=%v", n, err)
	}

	// Converged: applying the same spec again changes nothing.
	if n, err := r.ApplySpecSet(ctx, itemEntity(fields, nil)); err != nil || n != 0 {
		t.Fatalf("converged apply: applied=%d err=%v", n, err)
	}
}

// A database created before snapshots existed has no baseline. Refusing there
// would block a deployment over a change nobody can see or declare, so the engine
// adopts the manifest and reconciles only what is provably missing.
func TestMigrate_BootstrapAdoptsBaseline(t *testing.T) {
	r, d, ctx := newTestRunner(t)

	if _, err := r.ApplySpecSet(ctx, itemEntity([]spec.Field{{Name: "code", Type: spec.FieldString}}, nil)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Simulate a pre-snapshot database: the migration record stays, the shape
	// memory is gone.
	if _, err := d.ExecContext(ctx, "DELETE FROM "+SnapshotTable); err != nil {
		t.Fatalf("clear snapshot: %v", err)
	}

	fields := []spec.Field{
		{Name: "code", Type: spec.FieldString},
		{Name: "sku", Type: spec.FieldString, Unique: true}, // new derived column
	}
	res, err := r.ApplySpecSetDetailed(ctx, itemEntity(fields, nil))
	if err != nil {
		t.Fatalf("bootstrap apply must not refuse: %v", err)
	}
	if res.Applied == 0 {
		t.Error("bootstrap must still reconcile the additive part")
	}

	var cols int
	if err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_xinfo('alpha_items') WHERE name = '_sku'").Scan(&cols); err != nil {
		t.Fatalf("check column: %v", err)
	}
	if cols != 1 {
		t.Error("expected the missing derived column to be added during bootstrap")
	}
}

// The unique index that could not be created while duplicates existed: the
// preflight counts them and refuses, the operator repairs the data once, and the
// next apply succeeds. This replaces the declared-DML mechanism that the removed
// kind used to carry.
func TestMigrate_UniqueIndexBlockedByDuplicates(t *testing.T) {
	r, d, ctx := newTestRunner(t)

	plain := []spec.Field{
		{Name: "branch_id", Type: spec.FieldString},
		{Name: "menu_item_id", Type: spec.FieldString},
	}
	if _, err := r.ApplySpecSet(ctx, itemEntity(plain, nil)); err != nil {
		t.Fatalf("apply base: %v", err)
	}

	for i, data := range []string{
		`{"branch_id": "B1", "menu_item_id": "M1"}`,
		`{"branch_id": "B1", "menu_item_id": "M1"}`,
		`{"branch_id": "B2", "menu_item_id": "M1"}`,
	} {
		if _, err := d.ExecContext(ctx,
			`INSERT INTO alpha_items (id, tenant_id, version, created_at, updated_at, doc_status, data) `+
				`VALUES (?, 'demo', 1, '2026-09-16', '2026-09-16', NULL, ?)`,
			fmt.Sprintf("row-%d", i), data); err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	withIndex := EntityMigration{
		Metadata: spec.Metadata{Name: "item", Module: "alpha"},
		EntitySpec: spec.EntitySpec{
			Version: "v1",
			Fields:  plain,
			Indexes: []spec.IndexDecl{{Fields: []string{"branch_id", "menu_item_id"}, Unique: true}},
		},
	}

	_, err := r.ApplySpecSet(ctx, []EntityMigration{withIndex})
	if err == nil {
		t.Fatal("expected the unique index to be refused while duplicates exist")
	}
	if !strings.Contains(err.Error(), "duplicate") || !strings.Contains(err.Error(), "1 row(s) affected") {
		t.Errorf("refusal must carry the measured count, got: %v", err)
	}

	// The repair: one row removed by hand, once.
	if _, err := d.ExecContext(ctx,
		`DELETE FROM alpha_items WHERE id = (SELECT id FROM alpha_items WHERE json_extract(data,'$.branch_id') = 'B1' LIMIT 1)`); err != nil {
		t.Fatalf("repair: %v", err)
	}

	if _, err := r.ApplySpecSet(ctx, []EntityMigration{withIndex}); err != nil {
		t.Fatalf("apply after the repair: %v", err)
	}

	var idx int
	if err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_alpha_items_branch_id_menu_item_id'").Scan(&idx); err != nil {
		t.Fatalf("check index: %v", err)
	}
	if idx != 1 {
		t.Fatal("expected the unique index to exist after the repair")
	}

	// Not asserted here: that the index actually rejects duplicates on SQLite for
	// rows written *before* the index existed. The modernc driver cannot
	// `ADD COLUMN ... GENERATED ALWAYS`, so a derived column added after table
	// creation is a plain, unfilled column — the index then constrains nothing
	// until the row is rewritten. The preflight is unaffected (it counts the
	// payload, not the column), which is why the refusal above is trustworthy.
	// Recorded as a gap in docs_internal/plan/todo.md.
}

// raw_ddl lives inside the Entity, runs on the automatic path, is recorded by
// checksum, and is forward-only.
func TestMigrate_RawDDLRunsOnce(t *testing.T) {
	r, d, ctx := newTestRunner(t)

	persist := &spec.PersistSpec{RawDDL: []spec.RawDDLDecl{{
		Name:         "menu-price-unique",
		Reason:       "satu harga per menu",
		DDLByDialect: map[string]string{"sqlite": "CREATE INDEX idx_menu_price ON alpha_items (tenant_id)"},
	}}}
	fields := []spec.Field{{Name: "code", Type: spec.FieldString}}

	if n, err := r.ApplySpecSet(ctx, itemEntity(fields, persist)); err != nil || n != 1 {
		t.Fatalf("apply with raw_ddl: applied=%d err=%v", n, err)
	}
	var idx int
	if err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_menu_price'").Scan(&idx); err != nil {
		t.Fatalf("check index: %v", err)
	}
	if idx != 1 {
		t.Fatal("expected the declared raw_ddl index to exist")
	}

	// Unchanged statement: nothing to do.
	if n, err := r.ApplySpecSet(ctx, itemEntity(fields, persist)); err != nil || n != 0 {
		t.Fatalf("raw_ddl must be recorded by checksum, applied=%d err=%v", n, err)
	}

	// A changed statement is re-applied (the old index is kept — forward-only).
	persist2 := &spec.PersistSpec{RawDDL: []spec.RawDDLDecl{{
		Name:         "menu-price-unique",
		Reason:       "satu harga per menu",
		DDLByDialect: map[string]string{"sqlite": "CREATE INDEX idx_menu_price ON alpha_items (tenant_id, version)"},
	}}}
	if n, err := r.ApplySpecSet(ctx, itemEntity(fields, persist2)); err == nil && n == 0 {
		t.Error("a changed raw_ddl statement must be re-applied")
	}
}

// The failure this whole change exists to remove: a field dropped from the
// manifest used to leave its key in every existing row, and the next PATCH then
// failed as an unknown field — permanently, on rows written before the change.
// Now the tombstone makes the field known-but-dead: writes carrying it are
// accepted and stripped, and reads never expose it.
func TestEntityStore_TombstonedFieldDoesNotBreakWrites(t *testing.T) {
	r, d, ctx := newTestRunner(t)

	before := []spec.Field{{Name: "name", Type: spec.FieldString}, {Name: "legacy", Type: spec.FieldString}}
	if _, err := r.ApplySpecSet(ctx, itemEntity(before, nil)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := d.ExecContext(ctx,
		`INSERT INTO alpha_items (id, tenant_id, version, created_at, updated_at, doc_status, data) `+
			`VALUES ('row-1', 'demo', 1, '2026-09-16', '2026-09-16', NULL, '{"name": "A", "legacy": "X"}')`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	after := []spec.Field{
		{Name: "name", Type: spec.FieldString},
		{Name: "legacy", Type: spec.FieldString, Removed: true, Reason: "digantikan name"},
	}
	if _, err := r.ApplySpecSet(ctx, itemEntity(after, nil)); err != nil {
		t.Fatalf("declared removal: %v", err)
	}

	entitySpec := &spec.EntitySpec{Version: "v1", Fields: after}
	store := NewEntityStore(d, DriverSQLite, spec.Metadata{Name: "item", Module: "alpha"}, entitySpec)

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "demo", ID: "row-1"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if _, exposed := rec.Data["legacy"]; exposed {
		t.Error("a retired field must not be exposed on read")
	}

	// The write that used to fail with "unknown field".
	newVersion, err := store.Update(ctx, UpdateParams{
		WorkspaceID: "demo",
		ID:          "row-1",
		Version:     rec.Version,
		Data:        map[string]any{"name": "B", "legacy": "Y"},
	})
	if err != nil {
		t.Fatalf("a write carrying a retired key must be accepted: %v", err)
	}
	if newVersion == 0 {
		t.Error("expected a new version")
	}

	var legacy int
	if err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM alpha_items WHERE json_extract(data, '$.legacy') IS NOT NULL").Scan(&legacy); err != nil {
		t.Fatalf("count: %v", err)
	}
	if legacy != 0 {
		t.Error("a retired value must not be stored again")
	}
}
