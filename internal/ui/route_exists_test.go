package ui

import (
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// routeExistsFixture has TWO tables for the same entity, and an authored Page
// that references only one of them.
//
// That asymmetry is the whole point: BuildBundle derives a `<name>-page`
// wrapper for a Table only when no authored Page already references it
// (`covered[...]`). So `covered-table` gets NO derived page — the SPA therefore
// has no route for `/billing/table/covered-table` — while `free-table` does.
const routeExistsFixture = `
apiVersion: formspec.dev/v1alpha1
kind: Table
metadata: { name: covered-table, module: billing }
spec:
  entity: order
  columns: [{ field: number }]
---
apiVersion: formspec.dev/v1alpha1
kind: Table
metadata: { name: free-table, module: billing }
spec:
  entity: order
  columns: [{ field: number }]
---
apiVersion: formspec.dev/v1alpha1
kind: Page
metadata: { name: order-list, module: billing }
spec:
  route: /orders
  title: Orders
  permissions: [orders.list]
  blocks:
    - table: { ref: covered-table, entity: order }
`

// loadRouteExistsFixture loads the two-table fixture above.
func loadRouteExistsFixture(t *testing.T) *Registry {
	t.Helper()
	loader := manifest.NewLoader("")
	raws, errs := loader.ParseBytes([]byte(routeExistsFixture), "routes.yaml")
	if len(errs) > 0 {
		t.Fatalf("parse fixture: %v", errs)
	}
	r := NewRegistry()
	if loadErrs := r.Load(raws); len(loadErrs) > 0 {
		t.Fatalf("load fixture: %v", loadErrs)
	}
	return r
}

// TestRouteExists_FormTableFollowBundlePages pins todo 5.22.6.
//
// The bug: `routeExists` asked the Form/Table REGISTRY whether `/M/table/<n>`
// exists, while the SPA registers routes from `bundle.pages`. Those disagree
// whenever a Table is already referenced by a Page block (or is `public: false`):
// the Table is present in `b.Tables`, so the menu kept the item, but no derived
// page exists, so the click hit the surface catch-all. A menu link that looks
// alive and answers 404 is worse than one that is visibly absent.
//
// The fix asks `b.Pages` instead, which is exactly what `buildRoutes`
// (renderers/react-shadcn/src/shell/router.tsx) iterates.
func TestRouteExists_FormTableFollowBundlePages(t *testing.T) {
	lister := func() []EntityDescriptor {
		return []EntityDescriptor{
			{Module: "billing", Name: "order", Spec: orderEntity()},
		}
	}

	r := loadRouteExistsFixture(t)
	can := func(p string) bool { return true }
	b := r.BuildBundle(lister, can, AppContext{})

	// Precondition: the Table registry knows BOTH tables, but the bundle only
	// derived a page for the uncovered one. If this ever stops holding, the test
	// below would pass for the wrong reason.
	if _, ok := r.Tables["covered-table"]; !ok {
		t.Fatal("covered-table must be loaded for this test to mean anything")
	}
	derived := map[string]bool{}
	for _, e := range b.Pages {
		if e.Spec != nil {
			derived[e.Spec.Route] = true
		}
	}
	if derived["/billing/table/covered-table"] {
		t.Fatal("covered-table must NOT get a derived page (the authored Page references it)")
	}
	if !derived["/billing/table/free-table"] {
		t.Fatal("free-table must get a derived page — no authored Page references it")
	}

	// The assertion under test.
	if r.routeExists("billing", "/billing/table/covered-table", b) {
		t.Error("routeExists(covered-table) = true, but nothing serves that route — " +
			"a menu item pointing here would 404")
	}
	if !r.routeExists("billing", "/billing/table/free-table", b) {
		t.Error("routeExists(free-table) = false, but its derived page exists")
	}

	// A menu item aimed at the suppressed route must be dropped, not served.
	menu := []spec.MenuItem{
		{Label: "Covered", Module: "billing", Route: "/billing/table/covered-table"},
		{Label: "Free", Module: "billing", Route: "/billing/table/free-table"},
	}
	b2 := r.BuildBundle(lister, can, AppContext{Menu: menu})
	labels := menuLabels(b2.Menu)
	if contains(labels, "Covered") {
		t.Errorf("menu kept a dead link to the suppressed table route: %v", labels)
	}
	if !contains(labels, "Free") {
		t.Errorf("menu dropped a live table route: %v", labels)
	}
}

func contains(items []string, want string) bool {
	for _, s := range items {
		if s == want {
			return true
		}
	}
	return false
}
