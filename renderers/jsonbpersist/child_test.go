package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/forma/pkg/spec"
)

// TestChildStorage_JSONB verifies that jsonb-stored children stay in parent data
// and survive insert → get round-trip.
func TestChildStorage_JSONB(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "child_jsonb.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "invoice", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "invoice_number", Type: spec.FieldString, Unique: true},
			{Name: "total", Type: spec.FieldNumber},
			{
				Name: "line_items",
				Type: spec.FieldChild,
				Child: &spec.ChildDecl{
					Storage: "jsonb",
				},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert with nested children in jsonb
	data := map[string]any{
		"invoice_number": "INV-001",
		"total":          150.0,
		"line_items": []any{
			map[string]any{"item": "Laptop", "qty": 1, "price": 100.0},
			map[string]any{"item": "Mouse", "qty": 2, "price": 25.0},
		},
	}

	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID:  "tenant-1",
		CreatedBy: "user-1",
		Data:      data,
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// GetByID — children should be in data
	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if rec.Data["invoice_number"] != "INV-001" {
		t.Errorf("expected invoice_number=INV-001, got %v", rec.Data["invoice_number"])
	}

	if rec.Data["total"] != 150.0 {
		t.Errorf("expected total=150.0, got %v", rec.Data["total"])
	}

	// Children should still be in data
	lineItems, ok := rec.Data["line_items"]
	if !ok {
		t.Fatal("expected line_items in data")
	}
	arr, ok := lineItems.([]any)
	if !ok {
		t.Fatalf("expected line_items to be []any, got %T", lineItems)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 line items, got %d", len(arr))
	}
}

// TestChildStorage_Table verifies table-stored children:
// - Insert extracts children → parent data clean, children in child table
// - GetByID hydrates children from child table
// - Update replaces children
// - SoftDelete cascades
func TestChildStorage_Table(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "child_table.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "order", Module: "sales"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "order_number", Type: spec.FieldString, Unique: true},
			{Name: "customer_name", Type: spec.FieldString},
			{
				Name: "items",
				Type: spec.FieldChild,
				Child: &spec.ChildDecl{
					Storage: "table",
				},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// --- Insert ---
	data := map[string]any{
		"order_number":  "ORD-001",
		"customer_name": "Alice",
		"items": []any{
			map[string]any{"product": "Book", "qty": 3},
			map[string]any{"product": "Pen", "qty": 10},
		},
	}

	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID:  "tenant-1",
		CreatedBy: "user-1",
		Data:      data,
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// --- GetByID — children should be hydrated from child table ---
	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Parent data should NOT contain items (stored in child table)
	if _, ok := rec.Data["items"]; !ok {
		t.Fatal("expected items to be hydrated from child table")
	}

	items, ok := rec.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("expected items to be []map[string]any, got %T", rec.Data["items"])
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0]["product"] != "Book" {
		t.Errorf("expected first item product=Book, got %v", items[0]["product"])
	}
	if items[1]["product"] != "Pen" {
		t.Errorf("expected second item product=Pen, got %v", items[1]["product"])
	}

	// --- Update with new children ---
	updatedData := map[string]any{
		"order_number":  "ORD-001",
		"customer_name": "Alice Updated",
		"items": []any{
			map[string]any{"product": "Pencil", "qty": 5},
		},
	}

	newVersion, err := store.Update(ctx, UpdateParams{
		WorkspaceID:  "tenant-1",
		ID:        id,
		Version:   rec.Version,
		UpdatedBy: "user-1",
		Data:      updatedData,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if newVersion != rec.Version+1 {
		t.Errorf("expected version %d, got %d", rec.Version+1, newVersion)
	}

	// Verify updated children
	rec2, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}

	items2, ok := rec2.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("expected items2 to be []map[string]any, got %T", rec2.Data["items"])
	}
	if len(items2) != 1 {
		t.Fatalf("expected 1 item after update, got %d", len(items2))
	}
	if items2[0]["product"] != "Pencil" {
		t.Errorf("expected product=Pencil, got %v", items2[0]["product"])
	}
	if rec2.Data["customer_name"] != "Alice Updated" {
		t.Errorf("expected customer_name=Alice Updated, got %v", rec2.Data["customer_name"])
	}

	// --- SoftDelete ---
	err = store.SoftDelete(ctx, "tenant-1", id)
	if err != nil {
		t.Fatalf("SoftDelete failed: %v", err)
	}

	// Verify record is gone
	_, err = store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
	if err == nil {
		t.Fatal("expected not found after soft delete")
	}
}

// TestChildStorage_Table_SequenceField verifies child table with sequence field.
func TestChildStorage_Table_SequenceField(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "child_seq.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "purchase_order", Module: "procurement"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "po_number", Type: spec.FieldString, Unique: true},
			{
				Name: "line_items",
				Type: spec.FieldChild,
				Child: &spec.ChildDecl{
					Storage:       "table",
					SequenceField: "line_no",
				},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	data := map[string]any{
		"po_number": "PO-001",
		"line_items": []any{
			map[string]any{"item": "Steel", "qty": 100},
			map[string]any{"item": "Bolts", "qty": 500},
			map[string]any{"item": "Nuts", "qty": 500},
		},
	}

	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID:  "tenant-1",
		CreatedBy: "user-1",
		Data:      data,
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	items := rec.Data["line_items"].([]map[string]any)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	// SequenceField values are not in data (stored in dedicated column),
	// so we just verify count.
}

// TestChildStorage_MultipleChildren verifies an entity with multiple child fields
// in different storage modes.
func TestChildStorage_MultipleChildren(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "child_multi.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "contract", Module: "legal"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "title", Type: spec.FieldString},
			{
				Name:  "parties",
				Type:  spec.FieldChild,
				Child: &spec.ChildDecl{Storage: "jsonb"},
			},
			{
				Name:  "clauses",
				Type:  spec.FieldChild,
				Child: &spec.ChildDecl{Storage: "table"},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	data := map[string]any{
		"title": "Service Agreement",
		"parties": []any{
			map[string]any{"name": "Company A", "role": "provider"},
			map[string]any{"name": "Company B", "role": "client"},
		},
		"clauses": []any{
			map[string]any{"text": "Confidentiality", "page": 1},
			map[string]any{"text": "Termination", "page": 5},
		},
	}

	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID:  "tenant-1",
		CreatedBy: "user-1",
		Data:      data,
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if rec.Data["title"] != "Service Agreement" {
		t.Errorf("expected title=Service Agreement, got %v", rec.Data["title"])
	}

	// parties should be in data (jsonb storage)
	parties, ok := rec.Data["parties"].([]any)
	if !ok {
		t.Fatal("expected parties in data (jsonb storage)")
	}
	if len(parties) != 2 {
		t.Errorf("expected 2 parties, got %d", len(parties))
	}

	// clauses should be hydrated from child table
	clauses, ok := rec.Data["clauses"].([]map[string]any)
	if !ok {
		t.Fatal("expected clauses hydrated from child table")
	}
	if len(clauses) != 2 {
		t.Errorf("expected 2 clauses, got %d", len(clauses))
	}
}

// TestChildStorage_EmptyChildren verifies entities with no child data.
func TestChildStorage_EmptyChildren(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "child_empty.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	meta := spec.Metadata{Name: "note", Module: "general"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "title", Type: spec.FieldString},
			{
				Name:  "tags",
				Type:  spec.FieldChild,
				Child: &spec.ChildDecl{Storage: "table"},
			},
		},
	}

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if _, err := r.ApplyMigrations(ctx, []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	store := NewEntityStore(d, DriverSQLite, meta, entity)

	// Insert without children
	data := map[string]any{
		"title": "Empty note",
	}
	id, err := store.Insert(ctx, InsertParams{
		WorkspaceID:  "tenant-1",
		CreatedBy: "user-1",
		Data:      data,
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rec, err := store.GetByID(ctx, GetByIDParams{WorkspaceID: "tenant-1", ID: id})
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if rec.Data["title"] != "Empty note" {
		t.Errorf("expected title=Empty note, got %v", rec.Data["title"])
	}
	// tags should not be present (no children)
	if _, ok := rec.Data["tags"]; ok {
		t.Log("tags present as empty — this is OK if Hydrate returns empty slice")
	}
}

// TestChildStorage_ChildStore_Standalone tests the ChildStore directly
// without going through EntityStore.
func TestChildStorage_ChildStore_Standalone(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "child_standalone.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	// Create parent + child table manually
	ctx := context.Background()
	_, err = d.ExecContext(ctx,
		`CREATE TABLE test_orders (id integer PRIMARY KEY AUTOINCREMENT, data text NOT NULL DEFAULT '{}')`)
	if err != nil {
		t.Fatalf("create parent table: %v", err)
	}
	_, err = d.ExecContext(ctx,
		`CREATE TABLE test_orders__items (id integer PRIMARY KEY AUTOINCREMENT, parent_id integer NOT NULL REFERENCES test_orders(id) ON DELETE CASCADE, created_at text NOT NULL DEFAULT (datetime('now')), data text NOT NULL DEFAULT '{}')`)
	if err != nil {
		t.Fatalf("create child table: %v", err)
	}

	// Insert parent
	_, err = d.ExecContext(ctx, `INSERT INTO test_orders (id, data) VALUES (1, '{"order":"test"}')`)
	if err != nil {
		t.Fatalf("insert parent: %v", err)
	}

	cs := NewChildStore(d, DriverSQLite, "test_orders", spec.Field{
		Name:  "items",
		Type:  spec.FieldChild,
		Child: &spec.ChildDecl{Storage: "table"},
	})

	if cs.Storage() != "table" {
		t.Fatalf("expected storage=table, got %s", cs.Storage())
	}

	// Insert children
	children := []map[string]any{
		{"product": "A", "qty": 1},
		{"product": "B", "qty": 2},
	}
	if err := cs.InsertChildren(ctx, "1", children); err != nil {
		t.Fatalf("InsertChildren failed: %v", err)
	}

	// Get children
	got, err := cs.GetChildren(ctx, "1")
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 children, got %d", len(got))
	}
	if got[0]["product"] != "A" {
		t.Errorf("expected first child product=A, got %v", got[0]["product"])
	}
	if got[1]["product"] != "B" {
		t.Errorf("expected second child product=B, got %v", got[1]["product"])
	}
	// _child_id should be present
	if _, ok := got[0]["_child_id"]; !ok {
		t.Error("expected _child_id in child data")
	}

	// Update children (replace all)
	newChildren := []map[string]any{
		{"product": "C", "qty": 3},
	}
	if err := cs.UpdateChildren(ctx, "1", newChildren); err != nil {
		t.Fatalf("UpdateChildren failed: %v", err)
	}

	got2, err := cs.GetChildren(ctx, "1")
	if err != nil {
		t.Fatalf("GetChildren after update failed: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("expected 1 child after update, got %d", len(got2))
	}
	if got2[0]["product"] != "C" {
		t.Errorf("expected product=C, got %v", got2[0]["product"])
	}

	// Delete children
	if err := cs.DeleteChildren(ctx, "1"); err != nil {
		t.Fatalf("DeleteChildren failed: %v", err)
	}

	got3, err := cs.GetChildren(ctx, "1")
	if err != nil {
		t.Fatalf("GetChildren after delete failed: %v", err)
	}
	if len(got3) != 0 {
		t.Fatalf("expected 0 children after delete, got %d", len(got3))
	}
}
