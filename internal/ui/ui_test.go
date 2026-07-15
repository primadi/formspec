package ui

import (
	"strings"
	"testing"

	"github.com/primadi/forma/internal/manifest"
	"github.com/primadi/forma/pkg/spec"
)

// fixtureYAML is a minimal app exercising every frontend kind + cross-refs.
const fixtureYAML = `
apiVersion: forma.dev/v1alpha1
kind: Table
metadata: { name: order-table, module: billing }
spec:
  entity: order
  default_sort: "-created_at"
  columns:
    - { field: number, label: "No." }
    - { field: customer.name, label: "Customer" }
    - { field: status }
  filters:
    - { field: status, label: Status, type: select }
  row_actions:
    - { action: view, label: Detail }
    - { action: checkout, label: Checkout }
---
apiVersion: forma.dev/v1alpha1
kind: Form
metadata: { name: order-edit, module: billing }
spec:
  entity: order
  render: { mode: separate_page }
  sections:
    - title: Main
      fields:
        - { name: number, read_only: true }
        - { name: status, visible_when: "fields.status != 'draft'" }
---
apiVersion: forma.dev/v1alpha1
kind: Page
metadata: { name: order-list, module: billing }
spec:
  route: /app/orders
  title: Orders
  permissions: [orders.list]
  blocks:
    - table: { ref: order-table, entity: order }
---
apiVersion: forma.dev/v1alpha1
kind: Page
metadata: { name: settings, module: billing }
spec:
  route: /app/settings
  title: Settings
  tabs:
    - { label: General, form: { ref: order-edit, entity: order, id: "1" } }
---
apiVersion: forma.dev/v1alpha1
kind: Widget
metadata: { name: rev-today, module: billing }
spec:
  title: Revenue
  type: metric
  entity: order
---
apiVersion: forma.dev/v1alpha1
kind: Dashboard
metadata: { name: main-dash, module: billing }
spec:
  title: Main
  widgets:
    - { ref: rev-today, layout: { x: 0, y: 0, w: 1, h: 1 } }
---
apiVersion: forma.dev/v1alpha1
kind: Kanban
metadata: { name: order-board, module: billing }
spec:
  entity: order
  status_field: status
  columns:
    - { status: draft, label: Draft }
    - { status: paid, label: Paid }
---
apiVersion: forma.dev/v1alpha1
kind: Timeline
metadata: { name: order-history, module: billing }
spec:
  entity: order
  date_field: created_at
  bind_param: customer_id
---
apiVersion: forma.dev/v1alpha1
kind: Wizard
metadata: { name: checkout-wiz, module: billing }
spec:
  title: Checkout
  entity: order
  steps:
    - { title: Edit, form: order-edit }
    - { title: Commit, action: checkout }
---
apiVersion: forma.dev/v1alpha1
kind: Report
metadata: { name: sales, module: billing }
spec:
  title: Sales
  entity: order
  columns:
    - { field: number, label: "No." }
---
apiVersion: forma.dev/v1alpha1
kind: Print
metadata: { name: receipt, module: billing }
spec:
  entity: order
  output: { format: thermal, paper: { size: thermal_58mm } }
  body:
    - fields: [number]
    - separator: "---"
---
apiVersion: forma.dev/v1alpha1
kind: Theme
metadata: { name: dark, module: billing }
spec:
  tokens:
    color.primary: "#000"
`

func orderEntity() *spec.EntitySpec {
	return &spec.EntitySpec{
		Plural:         "orders",
		Characteristic: "transaction",
		Expose:         []spec.ExposeConfig{{Type: "rest", Enabled: true}},
		Fields: []spec.Field{
			{Name: "number", Type: spec.FieldString, NaturalKey: true},
			{Name: "status", Type: spec.FieldEnum, EnumValues: []string{"draft", "paid"}, Index: true},
			{Name: "customer_id", Type: spec.FieldRelation, Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "customer"}},
			{Name: "total", Type: spec.FieldDecimal},
		},
		Actions: []spec.Action{
			{Name: "checkout", RequiredPermission: "orders.checkout"},
			{Name: "delete", Disabled: true},
		},
	}
}

func customerEntity() *spec.EntitySpec {
	return &spec.EntitySpec{
		Plural: "customers",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
		},
	}
}

func loadFixture(t *testing.T) *Registry {
	t.Helper()
	loader := manifest.NewLoader("")
	raws, errs := loader.ParseBytes([]byte(fixtureYAML), "fixture.yaml")
	if len(errs) > 0 {
		t.Fatalf("parse fixture: %v", errs)
	}
	r := NewRegistry()
	if loadErrs := r.Load(raws); len(loadErrs) > 0 {
		t.Fatalf("load fixture: %v", loadErrs)
	}
	return r
}

func testResolver() EntityResolver {
	return func(module, name string) (*spec.EntitySpec, bool) {
		if module != "billing" {
			return nil, false
		}
		switch name {
		case "order":
			return orderEntity(), true
		case "customer":
			return customerEntity(), true
		}
		return nil, false
	}
}

