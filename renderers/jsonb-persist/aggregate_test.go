package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// setupAggregateStore creates an in-memory store for an entity with a
// numeric `amount` field and a `category` field, and inserts sample rows.
func setupAggregateStore(t *testing.T) *EntityStore {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "agg.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	meta := spec.Metadata{Name: "sale", Module: "agg"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "amount", Type: spec.FieldNumber},
			{Name: "category", Type: spec.FieldString},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	rows := []map[string]any{
		{"amount": 100.0, "category": "a"},
		{"amount": 200.0, "category": "a"},
		{"amount": 300.0, "category": "b"},
		{"amount": 400.0, "category": "b"},
	}
	for _, r := range rows {
		if _, err := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: r}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	return store
}

func TestAggregate_SumCountAvgMinMax(t *testing.T) {
	store := setupAggregateStore(t)
	ctx := context.Background()

	cases := []struct {
		fn    string
		field string
		want  float64
	}{
		{"sum", "amount", 1000},
		{"count", "", 4},
		{"avg", "amount", 250},
		{"min", "amount", 100},
		{"max", "amount", 400},
	}
	for _, c := range cases {
		res, err := store.Aggregate(ctx, AggregateParams{
			WorkspaceID: "demo", Func: c.fn, Field: c.field,
		})
		if err != nil {
			t.Fatalf("%s: %v", c.fn, err)
		}
		if len(res.Groups) != 1 {
			t.Fatalf("%s: expected 1 group, got %d", c.fn, len(res.Groups))
		}
		if res.Groups[0].Value != c.want {
			t.Fatalf("%s: expected %v, got %v", c.fn, c.want, res.Groups[0].Value)
		}
	}
}

func TestAggregate_GroupByAndHaving(t *testing.T) {
	store := setupAggregateStore(t)
	ctx := context.Background()

	// SUM(amount) GROUP BY category → a=300, b=700.
	res, err := store.Aggregate(ctx, AggregateParams{
		WorkspaceID: "demo", Func: "sum", Field: "amount", GroupBy: []string{"category"},
	})
	if err != nil {
		t.Fatalf("group by: %v", err)
	}
	if len(res.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(res.Groups))
	}
	byCat := map[string]float64{}
	for _, g := range res.Groups {
		byCat[g.Key["category"].(string)] = g.Value
	}
	if byCat["a"] != 300 || byCat["b"] != 700 {
		t.Fatalf("unexpected group sums: %v", byCat)
	}

	// HAVING SUM(amount) > 500 → only category b.
	res, err = store.Aggregate(ctx, AggregateParams{
		WorkspaceID: "demo", Func: "sum", Field: "amount", GroupBy: []string{"category"},
		Having: []FilterOp{{Op: "gt", Value: 500}},
	})
	if err != nil {
		t.Fatalf("having: %v", err)
	}
	if len(res.Groups) != 1 {
		t.Fatalf("expected 1 group after having, got %d", len(res.Groups))
	}
	if res.Groups[0].Key["category"] != "b" {
		t.Fatalf("expected category b, got %v", res.Groups[0].Key)
	}
}

func TestAggregate_UnknownFunction(t *testing.T) {
	store := setupAggregateStore(t)
	_, err := store.Aggregate(context.Background(), AggregateParams{
		WorkspaceID: "demo", Func: "median", Field: "amount",
	})
	if err == nil {
		t.Fatal("expected error for unknown aggregate function")
	}
}

// setupDateAggregateStore creates a store with a date field and rows across
// two months.
func setupDateAggregateStore(t *testing.T) *EntityStore {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "aggdate.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	meta := spec.Metadata{Name: "sale", Module: "agg"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "amount", Type: spec.FieldNumber},
			{Name: "sale_date", Type: spec.FieldDate},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	rows := []map[string]any{
		{"amount": 100.0, "sale_date": "2026-01-05"},
		{"amount": 200.0, "sale_date": "2026-01-20"},
		{"amount": 300.0, "sale_date": "2026-02-10"},
	}
	for _, r := range rows {
		if _, err := store.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: r}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	return store
}

func TestAggregate_DateTrunc(t *testing.T) {
	store := setupDateAggregateStore(t)
	ctx := context.Background()

	// SUM(amount) GROUP BY date_trunc('month', sale_date) → 2026-01=300, 2026-02=300.
	res, err := store.Aggregate(ctx, AggregateParams{
		WorkspaceID: "demo", Func: "sum", Field: "amount",
		DateTrunc: &DateTruncDecl{Field: "sale_date", Unit: "month"},
	})
	if err != nil {
		t.Fatalf("date_trunc: %v", err)
	}
	if len(res.Groups) != 2 {
		t.Fatalf("expected 2 month buckets, got %d", len(res.Groups))
	}
	total := 0.0
	for _, g := range res.Groups {
		total += g.Value
	}
	if total != 600 {
		t.Fatalf("expected total 600, got %v", total)
	}
}

func TestAggregate_DateTruncInvalidUnit(t *testing.T) {
	store := setupDateAggregateStore(t)
	_, err := store.Aggregate(context.Background(), AggregateParams{
		WorkspaceID: "demo", Func: "sum", Field: "amount",
		DateTrunc: &DateTruncDecl{Field: "sale_date", Unit: "fortnight"},
	})
	if err == nil {
		t.Fatal("expected error for invalid date_trunc unit")
	}
}

func TestWindow_RunningTotal(t *testing.T) {
	store := setupAggregateStore(t)
	ctx := context.Background()

	// Running total of amount ordered by amount ASC → 100, 300, 600, 1000.
	res, err := store.Window(ctx, WindowParams{
		WorkspaceID: "demo", Func: "running_total", Field: "amount",
		OrderBy: []string{"amount"},
	})
	if err != nil {
		t.Fatalf("running_total: %v", err)
	}
	if len(res.Rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(res.Rows))
	}
	// Values should be monotonically increasing running totals.
	last := 0.0
	for _, r := range res.Rows {
		if r.Value < last {
			t.Fatalf("running total not monotonic: %v then %v", last, r.Value)
		}
		last = r.Value
	}
	if last != 1000 {
		t.Fatalf("expected final running total 1000, got %v", last)
	}
}

func TestWindow_Rank(t *testing.T) {
	store := setupAggregateStore(t)
	ctx := context.Background()

	res, err := store.Window(ctx, WindowParams{
		WorkspaceID: "demo", Func: "rank",
		OrderBy: []string{"-amount"},
	})
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if len(res.Rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(res.Rows))
	}
	// Highest amount (400) should rank 1.
	if res.Rows[0].Value != 1 {
		t.Fatalf("expected rank 1 for highest amount, got %v", res.Rows[0].Value)
	}
}

func TestWindow_UnknownFunction(t *testing.T) {
	store := setupAggregateStore(t)
	_, err := store.Window(context.Background(), WindowParams{
		WorkspaceID: "demo", Func: "lag", Field: "amount",
	})
	if err == nil {
		t.Fatal("expected error for unknown window function")
	}
}
