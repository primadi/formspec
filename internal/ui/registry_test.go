package ui

import (
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// ─── ResolveViewRoute — the kind → route table ───
//
// Why this file exists: ResolveViewRoute is one of FOUR copies of the same
// routing convention — together with the client's buildRoutes
// (renderers/react-shadcn/src/shell/router.tsx), routeExists/navigationPrefix
// (meta.go), and viewKinds (cmd/formspec/validate_dangling.go). Nothing pinned
// the table, so the copies drifted exactly once, and the drift was invisible:
// `Listing` was missing HERE only, which meant `formspec validate` accepted
// `view: product-catalog` (Listing is in viewKinds) while app.Resolve returned
// "view not found" — an App that validates green and mounts nothing. The live
// storefront example only escaped it by using the `route:` escape hatch.
//
// The table below is deliberately exhaustive per kind: adding a navigable kind
// must be a conscious edit in all four places, and this test is the one that
// fails loudly when it is not.

// menusFixtureYAML is a SEPARATE fixture from fixtureYAML in ui_test.go.
//
// Why separate: TestLoadAllKinds asserts an exact manifest count (12), so
// covering ResolveViewRoute's whole table by extending the shared fixture would
// break an unrelated assertion. This one carries the kinds the shared fixture
// lacks — Listing, Calendar, ApprovalInbox, NotificationCenter — all in module
// `catalog`, plus a second module so the module-scoping branch has something to
// reject.
const menusFixtureYAML = `
apiVersion: formspec.dev/v1alpha1
kind: Page
metadata: { name: catalog-home, module: catalog }
spec:
  route: /catalog
  title: Catalog Home
---
apiVersion: formspec.dev/v1alpha1
kind: Form
metadata: { name: catalog-form, module: catalog }
spec:
  entity: shop.product
  sections:
    - { title: Main, fields: [{ field: name }] }
---
apiVersion: formspec.dev/v1alpha1
kind: Table
metadata: { name: catalog-table, module: catalog }
spec:
  entity: shop.product
  columns:
    - { field: name }
---
apiVersion: formspec.dev/v1alpha1
kind: Listing
metadata: { name: product-catalog, module: catalog }
spec:
  entity: shop.product
  search: true
---
apiVersion: formspec.dev/v1alpha1
kind: Calendar
metadata: { name: visit-calendar, module: catalog }
spec:
  entity: shop.product
  date_field: created_at
---
apiVersion: formspec.dev/v1alpha1
kind: ApprovalInbox
metadata: { name: approval-inbox, module: catalog }
spec:
  search: true
---
apiVersion: formspec.dev/v1alpha1
kind: NotificationCenter
metadata: { name: notification-center, module: catalog }
spec:
  realtime: true
---
apiVersion: formspec.dev/v1alpha1
kind: Widget
metadata: { name: catalog-metric, module: catalog }
spec:
  title: Metric
  type: metric
  entity: shop.product
---
apiVersion: formspec.dev/v1alpha1
kind: Dashboard
metadata: { name: catalog-dash, module: catalog }
spec:
  title: Catalog
  widgets:
    - { ref: catalog-metric, layout: { x: 0, y: 0, w: 1, h: 1 } }
---
apiVersion: formspec.dev/v1alpha1
kind: Report
metadata: { name: catalog-report, module: catalog }
spec:
  title: Report
  entity: shop.product
  columns:
    - { field: name }
---
apiVersion: formspec.dev/v1alpha1
kind: Wizard
metadata: { name: catalog-wizard, module: catalog }
spec:
  title: Wizard
  entity: shop.product
  steps:
    - { title: One }
---
apiVersion: formspec.dev/v1alpha1
kind: Kanban
metadata: { name: catalog-kanban, module: catalog }
spec:
  entity: shop.product
  status_field: status
  columns:
    - { status: draft, label: Draft }
---
apiVersion: formspec.dev/v1alpha1
kind: Timeline
metadata: { name: catalog-timeline, module: catalog }
spec:
  entity: shop.product
  date_field: created_at
---
apiVersion: formspec.dev/v1alpha1
kind: Print
metadata: { name: catalog-print, module: catalog }
spec:
  entity: shop.product
  output: { format: thermal, paper: { size: thermal_58mm } }
  body:
    - fields: [name]
---
apiVersion: formspec.dev/v1alpha1
kind: Theme
metadata: { name: catalog-theme, module: catalog }
spec:
  tokens:
    color.primary: "#111"
`

func loadMenusFixture(t *testing.T) *Registry {
	t.Helper()
	loader := manifest.NewLoader("")
	raws, errs := loader.ParseBytes([]byte(menusFixtureYAML), "menus-fixture.yaml")
	if len(errs) > 0 {
		t.Fatalf("parse menus fixture: %v", errs)
	}
	r := NewRegistry()
	if loadErrs := r.Load(raws); len(loadErrs) > 0 {
		t.Fatalf("load menus fixture: %v", loadErrs)
	}
	return r
}

// TestResolveViewRoute pins every kind's route shape, including the two kinds
// that carry their route in the manifest rather than following the convention
// (Page → `spec.route`) and the two that use a module-scoped prefix
// (Form/Table → `/<module>/form|table/<name>`).
func TestResolveViewRoute(t *testing.T) {
	r := loadMenusFixture(t)

	cases := []struct {
		kind string
		name string
		want string
	}{
		// Route comes from the manifest itself, not from a convention.
		{"Page", "catalog-home", "/catalog"},
		// Module-scoped prefixes — the shape the server also generates as a
		// derived Page wrapper (makeDerivedPage) and the client must register.
		{"Form", "catalog-form", "/catalog/form/catalog-form"},
		{"Table", "catalog-table", "/catalog/table/catalog-table"},
		// `/<kind-lowercase>/<name>` conventions. Listing is the case that was
		// missing and let a validating App fail to resolve.
		{"Listing", "product-catalog", "/listing/product-catalog"},
		{"Dashboard", "catalog-dash", "/dashboard/catalog-dash"},
		{"Widget", "catalog-metric", "/widget/catalog-metric"},
		{"Report", "catalog-report", "/report/catalog-report"},
		{"Wizard", "catalog-wizard", "/wizard/catalog-wizard"},
		{"Kanban", "catalog-kanban", "/kanban/catalog-kanban"},
		{"Timeline", "catalog-timeline", "/timeline/catalog-timeline"},
		{"Calendar", "visit-calendar", "/calendar/visit-calendar"},
		{"ApprovalInbox", "approval-inbox", "/approval-inbox/approval-inbox"},
		{"NotificationCenter", "notification-center", "/notification-center/notification-center"},
		{"Print", "catalog-print", "/print/catalog-print"},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			got, err := r.ResolveViewRoute("catalog", tc.name)
			if err != nil {
				t.Fatalf("ResolveViewRoute(%q) error: %v", tc.name, err)
			}
			if got != tc.want {
				t.Errorf("ResolveViewRoute(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestResolveViewRoute_CoversEveryNavigableViewKind is the anti-drift guard: it
// asserts that every kind the menu may point at is actually reachable through
// ResolveViewRoute.
//
// It works by trying the kinds by name and requiring none to fail — if a kind
// is registered in the registry (and allowed by viewKinds in
// cmd/formspec/validate_dangling.go) but has no branch here, this fails. That
// is the exact Listing bug, expressed as a test instead of a comment.
func TestResolveViewRoute_CoversEveryNavigableViewKind(t *testing.T) {
	r := loadMenusFixture(t)

	// One registered name per navigable kind, mirroring
	// cmd/formspec/validate_dangling.go `viewKinds`.
	navigable := []struct {
		kind string
		name string
	}{
		{"Page", "catalog-home"},
		{"Form", "catalog-form"},
		{"Table", "catalog-table"},
		{"Listing", "product-catalog"},
		{"Dashboard", "catalog-dash"},
		{"Widget", "catalog-metric"},
		{"Report", "catalog-report"},
		{"Wizard", "catalog-wizard"},
		{"Kanban", "catalog-kanban"},
		{"Timeline", "catalog-timeline"},
		{"Calendar", "visit-calendar"},
		{"ApprovalInbox", "approval-inbox"},
		{"NotificationCenter", "notification-center"},
		{"Print", "catalog-print"},
	}

	for _, tc := range navigable {
		if _, err := r.ResolveViewRoute("catalog", tc.name); err != nil {
			t.Errorf("kind %s is offered as a menu `view` target (viewKinds) but ResolveViewRoute cannot resolve %q: %v",
				tc.kind, tc.name, err)
		}
	}
}

func TestResolveViewRoute_ModuleScoped(t *testing.T) {
	r := loadMenusFixture(t)

	// Same name, wrong module → the entry exists but belongs to another module.
	// This is why the module check is not redundant with the map lookup.
	if got, err := r.ResolveViewRoute("other", "catalog-home"); err == nil {
		t.Errorf("expected an error for a wrong module, got %q", got)
	}
}

func TestResolveViewRoute_UnknownName(t *testing.T) {
	r := loadMenusFixture(t)

	if got, err := r.ResolveViewRoute("catalog", "does-not-exist"); err == nil {
		t.Errorf("expected an error for an unknown view, got %q", got)
	}
}

// TestResolveViewRoute_FormAndTableAreNavigable documents a contract point that
// two documents state differently: docs/spec/platform/02-workspace-app-module.md
// §4 says Form and Table are NOT valid `view` targets, while
// docs/kind/curation/App.md says they are, via the auto-derived Page wrapper.
// The code follows the latter, and internal/auth/materialize.go depends on it
// (`{entity}-page` footprint). Pinned here so the contradiction is settled by a
// test rather than by whichever document a reader happens to open.
func TestResolveViewRoute_FormAndTableAreNavigable(t *testing.T) {
	r := loadMenusFixture(t)

	for _, tc := range []struct{ kind, name, want string }{
		{"Form", "catalog-form", "/catalog/form/catalog-form"},
		{"Table", "catalog-table", "/catalog/table/catalog-table"},
	} {
		got, err := r.ResolveViewRoute("catalog", tc.name)
		if err != nil {
			t.Fatalf("%s: expected navigable, got error: %v", tc.kind, err)
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.kind, got, tc.want)
		}
	}
}

// TestResolveViewRoute_ListingResolvesSoValidateAgrees closes the loop on the
// original bug from the other side: `formspec validate` accepts a Listing as a
// menu `view` (viewKinds), so resolution must succeed — otherwise the two
// layers disagree and the App silently fails to mount.
func TestResolveViewRoute_ListingResolvesSoValidateAgrees(t *testing.T) {
	r := loadMenusFixture(t)

	got, err := r.ResolveViewRoute("catalog", "product-catalog")
	if err != nil {
		t.Fatalf("Listing is accepted by validate's viewKinds but ResolveViewRoute failed: %v", err)
	}
	// Must match the client's buildRoutes prefix for the same kind
	// (renderers/react-shadcn/src/shell/router.tsx §10).
	if want := "/listing/product-catalog"; got != want {
		t.Errorf("route %q does not match the client's /listing/<name> convention (%q)", got, want)
	}
}

// TestViewKindsMatchResolveViewRoute is the cross-layer parity check that would
// have caught the original bug at its source: every kind the validator accepts
// as a `view` target must be resolvable here.
//
// The list is duplicated from cmd/formspec/validate_dangling.go `viewKinds`
// deliberately — this test's whole purpose is to fail when the two drift, so it
// must not import the other list and inherit its mistake.
func TestViewKindsMatchResolveViewRoute(t *testing.T) {
	validatorViewKinds := []spec.Kind{
		spec.KindPage, spec.KindForm, spec.KindTable, spec.KindWizard,
		spec.KindReport, spec.KindKanban, spec.KindTimeline, spec.KindCalendar,
		spec.KindDashboard, spec.KindListing,
	}

	// Kind → the module/name of an instance registered in the fixture.
	instance := map[spec.Kind][2]string{
		spec.KindPage:      {"catalog", "catalog-home"},
		spec.KindForm:      {"catalog", "catalog-form"},
		spec.KindTable:     {"catalog", "catalog-table"},
		spec.KindWizard:    {"catalog", "catalog-wizard"},
		spec.KindReport:    {"catalog", "catalog-report"},
		spec.KindKanban:    {"catalog", "catalog-kanban"},
		spec.KindTimeline:  {"catalog", "catalog-timeline"},
		spec.KindCalendar:  {"catalog", "visit-calendar"},
		spec.KindDashboard: {"catalog", "catalog-dash"},
		spec.KindListing:   {"catalog", "product-catalog"},
	}

	r := loadMenusFixture(t)
	for _, kind := range validatorViewKinds {
		ref, ok := instance[kind]
		if !ok {
			t.Fatalf("kind %s is in viewKinds but has no fixture instance — add one so the parity check covers it", kind)
		}
		if _, err := r.ResolveViewRoute(ref[0], ref[1]); err != nil {
			t.Errorf("%s is accepted as a menu `view` by formspec validate but ResolveViewRoute cannot resolve it: %v",
				kind, err)
		}
	}
}

// TestFilterMenuHonoursItemPermissions pins the RBAC axis of menu visibility.
//
// Before this, `Menu.permissions` was read by the CLIENT only
// (hooks/useResolvedMenu.ts) — and the field did not even exist in pkg/spec, so
// the check could never match anything. RBAC in a renderer is bypassable by
// construction, and worse, it drifts from the server silently. It is enforced
// here now, with the same checker that decides which entities ship.
func TestFilterMenuHonoursItemPermissions(t *testing.T) {
	entities := func() []EntityDescriptor {
		return []EntityDescriptor{
			{Module: "billing", Name: "order", Spec: orderEntity()},
		}
	}

	menu := []spec.MenuItem{
		{Label: "Orders", Module: "billing", Route: "/billing/orders"},
		{
			Label:       "Settings",
			Module:      "billing",
			Route:       "/settings",
			Permissions: []string{"billing.settings.update"},
		},
		{
			Label:       "Reports",
			Module:      "billing",
			Route:       "/report/sales",
			Permissions: []string{"billing.reports.view", "billing.reports.own"},
		},
	}

	t.Run("caller without the permission does not receive the item", func(t *testing.T) {
		r := loadFixture(t)
		can := func(p string) bool { return p == "billing.orders.list" }
		b := r.BuildBundle(entities, can, AppContext{Menu: menu})

		got := menuLabels(b.Menu)
		want := []string{"Orders"}
		if !equalStrings(got, want) {
			t.Errorf("visible menu:\n want %v\n  got %v", want, got)
		}
	})

	t.Run("any-of: one matching permission is enough", func(t *testing.T) {
		r := loadFixture(t)
		// `billing.orders.list` is needed for the report to ship at all (the
		// bundle filters entity-backed kinds by the entity's read permission, so
		// without it the route does not exist and the item would be dropped for
		// a different reason than the one under test).
		can := func(p string) bool {
			return p == "billing.orders.list" || p == "billing.reports.own"
		}
		b := r.BuildBundle(entities, can, AppContext{Menu: menu})

		got := menuLabels(b.Menu)
		want := []string{"Orders", "Reports"}
		if !equalStrings(got, want) {
			t.Errorf("visible menu:\n want %v\n  got %v", want, got)
		}
	})

	t.Run("always-visible checker keeps every item (the ?grants bundle)", func(t *testing.T) {
		// The grants editor must offer everything so an admin can grant
		// permissions they do not personally hold — internal/api/meta.go passes
		// exactly this checker for ?admin=true and ?grants=true.
		r := loadFixture(t)
		always := func(string) bool { return true }
		b := r.BuildBundle(entities, always, AppContext{Menu: menu})

		got := menuLabels(b.Menu)
		want := []string{"Orders", "Settings", "Reports"}
		if !equalStrings(got, want) {
			t.Errorf("visible menu:\n want %v\n  got %v", want, got)
		}
	})

	t.Run("group disappears when its only gated child is withheld", func(t *testing.T) {
		r := loadFixture(t)
		grouped := []spec.MenuItem{
			{
				Label: "Admin",
				Children: []spec.MenuItem{
					{
						Label:       "Settings",
						Module:      "billing",
						Route:       "/settings",
						Permissions: []string{"billing.settings.update"},
					},
				},
			},
		}
		can := func(string) bool { return false }
		b := r.BuildBundle(entities, can, AppContext{Menu: grouped})

		if len(b.Menu) != 0 {
			t.Errorf("expected the group to disappear with its only child, got %v", menuLabels(b.Menu))
		}
	})
}
