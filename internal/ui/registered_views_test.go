package ui

import (
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// The App surface allowlist (plan docs_internal/plan/registered-views.md):
// an App's reachable surface is "every menu leaf target ∪ registered_views".
// Anything outside it ships WITHOUT a derived route (`routable: false`) so a
// direct URL answers 404, while the entity itself still ships so relations and
// pickers keep resolving.

// registeredFixtureYAML carries the kinds whose derived routes the tests probe:
// a Table (its `<module>/table/<name>` wrapper) and an authored Page.
const registeredFixtureYAML = `
apiVersion: formspec.dev/v1alpha1
kind: Table
metadata: { name: sheet-category-table, module: portal }
spec:
  entity: category
  columns:
    - { field: name }
---
apiVersion: formspec.dev/v1alpha1
kind: Page
metadata: { name: sheet-page, module: portal }
spec:
  route: /portal/sheet-page
  title: Sheet Page
`

func loadRegisteredFixture(t *testing.T) *Registry {
	t.Helper()
	loader := manifest.NewLoader("")
	raws, errs := loader.ParseBytes([]byte(registeredFixtureYAML), "registered_fixture.yaml")
	if len(errs) > 0 {
		t.Fatalf("parse fixture: %v", errs)
	}
	r := NewRegistry()
	if loadErrs := r.Load(raws); len(loadErrs) > 0 {
		t.Fatalf("load fixture: %v", loadErrs)
	}
	return r
}

func pluralEntity(plural string) *spec.EntitySpec {
	return &spec.EntitySpec{
		Plural: plural,
		Fields: []spec.Field{{Name: "name", Type: spec.FieldString}},
	}
}

// surfaceEntities is the module set the surface tests gate: two explicitly
// mounted modules, four entities — only some of which the App exposes.
func surfaceEntities() func() []EntityDescriptor {
	return func() []EntityDescriptor {
		return []EntityDescriptor{
			{Module: "portal", Name: "category", Spec: pluralEntity("categories")},
			{Module: "portal", Name: "item", Spec: pluralEntity("items")},
			{Module: "store", Name: "product", Spec: pluralEntity("products")},
			{Module: "store", Name: "warehouse", Spec: pluralEntity("warehouses")},
		}
	}
}

func routableByRef(t *testing.T, b *Bundle, ref string) bool {
	t.Helper()
	for _, e := range b.Entities {
		if e.Module+"/"+e.Name == ref {
			return e.Routable
		}
	}
	t.Fatalf("entity %s not in bundle", ref)
	return false
}

func pageExists(b *Bundle, route string) bool {
	for _, p := range b.Pages {
		if p.Spec != nil && p.Spec.Route == route {
			return true
		}
	}
	return false
}

func TestBuildBundle_SurfaceGate_UnionOfMenuAndRegisteredViews(t *testing.T) {
	all := func(string) bool { return true }
	ctx := AppContext{
		Modules: map[string]bool{"portal": true, "store": true},
		Menu: []spec.MenuItem{
			// A raw `route:` target registers the entity it points at.
			{Label: "Products", Module: "store", Route: "/store/products"},
		},
		RegisteredViews: []spec.RegisteredViewDecl{
			// A `{view:}` entry registers one kind's derived route.
			{View: "portal/sheet-category-table"},
			// A `{entity:}` entry registers every derived view of the entity.
			{Entity: "portal/item"},
		},
	}
	r := loadRegisteredFixture(t)
	b := r.BuildBundle(surfaceEntities(), all, ctx)

	if !routableByRef(t, b, "store/product") {
		t.Errorf("menu route target must be routable")
	}
	if !routableByRef(t, b, "portal/item") {
		t.Errorf("registered_views entity must be routable")
	}
	if routableByRef(t, b, "portal/category") {
		t.Errorf("entity in neither menu nor registered_views must NOT be routable")
	}
	if routableByRef(t, b, "store/warehouse") {
		t.Errorf("unexposed entity must NOT be routable")
	}

	// The registered `{view:}` produced its derived Table wrapper page.
	if !pageExists(b, "/portal/table/sheet-category-table") {
		t.Errorf("registered view must produce its derived page route")
	}
	// The authored page is not registered → its route must not ship.
	if pageExists(b, "/portal/sheet-page") {
		t.Errorf("unregistered authored page must not ship")
	}
}

func TestBuildBundle_SurfaceGate_DerivedWrapperNeedsRegistration(t *testing.T) {
	all := func(string) bool { return true }
	base := AppContext{Modules: map[string]bool{"portal": true, "store": true}}
	r := loadRegisteredFixture(t)

	// Without the registered view, the Table gets no derived page.
	b := r.BuildBundle(surfaceEntities(), all, base)
	if pageExists(b, "/portal/table/sheet-category-table") {
		t.Errorf("derived Table page must not ship when its route is not registered")
	}
}

func TestBuildBundle_SurfaceGate_RouteExistsHonoursRoutable(t *testing.T) {
	all := func(string) bool { return true }
	ctx := AppContext{
		Modules: map[string]bool{"portal": true, "store": true},
		Menu: []spec.MenuItem{
			{Label: "Products", Module: "store", Route: "/store/products"},
		},
	}
	r := loadRegisteredFixture(t)
	b := r.BuildBundle(surfaceEntities(), all, ctx)

	if !r.routeExists("store", "/store/products", b) {
		t.Errorf("routable entity route must exist")
	}
	if r.routeExists("portal", "/portal/categories", b) {
		t.Errorf("non-routable entity route must NOT exist")
	}
}

func TestBuildBundle_SurfaceGate_InactiveWithoutAppModules(t *testing.T) {
	all := func(string) bool { return true }
	// A zero AppContext (Modules nil) — the `_admin` surface — is not gated.
	r := loadRegisteredFixture(t)
	b := r.BuildBundle(surfaceEntities(), all, AppContext{})

	for _, e := range b.Entities {
		if !e.Routable {
			t.Errorf("without an App scope every entity stays routable, got %s/%s non-routable", e.Module, e.Name)
		}
	}
	if !pageExists(b, "/portal/sheet-page") {
		t.Errorf("without an App scope authored pages ship")
	}
}

func TestBuildBundle_SurfaceGate_UnfilteredBypass(t *testing.T) {
	all := func(string) bool { return true }
	// The grants editor (`?grants=true`) must see every view regardless of the
	// surface allowlist, so an admin can grant access to an unregistered one.
	r := loadRegisteredFixture(t)
	b := r.BuildBundle(surfaceEntities(), all, AppContext{
		Modules:    map[string]bool{"portal": true, "store": true},
		Unfiltered: true,
	})

	for _, e := range b.Entities {
		if !e.Routable {
			t.Errorf("Unfiltered must keep every entity routable, got %s/%s", e.Module, e.Name)
		}
	}
}

func TestBuildBundle_SurfaceGate_MenuRegistersItsTargets(t *testing.T) {
	all := func(string) bool { return true }
	// A menu leaf IS a declaration: its target becomes reachable (union), so the
	// item stays. A leaf pointing at a route nothing backs is still dropped by
	// routeExists — the surface gate does not resurrect dead links.
	ctx := AppContext{
		Modules: map[string]bool{"portal": true, "store": true},
		Menu: []spec.MenuItem{
			{Label: "Categories", Module: "portal", Route: "/portal/categories"},
			{Label: "Ghost page", Module: "portal", Route: "/portal/table/nope"},
		},
	}
	r := loadRegisteredFixture(t)
	b := r.BuildBundle(surfaceEntities(), all, ctx)

	if !routableByRef(t, b, "portal/category") {
		t.Errorf("a menu target must make its entity routable")
	}
	var keptCategories, keptGhost bool
	for _, m := range b.Menu {
		switch m.Label {
		case "Categories":
			keptCategories = true
		case "Ghost page":
			keptGhost = true
		}
	}
	if !keptCategories {
		t.Errorf("a menu item whose entity it itself declares must be kept")
	}
	if keptGhost {
		t.Errorf("a menu item pointing at a route nothing backs must be dropped, got %+v", b.Menu)
	}
}
