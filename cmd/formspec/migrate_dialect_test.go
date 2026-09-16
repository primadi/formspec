package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// writeMigration drops a kind: Migration manifest into a temp spec tree.
func writeMigration(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "apiVersion: formspec.dev/v1\nkind: Migration\nmetadata: { name: " + name + " }\nspec:\n" + body
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Gap #35: the same JSONB read is spelled differently per driver, so a single
// `ddl` string is correct for dev and wrong for production — a failure that only
// appears at deploy time. `ddl_by` carries both, and the loader picks by driver.
func TestLoadCustomMigrations_PicksDialect(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "dedupe-index", `
  ddl_by:
    sqlite: "CREATE INDEX idx_sqlite ON t (id)"
    postgres: "CREATE INDEX idx_pg ON t (id)"
`)

	for _, tc := range []struct {
		driver db.DriverType
		want   string
	}{
		{db.DriverSQLite, "idx_sqlite"},
		{db.DriverPostgres, "idx_pg"},
	} {
		got := loadCustomMigrations(dir, tc.driver)
		if len(got) != 1 {
			t.Fatalf("%s: want 1 migration, got %d", tc.driver, len(got))
		}
		if !strings.Contains(got[0].DDL, tc.want) {
			t.Errorf("%s: DDL = %q, want it to mention %s", tc.driver, got[0].DDL, tc.want)
		}
	}

	// A portable `ddl` applies to every driver.
	portable := t.TempDir()
	writeMigration(t, portable, "portable", "  ddl: \"CREATE INDEX idx_any ON t (id)\"\n")
	for _, driver := range []db.DriverType{db.DriverSQLite, db.DriverPostgres} {
		if got := loadCustomMigrations(portable, driver); len(got) != 1 {
			t.Errorf("%s: a portable ddl must apply everywhere, got %d migration(s)", driver, len(got))
		}
	}
}

// Validation refuses the shapes that would silently skip DDL (gap #35) or land
// an unlabelled data change (gap #36).
func TestValidateMigrationSpec(t *testing.T) {
	bad := []struct {
		name    string
		ms      *spec.MigrationSpec
		wantSub string
	}{
		{"no ddl at all", &spec.MigrationSpec{}, "declares no DDL"},
		{"both forms", &spec.MigrationSpec{DDL: "CREATE INDEX i ON t(c)", DDLByDialect: map[string]string{"sqlite": "x"}}, "declares both"},
		{"unknown dialect", &spec.MigrationSpec{DDLByDialect: map[string]string{"mysql": "x"}}, "unknown dialect"},
		{"empty variant", &spec.MigrationSpec{DDLByDialect: map[string]string{"sqlite": "  "}}, "is empty"},
		{"dml without reason", &spec.MigrationSpec{DDL: "CREATE INDEX i ON t(c)", DML: []string{"DELETE FROM t"}}, "without `reason`"},
		{"ddl smuggled into dml", &spec.MigrationSpec{DDL: "CREATE INDEX i ON t(c)", DML: []string{"DROP TABLE t"}, Reason: "cleanup"}, "not a data statement"},
	}
	for _, c := range bad {
		err := spec.ValidateMigrationSpec(c.ms)
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("%s: error %q does not contain %q", c.name, err.Error(), c.wantSub)
		}
	}

	ok := &spec.MigrationSpec{
		DDLByDialect: map[string]string{"sqlite": "CREATE INDEX i ON t(c)"},
		DML:          []string{"DELETE FROM t WHERE rowid NOT IN (SELECT MIN(rowid) FROM t GROUP BY c)"},
		Reason:       "hapus duplikat sebelum unique index",
	}
	if err := spec.ValidateMigrationSpec(ok); err != nil {
		t.Errorf("declared data repair: expected no error, got %v", err)
	}
}

// Gap #36, the scenario the ledger describes end to end: the constraint could not
// be added while duplicates existed, and duplicates *did* appear because the
// constraint was missing. Refusing DML meant the repair happened by hand outside
// the spec; declaring it makes the migration able to finish the job, in order.
func TestApplyCustomMigrations_DataRepairRunsBeforeDDL(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "menu-price-unique", `
  reason: "dua harga untuk menu yang sama muncul selagi unique index belum ada"
  dml:
    - "DELETE FROM menu_prices WHERE rowid NOT IN (SELECT MIN(rowid) FROM menu_prices GROUP BY menu_id)"
  ddl: "CREATE UNIQUE INDEX idx_menu_price ON menu_prices (menu_id)"
`)

	migrations := loadCustomMigrations(dir, db.DriverSQLite)
	if len(migrations) != 1 || len(migrations[0].DML) != 1 {
		t.Fatalf("expected 1 migration carrying 1 repair, got %#v", migrations)
	}

	ctx := context.Background()
	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if _, err := database.ExecContext(ctx,
		"CREATE TABLE menu_prices (rowid INTEGER PRIMARY KEY, menu_id text, amount text)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	// Duplicates of the kind that appear when nothing enforces uniqueness.
	for _, stmt := range []string{
		"INSERT INTO menu_prices (menu_id, amount) VALUES ('kopi', '25000')",
		"INSERT INTO menu_prices (menu_id, amount) VALUES ('kopi', '27000')",
		"INSERT INTO menu_prices (menu_id, amount) VALUES ('teh', '15000')",
	} {
		if _, err := database.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Without the repair the constraint cannot be added at all — this is the
	// failure that made the ledger ask for a safe path. (Nothing to clean up:
	// the statement fails, so no index exists.)
	if _, err := database.ExecContext(ctx, "CREATE UNIQUE INDEX probe ON menu_prices (menu_id)"); err == nil {
		t.Fatal("expected CREATE UNIQUE INDEX to fail while duplicates exist")
	}

	applied, err := applyCustomMigrations(ctx, database, migrations)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied != 1 {
		t.Fatalf("want 1 applied, got %d", applied)
	}

	var rows int
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM menu_prices").Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 2 {
		t.Fatalf("after repair: want 2 rows (one per menu), got %d", rows)
	}
	// And the constraint now holds.
	if _, err := database.ExecContext(ctx, "INSERT INTO menu_prices (menu_id, amount) VALUES ('kopi', '999')"); err == nil {
		t.Fatal("unique index must reject a duplicate after the repair")
	}
}
