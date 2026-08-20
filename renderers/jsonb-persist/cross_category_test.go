package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestCrossCategoryJoinBlock(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "crosscat.db"), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()

	// Two entities in different categories.
	operational := spec.Metadata{Name: "order", Module: "sales"}
	operationalEntity := &spec.EntitySpec{
		Version: "v1",
		Persist: &spec.PersistSpec{Category: "operational"},
		Fields:  []spec.Field{{Name: "number", Type: spec.FieldString}},
		Actions: []spec.Action{{Name: "submit", Disabled: true}},
	}
	financial := spec.Metadata{Name: "invoice", Module: "fin"}
	financialEntity := &spec.EntitySpec{
		Version: "v1",
		Persist: &spec.PersistSpec{Category: "financial"},
		Fields: []spec.Field{
			{Name: "number", Type: spec.FieldString},
			{
				Name:     "order_id",
				Type:     spec.FieldRelation,
				Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "sales.order"},
			},
		},
		Actions: []spec.Action{{Name: "submit", Disabled: true}},
	}

	if _, err := r.ApplyMigrations(ctx, []EntityMigration{
		{Metadata: operational, EntitySpec: *operationalEntity},
		{Metadata: financial, EntitySpec: *financialEntity},
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Wire a cross-category resolver: sales.order → operational, fin.invoice → financial.
	cat := func(module, entity string) string {
		if module == "sales" {
			return "operational"
		}
		return "financial"
	}

	// Insert an order.
	orderStore := NewEntityStore(d, DriverSQLite, operational, operationalEntity)
	orderStore.category = "operational"
	orderStore.SetTargetCategoryResolver(cat)
	orderID, err := orderStore.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"number": "O-1"}})
	if err != nil {
		t.Fatalf("insert order: %v", err)
	}

	// Insert an invoice referencing the order (cross-category).
	invoiceStore := NewEntityStore(d, DriverSQLite, financial, financialEntity)
	invoiceStore.category = "financial"
	invoiceStore.SetTargetCategoryResolver(cat)
	if _, err := invoiceStore.Insert(ctx, InsertParams{WorkspaceID: "demo", Data: map[string]any{"number": "INV-1", "order_id": orderID}}); err != nil {
		t.Fatalf("insert invoice: %v", err)
	}

	// List invoices — the cross-category relation must NOT be resolved.
	res, err := invoiceStore.List(ctx, ListParams{WorkspaceID: "demo", Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.Data) != 1 {
		t.Fatalf("expected 1 invoice, got %d", len(res.Data))
	}
	// The relation alias "order" should not be populated (cross-category blocked).
	if _, ok := res.Data[0].Data["order"]; ok {
		t.Fatal("expected cross-category relation NOT resolved")
	}
}
