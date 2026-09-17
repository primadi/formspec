package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// Money aggregation (S7 / gap #28): a money value is the object
// {amount, currency}, so SUM(money) must sum `.amount` — summing the JSON text
// of the object silently yields 0, which is exactly the bug reports and
// dashboard metrics were hitting.

// setupMoneyAggregateStore creates an entity with a money field and a money
// child line, and inserts sample rows.
func setupMoneyAggregateStore(t *testing.T) *EntityStore {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "money.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	meta := spec.Metadata{Name: "order", Module: "kafe"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "total_amount", Type: spec.FieldMoney},
			{Name: "note", Type: spec.FieldText},
			{Name: "status", Type: spec.FieldString},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)
	rows := []map[string]any{
		{"total_amount": spec.Money{Amount: "25000", Currency: "IDR"}, "status": "paid"},
		{"total_amount": spec.Money{Amount: "12500", Currency: "IDR"}, "status": "paid"},
		{"total_amount": spec.Money{Amount: "10000", Currency: "IDR"}, "status": "open"},
	}
	for _, row := range rows {
		if _, err := store.Insert(ctx, InsertParams{WorkspaceID: "kafe", Data: row}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	return store
}

// TestAggregate_MoneySumsAmount — SUM/AVG/MIN/MAX over a money field.
func TestAggregate_MoneySumsAmount(t *testing.T) {
	store := setupMoneyAggregateStore(t)
	ctx := context.Background()

	sum, err := store.Aggregate(ctx, AggregateParams{
		WorkspaceID: "kafe", Func: "sum", Field: "total_amount",
	})
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if sum.Groups[0].Value != 47500 {
		t.Fatalf("SUM(total_amount): want 47500, got %v", sum.Groups[0].Value)
	}

	avg, err := store.Aggregate(ctx, AggregateParams{
		WorkspaceID: "kafe", Func: "avg", Field: "total_amount",
	})
	if err != nil {
		t.Fatalf("avg: %v", err)
	}
	if avg.Groups[0].Value != 47500.0/3 {
		t.Fatalf("AVG(total_amount): want %v, got %v", 47500.0/3, avg.Groups[0].Value)
	}

	// Grouped money total with a pre-aggregation filter.
	paid, err := store.Aggregate(ctx, AggregateParams{
		WorkspaceID: "kafe", Func: "sum", Field: "total_amount",
		Filters: map[string]FilterOp{"status": {Op: "eq", Value: "paid"}},
	})
	if err != nil {
		t.Fatalf("filtered sum: %v", err)
	}
	if paid.Groups[0].Value != 37500 {
		t.Fatalf("filtered SUM(total_amount): want 37500, got %v", paid.Groups[0].Value)
	}
}

// TestAggregate_NonNumericFieldRejected — the engine must refuse, loudly, to
// sum something that is not a number (and must not quietly return 0).
func TestAggregate_NonNumericFieldRejected(t *testing.T) {
	store := setupMoneyAggregateStore(t)
	ctx := context.Background()

	for _, tc := range []struct{ fn, field, want string }{
		{"sum", "note", "not numeric"},
		{"avg", "note", "not numeric"},
		{"min", "status", "not numeric"},
		{"sum", "does_not_exist", "unknown field"},
	} {
		_, err := store.Aggregate(ctx, AggregateParams{
			WorkspaceID: "kafe", Func: tc.fn, Field: tc.field,
		})
		if err == nil {
			t.Fatalf("%s(%s): want error, got none", tc.fn, tc.field)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s(%s): want error containing %q, got %q", tc.fn, tc.field, tc.want, err.Error())
		}
	}

	// COUNT is defined over any field — it stays legal.
	if _, err := store.Aggregate(ctx, AggregateParams{
		WorkspaceID: "kafe", Func: "count", Field: "note",
	}); err != nil {
		t.Fatalf("count(note): unexpected error: %v", err)
	}
}

// TestWindow_RunningTotalMoney — running_total over a money field sums amounts.
func TestWindow_RunningTotalMoney(t *testing.T) {
	store := setupMoneyAggregateStore(t)
	ctx := context.Background()

	res, err := store.Window(ctx, WindowParams{
		WorkspaceID: "kafe", Func: "running_total", Field: "total_amount",
		OrderBy: []string{"total_amount"},
	})
	if err != nil {
		t.Fatalf("window: %v", err)
	}
	if len(res.Rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(res.Rows))
	}
	// Ordered ascending by amount: 10000, 12500, 25000 → 10000, 22500, 47500.
	want := []float64{10000, 22500, 47500}
	for i, w := range want {
		if res.Rows[i].Value != w {
			t.Fatalf("row %d: want %v, got %v", i, w, res.Rows[i].Value)
		}
	}

	// A text field is not a valid running total.
	if _, err := store.Window(ctx, WindowParams{
		WorkspaceID: "kafe", Func: "running_total", Field: "note",
	}); err == nil || !strings.Contains(err.Error(), "not numeric") {
		t.Fatalf("want a 'not numeric' error for running_total(note), got %v", err)
	}
}

