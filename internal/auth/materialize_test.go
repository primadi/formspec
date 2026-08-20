package auth

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupMaterializer builds a UI registry (page/form/table) + entity registry
// (billing.order, billing.customer) and returns a Materializer over them.
func setupMaterializer(t *testing.T) (*Materializer, *entity.Registry) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "mat_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, "")
	// Register entities so the materializer can resolve plurals.
	for _, e := range []struct {
		name   string
		plural string
	}{
		{"order", "orders"},
		{"customer", "customers"},
	} {
		if err := reg.RegisterCoreEntity("billing", e.name, "test", &spec.EntitySpec{
			Version:        "v1",
			Plural:         e.plural,
			Characteristic: spec.CharMaster,
			Fields:         []spec.Field{{Name: "name", Type: spec.FieldString}},
		}); err != nil {
			t.Fatalf("RegisterCoreEntity %s: %v", e.name, err)
		}
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("SyncSchema: %v", err)
	}

	uiReg := ui.NewRegistry()
	// order-create form (mode create → billing.orders.create)
	uiReg.Forms["order-create"] = &ui.Entry[spec.FormSpec]{
		Name: "order-create", Module: "billing",
		Spec: &spec.FormSpec{Entity: "order", Mode: "create"},
	}
	// order-table (→ billing.orders.list + view)
	uiReg.Tables["order-table"] = &ui.Entry[spec.TableSpec]{
		Name: "order-table", Module: "billing",
		Spec: &spec.TableSpec{Entity: "order"},
	}
	// customer-table (→ billing.customers.list + view)
	uiReg.Tables["customer-table"] = &ui.Entry[spec.TableSpec]{
		Name: "customer-table", Module: "billing",
		Spec: &spec.TableSpec{Entity: "customer"},
	}
	// Tabbed page: Tab Order (form create + table), Tab Customer (table).
	uiReg.Pages["sales"] = &ui.Entry[spec.PageSpec]{
		Name: "sales", Module: "billing",
		Spec: &spec.PageSpec{
			Route: "/sales",
			Tabs: []spec.PageTab{
				{Label: "Order", Form: &spec.BlockRef{Ref: "order-create"}, Table: &spec.BlockRef{Ref: "order-table"}},
				{Label: "Customer", Table: &spec.BlockRef{Ref: "customer-table"}},
			},
		},
	}
	// Block page: single table.
	uiReg.Pages["order-list"] = &ui.Entry[spec.PageSpec]{
		Name: "order-list", Module: "billing",
		Spec: &spec.PageSpec{
			Route:  "/orders",
			Blocks: []spec.PageBlock{{Table: &spec.BlockRef{Ref: "order-table"}}},
		},
	}

	return NewMaterializer(uiReg, reg), reg
}

func TestMaterialize_TabbedPage(t *testing.T) {
	m, _ := setupMaterializer(t)

	perms, err := m.Materialize([]Grant{{
		Page: "sales",
		Tabs: []TabGrant{
			{Tab: "Order", Actions: []ActionGrant{{Name: "create"}, {Name: "list"}}},
			{Tab: "Customer", Actions: []ActionGrant{{Name: "list"}}},
		},
	}})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	sort.Strings(perms)
	want := []string{"billing.customers.list", "billing.orders.create", "billing.orders.list"}
	if len(perms) != len(want) {
		t.Fatalf("got %v, want %v", perms, want)
	}
	for i := range want {
		if perms[i] != want[i] {
			t.Errorf("perms[%d]=%q, want %q", i, perms[i], want[i])
		}
	}
}

func TestMaterialize_BlockPage(t *testing.T) {
	m, _ := setupMaterializer(t)

	perms, err := m.Materialize([]Grant{{
		Page:    "order-list",
		Actions: []ActionGrant{{Name: "list"}, {Name: "view"}},
	}})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	sort.Strings(perms)
	want := []string{"billing.orders.list", "billing.orders.view"}
	if len(perms) != len(want) {
		t.Fatalf("got %v, want %v", perms, want)
	}
	for i := range want {
		if perms[i] != want[i] {
			t.Errorf("perms[%d]=%q, want %q", i, perms[i], want[i])
		}
	}
}

func TestMaterialize_UnknownPage(t *testing.T) {
	m, _ := setupMaterializer(t)
	if _, err := m.Materialize([]Grant{{Page: "nope"}}); err == nil {
		t.Fatal("expected error for unknown page")
	}
}

func TestMaterialize_Deduplicates(t *testing.T) {
	m, _ := setupMaterializer(t)
	perms, err := m.Materialize([]Grant{
		{Page: "order-list", Actions: []ActionGrant{{Name: "list"}}},
		{Page: "order-list", Actions: []ActionGrant{{Name: "list"}}},
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(perms) != 1 {
		t.Fatalf("expected dedup to 1, got %v", perms)
	}
}
