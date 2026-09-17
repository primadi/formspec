package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// Gap #23: a `money` field is the object {amount, currency}, so a derived column
// built from the JSON text stored `{"amount":"9000",…}` and compared it as a
// string — making 9000 sort AFTER 10000. The derived column now extracts
// `.amount` into a numeric column, which is what the index on top of it, and the
// sort/filter/aggregate paths, all rely on.
func TestGenerateEntityDDL_MoneyDerivedColumnReadsAmount(t *testing.T) {
	meta := spec.Metadata{Name: "menu-item-price", Module: "cafe-master"}
	entity := &spec.EntitySpec{
		Version:        "v1",
		Plural:         "menu-item-prices",
		Characteristic: spec.CharMaster,
		Fields: []spec.Field{
			{Name: "price", Type: spec.FieldMoney, Index: true},
			{Name: "note", Type: spec.FieldString, Index: true},
		},
	}

	for _, tc := range []struct {
		driver      DriverType
		wantExpr    string
		unwantedOld string
	}{
		{DriverSQLite, "json_extract(data, '$.price.amount')", "json_extract(data, '$.price')"},
		{DriverPostgres, "data->'price'->>'amount'", "data->>'price'"},
	} {
		ti, err := GenerateEntityDDL(meta, entity, tc.driver)
		if err != nil {
			t.Fatalf("%s: generate DDL: %v", tc.driver, err)
		}
		ddl := ti.CreateTableSQL + "\n" + strings.Join(ti.CreateIndexSQL, "\n")

		if !strings.Contains(ddl, tc.wantExpr) {
			t.Errorf("%s: money derived column must read .amount, got:\n%s", tc.driver, ddl)
		}
		if strings.Contains(ddl, tc.unwantedOld) {
			t.Errorf("%s: money must not be derived from the whole JSON object:\n%s", tc.driver, ddl)
		}
		// The derived column must be numeric, not text: that is what makes the
		// comparison numeric (the string field below keeps the text form).
		if !strings.Contains(ddl, "_price numeric") {
			t.Errorf("%s: money derived column must be numeric, got:\n%s", tc.driver, ddl)
		}
		if !strings.Contains(ddl, "_note text") {
			t.Errorf("%s: a string field's derived column stays text, got:\n%s", tc.driver, ddl)
		}
	}
}

// The same rule at query time: ordering and range filters over a money field
// compare numbers. This is the behaviour the kafe menu list depends on ("paling
// mahal dulu"), and the one that was silently wrong — 9000 sorted after 10000.
func TestEntityStore_MoneySortAndRangeAreNumeric(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "money-sort.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	ctx := context.Background()
	meta := spec.Metadata{Name: "menu-item-price", Module: "cafe-master"}
	entity := &spec.EntitySpec{
		Version:        "v1",
		Plural:         "menu-item-prices",
		Characteristic: spec.CharMaster,
		Fields: []spec.Field{
			{Name: "price", Type: spec.FieldMoney, Index: true},
		},
	}

	runner := NewMigrationRunner(d, DriverSQLite)
	if _, err := runner.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	store := NewEntityStore(d, DriverSQLite, meta, entity)

	for _, amount := range []string{"9000", "10000"} {
		if _, err := store.Insert(ctx, InsertParams{
			WorkspaceID: "kafe",
			Data:        map[string]any{"price": spec.Money{Amount: amount, Currency: "IDR"}},
		}); err != nil {
			t.Fatalf("insert %s: %v", amount, err)
		}
	}

	asc, err := store.List(ctx, ListParams{WorkspaceID: "kafe", Sort: "price"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(asc.Data) != 2 {
		t.Fatalf("want 2 rows, got %d", len(asc.Data))
	}
	if got := amountOf(t, asc.Data[0]); got != "9000" {
		t.Fatalf("ascending by money: first = %q, want 9000 (lexicographic order gives 10000)", got)
	}
	if got := amountOf(t, asc.Data[1]); got != "10000" {
		t.Fatalf("ascending by money: second = %q, want 10000", got)
	}

	filtered, err := store.List(ctx, ListParams{
		WorkspaceID: "kafe",
		Filters:     map[string]FilterOp{"price": {Op: "gte", Value: "9500"}},
	})
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}
	if len(filtered.Data) != 1 || amountOf(t, filtered.Data[0]) != "10000" {
		t.Fatalf("price >= 9500 must return only 10000, got %d row(s)", len(filtered.Data))
	}
}

// amountOf reads the amount out of a stored money value, whichever shape the
// record carries.
func amountOf(t *testing.T, rec EntityRecord) string {
	t.Helper()
	switch v := rec.Data["price"].(type) {
	case spec.Money:
		return v.Amount
	case map[string]any:
		if s, ok := v["amount"].(string); ok {
			return s
		}
	case string:
		return v
	}
	t.Fatalf("unexpected money shape: %#v", rec.Data["price"])
	return ""
}