// TestAggregate_MoneyComputedOverChildren — an order whose total_amount is
// computed from child money lines: the stored value must be a money total, not
// a missing field (the cafe_order formula).
func TestAggregate_MoneyComputedOverChildren(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "computed.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	meta := spec.Metadata{Name: "order", Module: "cafe-order"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{
				Name: "items",
				Type: spec.FieldChild,
				Child: &spec.ChildDecl{Fields: []spec.Field{
					{Name: "quantity", Type: spec.FieldInteger},
					{Name: "unit_price", Type: spec.FieldMoney},
					{
						Name:     "line_total",
						Type:     spec.FieldMoney,
						Computed: &spec.ComputedDecl{Formula: "quantity * unit_price"},
					},
				}},
			},
			{
				Name:     "total_amount",
				Type:     spec.FieldMoney,
				Computed: &spec.ComputedDecl{Formula: `sum([i["line_total"] for i in items])`},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	store := NewEntityStore(d, DriverSQLite, meta, entity)

	id, err := store.Insert(ctx, InsertParams{WorkspaceID: "kafe", Data: map[string]any{
		"items": []any{
			map[string]any{"quantity": 2, "unit_price": spec.Money{Amount: "25000", Currency: "IDR"}},
			map[string]any{"quantity": 1, "unit_price": spec.Money{Amount: "12500", Currency: "IDR"}},
		},
	}})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "kafe", ID: id})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	// The stored value must be the canonical money object on the wire, whether
	// it travels as spec.Money (computed just now) or as a decoded map.
	if got := moneyJSON(t, rec.Data["total_amount"]); got != `{"amount":"62500","currency":"IDR"}` {
		t.Fatalf("total_amount: want {62500 IDR}, got %s", got)
	}

	items, _ := rec.Data["items"].([]any)
	line, _ := items[0].(map[string]any)
	if got := moneyJSON(t, line["line_total"]); got != `{"amount":"50000","currency":"IDR"}` {
		t.Fatalf("line_total: want {50000 IDR}, got %s", got)
	}
}

// moneyJSON renders a money value (struct or decoded map) as its wire JSON.
func moneyJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal money value: %v", err)
	}
	return string(b)
}

// TestList_BooleanFilterAcceptsTrueString — a boolean field is stored as a JSON
// boolean and compared through a numeric cast, so the natural `?flag=true`
// (bound as the string "true") matched nothing at all. A filter that silently
// returns zero rows is worse than one that errors: the kafe QR catalog declares
// `filter: {is_available: "true"}` and would have shown an empty menu.
func TestList_BooleanFilterAcceptsTrueString(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "boolfilter.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	meta := spec.Metadata{Name: "menu-item", Module: "cafe-master"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "is_available", Type: spec.FieldBoolean},
		},
	}
	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	store := NewEntityStore(d, DriverSQLite, meta, entity)
	for _, row := range []map[string]any{
		{"name": "Kopi", "is_available": true},
		{"name": "Roti", "is_available": true},
		{"name": "Menu Lama", "is_available": false},
	} {
		if _, err := store.Insert(ctx, InsertParams{WorkspaceID: "kafe", Data: row}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	for _, value := range []any{"true", "1", true, "TRUE"} {
		res, err := store.List(ctx, ListParams{
			WorkspaceID: "kafe",
			Filters:     map[string]FilterOp{"is_available": {Op: "eq", Value: value}},
		})
		if err != nil {
			t.Fatalf("filter %v: %v", value, err)
		}
		if len(res.Data) != 2 {
			t.Errorf("filter is_available=%v: want 2 rows, got %d", value, len(res.Data))
		}
	}

	res, err := store.List(ctx, ListParams{
		WorkspaceID: "kafe",
		Filters:     map[string]FilterOp{"is_available": {Op: "eq", Value: "false"}},
	})
	if err != nil {
		t.Fatalf("filter false: %v", err)
	}
	if len(res.Data) != 1 {
		t.Errorf("filter is_available=false: want 1 row, got %d", len(res.Data))
	}
}