func TestLoadAllKinds(t *testing.T) {
	r := loadFixture(t)
	if r.Count() != 12 {
		t.Fatalf("expected 12 manifests, got %d", r.Count())
	}
	if r.Pages["settings"].Spec.Tabs[0].Label != "General" {
		t.Errorf("tabs not parsed")
	}
	if r.Forms["order-edit"].Spec.Render.Mode != "separate_page" {
		t.Errorf("render mode not parsed")
	}
	if r.Prints["receipt"].Spec.Output.Paper.Size != "thermal_58mm" {
		t.Errorf("print output not parsed")
	}
	if r.Themes["dark"].Spec.Tokens["color.primary"] != "#000" {
		t.Errorf("theme tokens not parsed")
	}
	if r.Timelines["order-history"].Spec.BindParam != "customer_id" {
		t.Errorf("timeline bind_param not parsed")
	}
}

func TestDuplicateNameRejected(t *testing.T) {
	loader := manifest.NewLoader("")
	dup := `
apiVersion: forma.dev/v1alpha1
kind: Theme
metadata: { name: main, module: a }
spec: { tokens: { color.primary: "#111" } }
---
apiVersion: forma.dev/v1alpha1
kind: Theme
metadata: { name: main, module: b }
spec: { tokens: { color.primary: "#222" } }
`
	raws, _ := loader.ParseBytes([]byte(dup), "dup.yaml")
	r := NewRegistry()
	errs := r.Load(raws)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", errs)
	}
}

func TestValidateClean(t *testing.T) {
	r := loadFixture(t)
	if errs := r.Validate(testResolver()); len(errs) > 0 {
		t.Fatalf("expected clean validation, got: %v", errs)
	}
}

func TestValidateCatchesErrors(t *testing.T) {
	broken := `
apiVersion: forma.dev/v1alpha1
kind: Table
metadata: { name: bad-table, module: billing }
spec:
  entity: nonexistent
  columns: [{ field: x }]
---
apiVersion: forma.dev/v1alpha1
kind: Form
metadata: { name: bad-form, module: billing }
spec:
  entity: order
  sections:
    - title: T
      fields:
        - { name: no_such_field }
---
apiVersion: forma.dev/v1alpha1
kind: Kanban
metadata: { name: bad-board, module: billing }
spec:
  entity: order
  status_field: status
  columns:
    - { status: bogus_status, label: X }
---
apiVersion: forma.dev/v1alpha1
kind: Page
metadata: { name: p1, module: billing }
spec:
  route: /dup
  title: A
  blocks: [{ table: { ref: missing-table } }]
---
apiVersion: forma.dev/v1alpha1
kind: Page
metadata: { name: p2, module: billing }
spec:
  route: /dup
  title: B
---
apiVersion: forma.dev/v1alpha1
kind: Table
metadata: { name: disabled-action-table, module: billing }
spec:
  entity: order
  columns: [{ field: number }]
  row_actions:
    - { action: delete, label: Del }
    - { action: nope, label: Nope }
`
	loader := manifest.NewLoader("")
	raws, _ := loader.ParseBytes([]byte(broken), "broken.yaml")
	r := NewRegistry()
	if errs := r.Load(raws); len(errs) > 0 {
		t.Fatalf("load: %v", errs)
	}
	errs := r.Validate(testResolver())

	wantSubstrings := []string{
		`entity "nonexistent" not found`,
		`field "no_such_field" not on entity`,
		`column status "bogus_status"`,
		`table ref "missing-table" not found`,
		`duplicate route "/dup"`,
		`action "nope" not on entity`,
	}
	for _, want := range wantSubstrings {
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected error %q in: %v", want, errs)
		}
	}
	// "delete" row action references a disabled action → builtin, allowed
	// (renderer hides it because the route doesn't exist). Ensure no error for it:
	for _, e := range errs {
		if strings.Contains(e.Error(), `action "delete"`) {
			t.Errorf("builtin delete should not error: %v", e)
		}
	}
}

func TestDotPathRelationValidation(t *testing.T) {
	r := loadFixture(t)
	// customer.name resolves through the relation to customer entity — must be clean.
	if errs := r.Validate(testResolver()); len(errs) > 0 {
		t.Fatalf("dot-path should validate: %v", errs)
	}

	// unknown target field through relation must error
	bad := `
apiVersion: forma.dev/v1alpha1
kind: Table
metadata: { name: t2, module: billing }
spec:
  entity: order
  columns: [{ field: customer.bogus }]
`
	loader := manifest.NewLoader("")
	raws, _ := loader.ParseBytes([]byte(bad), "bad.yaml")
	r2 := NewRegistry()
	r2.Load(raws)
	errs := r2.Validate(testResolver())
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "customer.bogus") {
		t.Fatalf("expected dot-path error, got %v", errs)
	}
}

