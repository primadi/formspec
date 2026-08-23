package auth

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
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
	// Register entities so the materializer can resolve plurals. Uses
	// RegisterArtifactManifest (non-internal) so they're visible to
	// ListEntities — the same set the GrantsEditor derives entity pages from.
	for _, e := range []struct {
		name    string
		plural  string
		actions []spec.Action
	}{
		{"order", "orders", []spec.Action{{Name: "approve"}}},
		{"customer", "customers", nil},
	} {
		raw := manifest.RawManifest{
			Kind: "Entity",
			Metadata: manifest.RawMetadata{
				Name:        e.name,
				Module:      "billing",
				Description: "test",
			},
			Source: "test",
		}
		if err := reg.RegisterArtifactManifest(raw, &spec.EntitySpec{
			Version:        "v1",
			Plural:         e.plural,
			Characteristic: spec.CharMaster,
			Fields:         []spec.Field{{Name: "name", Type: spec.FieldString}},
			Actions:        e.actions,
		}); err != nil {
			t.Fatalf("RegisterArtifactManifest %s: %v", e.name, err)
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

	// Navigation kinds for derived-page tests.
	uiReg.Widgets["open-orders"] = &ui.Entry[spec.WidgetSpec]{
		Name: "open-orders", Module: "billing",
		Spec: &spec.WidgetSpec{Title: "Open Orders", Type: "metric", Entity: "order"},
	}
	uiReg.Dashboards["sales-dashboard"] = &ui.Entry[spec.DashboardSpec]{
		Name: "sales-dashboard", Module: "billing",
		Spec: &spec.DashboardSpec{Title: "Sales", Widgets: []spec.DashboardWidget{{Ref: "open-orders"}}},
	}
	uiReg.Reports["sales-recap"] = &ui.Entry[spec.ReportSpec]{
		Name: "sales-recap", Module: "billing",
		Spec: &spec.ReportSpec{Title: "Sales Recap", Entity: "order"},
	}
	uiReg.Kanbans["order-board"] = &ui.Entry[spec.KanbanSpec]{
		Name: "order-board", Module: "billing",
		Spec: &spec.KanbanSpec{Entity: "order", StatusField: "status"},
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

func TestMaterialize_DerivedEntityPage(t *testing.T) {
	m, _ := setupMaterializer(t)
	perms, err := m.Materialize([]Grant{{
		Page:    "order-page",
		Actions: []ActionGrant{{Name: "list"}, {Name: "create"}, {Name: "delete"}},
	}})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	sort.Strings(perms)
	want := []string{"billing.orders.create", "billing.orders.delete", "billing.orders.list"}
	if len(perms) != len(want) {
		t.Fatalf("got %v, want %v", perms, want)
	}
	for i := range want {
		if perms[i] != want[i] {
			t.Errorf("perms[%d]=%q, want %q", i, perms[i], want[i])
		}
	}
}

func TestMaterialize_DerivedEntityPageCustomAction(t *testing.T) {
	m, _ := setupMaterializer(t)
	perms, err := m.Materialize([]Grant{{
		Page:    "order-page",
		Actions: []ActionGrant{{Name: "approve"}},
	}})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	want := []string{"billing.orders.approve"}
	if len(perms) != len(want) || perms[0] != want[0] {
		t.Fatalf("got %v, want %v", perms, want)
	}
}

func TestMaterialize_DashboardNavigation(t *testing.T) {
	m, _ := setupMaterializer(t)
	perms, err := m.Materialize([]Grant{{
		Page:    "dashboard:sales-dashboard",
		Actions: []ActionGrant{{Name: "view"}},
	}})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	want := []string{"billing.orders.view"}
	if len(perms) != len(want) || perms[0] != want[0] {
		t.Fatalf("got %v, want %v", perms, want)
	}
}

func TestMaterialize_ReportNavigation(t *testing.T) {
	m, _ := setupMaterializer(t)
	perms, err := m.Materialize([]Grant{{
		Page:    "report:sales-recap",
		Actions: []ActionGrant{{Name: "view"}},
	}})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	want := []string{"billing.orders.list"}
	if len(perms) != len(want) || perms[0] != want[0] {
		t.Fatalf("got %v, want %v", perms, want)
	}
}

func TestMaterialize_KanbanNavigation(t *testing.T) {
	m, _ := setupMaterializer(t)
	perms, err := m.Materialize([]Grant{{
		Page:    "kanban:order-board",
		Actions: []ActionGrant{{Name: "view"}},
	}})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	want := []string{"billing.orders.view"}
	if len(perms) != len(want) || perms[0] != want[0] {
		t.Fatalf("got %v, want %v", perms, want)
	}
}

func TestMaterialize_UnknownNavigationKind(t *testing.T) {
	m, _ := setupMaterializer(t)
	if _, err := m.Materialize([]Grant{{Page: "gadget:thing"}}); err == nil {
		t.Fatal("expected error for unknown navigation kind")
	}
}