func TestBuildBundlePermissionFiltering(t *testing.T) {
	r := loadFixture(t)
	entities := func() []EntityDescriptor {
		return []EntityDescriptor{
			{Module: "billing", Name: "order", Spec: orderEntity()},
			{Module: "billing", Name: "customer", Spec: customerEntity()},
		}
	}

	t.Run("admin sees everything", func(t *testing.T) {
		b := r.BuildBundle(entities, func(string) bool { return true }, AppContext{})
		if len(b.Entities) != 2 {
			t.Errorf("entities: want 2, got %d", len(b.Entities))
		}
		if len(b.Tables) != 1 || len(b.Pages) != 2 || len(b.Kanbans) != 1 ||
			len(b.Forms) != 1 || len(b.Widgets) != 1 || len(b.Themes) != 1 {
			t.Errorf("unexpected bundle counts: %+v", b)
		}
	})

	t.Run("no permissions sees nothing entity-backed", func(t *testing.T) {
		b := r.BuildBundle(entities, func(string) bool { return false }, AppContext{})
		if len(b.Entities) != 0 || len(b.Tables) != 0 || len(b.Forms) != 0 || len(b.Kanbans) != 0 {
			t.Errorf("expected empty entity-backed sections: %+v", b)
		}
		// order-list page requires orders.list → hidden; settings page has no perms → visible
		if len(b.Pages) != 1 || b.Pages[0].Name != "settings" {
			t.Errorf("expected only settings page, got %+v", b.Pages)
		}
		// themes always ship
		if len(b.Themes) != 1 {
			t.Errorf("themes should always ship")
		}
	})

	t.Run("scoped permission", func(t *testing.T) {
		can := func(p string) bool { return p == "billing.orders.list" }
		b := r.BuildBundle(entities, can, AppContext{})
		if len(b.Entities) != 1 || b.Entities[0].Name != "order" {
			t.Errorf("expected only order entity, got %+v", b.Entities)
		}
		// page permission "orders.list" auto-prefixed with module billing
		foundOrderList := false
		for _, p := range b.Pages {
			if p.Name == "order-list" {
				foundOrderList = true
			}
		}
		if !foundOrderList {
			t.Errorf("order-list page should be visible via module-relative permission")
		}
	})
}

// TestBuildBundle_AppContextModuleScoping verifies the module-filtering
// behavior AppContext.allows() gives (Core §4.4): a zero-value AppContext
// (used by the `_admin` surface, which isn't scoped to any App) sees entities
// from every module, while a scoped AppContext only sees its own modules'
// entities — even for a module no App declares at all.
func TestBuildBundle_AppContextModuleScoping(t *testing.T) {
	r := loadFixture(t)
	entities := func() []EntityDescriptor {
		return []EntityDescriptor{
			{Module: "billing", Name: "order", Spec: orderEntity()},
			{Module: "hr", Name: "employee", Spec: customerEntity()},
		}
	}
	can := func(string) bool { return true }

	t.Run("unscoped (_admin) sees every module", func(t *testing.T) {
		b := r.BuildBundle(entities, can, AppContext{})
		if len(b.Entities) != 2 {
			t.Fatalf("expected entities from both modules, got %+v", b.Entities)
		}
	})

	t.Run("scoped to one module (App) excludes the other", func(t *testing.T) {
		b := r.BuildBundle(entities, can, AppContext{Modules: map[string]bool{"billing": true}})
		if len(b.Entities) != 1 || b.Entities[0].Module != "billing" {
			t.Fatalf("expected only billing entity, got %+v", b.Entities)
		}
	})
}

func TestEntitySchemaDerivation(t *testing.T) {
	schema := BuildEntitySchema(EntityDescriptor{Module: "billing", Name: "order", Spec: orderEntity()})

	if schema.Plural != "orders" {
		t.Errorf("plural: %s", schema.Plural)
	}
	if schema.LabelField != "number" {
		t.Errorf("label_field should prefer natural key, got %s", schema.LabelField)
	}
	if schema.Lifecycle != "two_step_autosave" {
		t.Errorf("default lifecycle: %s", schema.Lifecycle)
	}
	if !schema.Exposed {
		t.Errorf("expected exposed")
	}
	// delete is disabled → not in actions; checkout permission qualified
	for _, a := range schema.Actions {
		if a.Name == "delete" {
			t.Errorf("disabled action leaked into schema")
		}
		if a.Name == "checkout" && a.Permission != "billing.orders.checkout" {
			t.Errorf("checkout permission not qualified: %s", a.Permission)
		}
	}
}

func TestLifecyclePlainCRUD(t *testing.T) {
	es := orderEntity()
	es.Actions = append(es.Actions, spec.Action{Name: "submit", Disabled: true})
	schema := BuildEntitySchema(EntityDescriptor{Module: "billing", Name: "order", Spec: es})
	if schema.Lifecycle != "plain_crud" {
		t.Errorf("submit disabled should yield plain_crud, got %s", schema.Lifecycle)
	}
}

func TestLabelFieldFallbacks(t *testing.T) {
	es := &spec.EntitySpec{Fields: []spec.Field{{Name: "title", Type: spec.FieldString}}}
	if lf := labelField(es); lf != "title" {
		t.Errorf("want title, got %s", lf)
	}
	es2 := &spec.EntitySpec{Fields: []spec.Field{{Name: "x", Type: spec.FieldString}}}
	if lf := labelField(es2); lf != "id" {
		t.Errorf("want id fallback, got %s", lf)
	}
}
