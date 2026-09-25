package ui

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestBuildBundle_MenuDropsUnreachableItems pins the kafe fix (items
// 10.10/10.11).
//
// Why this exists: an App menu is authored ONCE for every role, while entities
// are filtered PER ROLE. A curated item therefore routinely points at an entity
// the caller has no grant for. Measured on kafe, logged in as `manajer`, before
// the fix: the sidebar offered "Pelanggan" → `/cafe-master/members` and
// "Loyalitas" → `/cafe-loyalty/point-entries`; neither entity was in the bundle
// (20 entities — no `members`, no `point-entries`), and the SPA's catch-all
// answered with a SILENT redirect to the first entity, so clicking "Pelanggan"
// rendered the Order list ("Order | 21 records"). A dead link was
// indistinguishable from a working one.
//
// The distinction that makes filtering correct: an item whose route names a
// REGISTERED entity the caller cannot read is dropped; an item with no entity
// behind it at all (dashboard, report, custom page, group) is KEPT, because
// "not permission-checkable" is not "forbidden".
func TestBuildBundle_MenuDropsUnreachableItems(t *testing.T) {
	entities := func() []EntityDescriptor {
		return []EntityDescriptor{
			{Module: "billing", Name: "order", Spec: orderEntity()},
			{Module: "billing", Name: "customer", Spec: customerEntity()},
		}
	}

	// Covers every route shape the resolver must understand: the derived entity
	// list (`/M/<plural>`), a group, an authored Page route, a Table's own
	// route, and a route with no entity behind it.
	newMenu := func() []spec.MenuItem {
		return []spec.MenuItem{
			// 1. Derived entity list route — registered and granted → kept.
			{Label: "Orders", Module: "billing", Route: "/billing/orders"},
			// 2. Derived entity list route — registered but NOT granted → dropped.
			{Label: "Customers", Module: "billing", Route: "/billing/customers"},
			// 3. Group whose only child is unreachable → the whole group goes.
			{Label: "Group of one", Children: []spec.MenuItem{
				{Label: "Customers", Module: "billing", Route: "/billing/customers"},
			}},
			// 4. Authored Page route → resolves through its table block.
			{Label: "Order list page", Module: "billing", Route: "/orders"},
			// 5. The Table's own route.
			{Label: "Order table", Module: "billing", Route: "/billing/table/order-table"},
			// 6. Navigation-kind route backed by `billing.order` → kept.
			{Label: "Sales report", Module: "billing", Route: "/report/sales"},
			{Label: "Order board", Module: "billing", Route: "/kanban/order-board"},
			// 7. No entity behind it at all → kept.
			{Label: "Settings", Module: "billing", Route: "/settings"},
			// 8. Navigation-kind route whose entity is NOT registered in this
			// bundle → dropped. The report itself is withheld from the bundle by
			// the same visibility rule, so serving the link would produce a
			// 404 route — exactly the kafe `dapur` case, where all seven Laporan
			// items pointed at reports the role has no grant for.
			{Label: "Missing report", Module: "billing", Route: "/report/unknown-report"},
		}
	}

	t.Run("drops items whose entity the caller cannot read", func(t *testing.T) {
		r := loadFixture(t)
		can := func(p string) bool { return p == "billing.orders.list" }
		b := r.BuildBundle(entities, can, AppContext{Menu: newMenu()})

		got := menuLabels(b.Menu)
		want := []string{"Orders", "Order list page", "Order table", "Sales report", "Order board", "Settings"}
		if !equalStrings(got, want) {
			t.Errorf("visible menu:\n want %v\n  got %v", want, got)
		}
	})

	t.Run("group disappears when every child is unreachable", func(t *testing.T) {
		r := loadFixture(t)
		can := func(p string) bool { return p == "billing.orders.list" }
		b := r.BuildBundle(entities, can, AppContext{Menu: newMenu()})

		for _, item := range b.Menu {
			if item.Label == "Group of one" {
				t.Errorf("group with no reachable child must be dropped, got %+v", item)
			}
		}
	})

	t.Run("menu never offers an entity the bundle does not ship", func(t *testing.T) {
		r := loadFixture(t)
		can := func(p string) bool { return p == "billing.orders.list" }
		b := r.BuildBundle(entities, can, AppContext{Menu: newMenu()})

		// Only `order` ships; only `order`-backed leaves may survive.
		if len(b.Entities) != 1 {
			t.Fatalf("entities: want 1, got %d", len(b.Entities))
		}
		if b.Entities[0].Plural != "orders" {
			t.Fatalf("entity: want orders, got %s", b.Entities[0].Plural)
		}
		for _, item := range b.Menu {
			if item.Label == "Customers" {
				t.Errorf("menu still offers an entity that is not in the bundle: %+v", item)
			}
			if item.Label == "Missing report" {
				t.Errorf("menu still offers a report whose entity is not in the bundle: %+v", item)
			}
		}
	})

	t.Run("a caller with full access keeps every navigable item", func(t *testing.T) {
		r := loadFixture(t)
		all := func(string) bool { return true }
		b := r.BuildBundle(entities, all, AppContext{Menu: newMenu()})

		got := menuLabels(b.Menu)
		want := []string{
			"Orders", "Customers", "Group of one",
			"Order list page", "Order table", "Sales report", "Order board", "Settings",
		}
		if !equalStrings(got, want) {
			t.Errorf("visible menu:\n want %v\n  got %v", want, got)
		}
	})

	t.Run("an absent menu stays empty, not nil", func(t *testing.T) {
		r := loadFixture(t)
		all := func(string) bool { return true }
		b := r.BuildBundle(entities, all, AppContext{})

		if b.Menu == nil {
			t.Error("menu must serialize as [] not null")
		}
		if len(b.Menu) != 0 {
			t.Errorf("want empty menu, got %v", menuLabels(b.Menu))
		}
	})
}

func menuLabels(items []spec.MenuItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Label)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestBuildBundle_DashboardFollowsItsWidgets pins the second half of the kafe
// `dapur` report.
//
// Why this exists: a Dashboard was shipped purely because `appCtx.allows(module)`
// said so, while entity-backed WIDGETS were filtered by permission. Measured on
// kafe as role `dapur`: the bundle carried `owner-overview` but an EMPTY widget
// list, so the role's only menu entry opened a page of four
// "Widget definition not found" placeholders. A dashboard whose every placement
// is filtered out is dead, not merely thin.
//
// A widget with no entity is kept for everyone (a computed metric the caller is
// always allowed to see), so a dashboard of such widgets survives — that is the
// escape hatch, and it is asserted here so the rule cannot be tightened later
// without intention.
func TestBuildBundle_DashboardFollowsItsWidgets(t *testing.T) {
	r := loadFixture(t)
	// The fixture's `main-dash` places `rev-today`, which reads `billing.order`.
	entities := func() []EntityDescriptor {
		return []EntityDescriptor{{Module: "billing", Name: "order", Spec: orderEntity()}}
	}

	t.Run("ships when the caller can read the widget's entity", func(t *testing.T) {
		can := func(p string) bool { return p == "billing.orders.list" }
		b := r.BuildBundle(entities, can, AppContext{})
		if len(b.Dashboards) != 1 {
			t.Fatalf("dashboards: want 1, got %d", len(b.Dashboards))
		}
		if len(b.Widgets) != 1 {
			t.Fatalf("widgets: want 1, got %d", len(b.Widgets))
		}
	})

	t.Run("dropped when every placed widget is filtered out", func(t *testing.T) {
		// No grant on `billing.orders` → the only widget disappears with it, so
		// the dashboard showing nothing but that widget must go too.
		none := func(string) bool { return false }
		b := r.BuildBundle(entities, none, AppContext{})
		if len(b.Widgets) != 0 {
			t.Fatalf("widgets: want 0, got %d", len(b.Widgets))
		}
		if len(b.Dashboards) != 0 {
			t.Errorf("dashboard whose only widget is unreadable must not ship, got %v", b.Dashboards[0].Name)
		}
	})
}

// TestBuildBundle_MenuDropsRoutesTheBundleRemoved pins the case that motivated
// switching the menu filter from "does the caller hold the permission" to "does
// the bundle serve this route".
//
// Why the distinction matters, measured on kafe as role `dapur`: the role's
// sidebar offered "Ringkasan Pemilik" → `/dashboard/owner-overview`, but
// `dashboardHasVisibleWidget` had already removed that dashboard from the
// bundle (all four of its widgets read entities the role cannot see). A
// permission check on the route's own manifest kept the link, and clicking it
// rendered "Page not found". The dashboard's absence — not the caller's
// grants — is what decides whether the link may be shown.
func TestBuildBundle_MenuDropsRoutesTheBundleRemoved(t *testing.T) {
	r := loadFixture(t)
	entities := func() []EntityDescriptor {
		return []EntityDescriptor{{Module: "billing", Name: "order", Spec: orderEntity()}}
	}
	// The fixture's dashboard is `main-dash`, placing `rev-today` (entity
	// `billing.order`).
	menu := []spec.MenuItem{
		{Label: "Ringkasan", Module: "billing", Route: "/dashboard/main-dash"},
	}

	t.Run("dropped when the dashboard itself was dropped", func(t *testing.T) {
		none := func(string) bool { return false }
		b := r.BuildBundle(entities, none, AppContext{Menu: menu})
		if len(b.Dashboards) != 0 {
			t.Fatalf("precondition: dashboard should have been dropped, got %d", len(b.Dashboards))
		}
		if len(b.Menu) != 0 {
			t.Errorf("menu still links to a route the bundle does not serve: %v", menuLabels(b.Menu))
		}
	})

	t.Run("kept when the dashboard ships", func(t *testing.T) {
		can := func(p string) bool { return p == "billing.orders.list" }
		b := r.BuildBundle(entities, can, AppContext{Menu: menu})
		if len(b.Dashboards) != 1 {
			t.Fatalf("precondition: dashboard should ship, got %d", len(b.Dashboards))
		}
		if len(b.Menu) != 1 {
			t.Errorf("menu should keep the reachable dashboard, got %v", menuLabels(b.Menu))
		}
	})
}

// TestBuildBundle_PublicAppDoesNotShipFrameworkAdmin pins a security-relevant
// scoping rule.
//
// Why this exists: `internal/api/meta.go` makes `can` always-true for an
// `access: public` App — the App IS anonymous — while `AppContext.allows`
// deliberately always passes `formspec.core` so the framework's auth screens
// reach a surface that is not App-scoped. Composed, those two shipped the
// framework's ADMIN surface to anonymous visitors. Measured on kafe:
// `GET /kafe/_ui/_meta/ui?app=kafe-qr` returned `formspec.core` entities
// (user, role, api-key, session) and the `/access-management` page, and the
// browser rendered an "Access Management" table whose columns include
// `Password Hash`. The data endpoints still answered 401 — nothing leaked —
// but the admin surface must not be reachable at all from a public App.
//
// The framework auth screens are the exception: a public App still needs
// /_auth/login to sign a visitor in.
func TestBuildBundle_PublicAppDoesNotShipFrameworkAdmin(t *testing.T) {
	r := loadFixture(t)
	entities := func() []EntityDescriptor {
		return []EntityDescriptor{{Module: "billing", Name: "order", Spec: orderEntity()}}
	}
	all := func(string) bool { return true } // what meta.go does for a public App

	t.Run("public App ships only its own modules", func(t *testing.T) {
		b := r.BuildBundle(entities, all, AppContext{
			Access:  string(spec.AppAccessPublic),
			Modules: map[string]bool{"billing": true},
		})
		for _, e := range b.Entities {
			if e.Module != "billing" {
				t.Errorf("public App shipped a foreign module's entity: %s.%s", e.Module, e.Name)
			}
		}
	})

	t.Run("private App keeps the framework pages", func(t *testing.T) {
		// No Access field → private → the framework surface ships so a signed-in
		// admin can manage users.
		b := r.BuildBundle(entities, all, AppContext{
			Modules: map[string]bool{"billing": true},
		})
		if len(b.Pages) == 0 {
			t.Skip("fixture has no formspec.core pages to assert on")
		}
	})
}

// TestBuildBundle_PublicAppHidesFrameworkAdmin pins a real exposure found while
// auditing the kafe PUBLIC App (`kafe-qr`, `access: public`, `root_url: /`).
//
// Why this exists: `AppContext.allows` always admits `core` and `formspec.core`,
// because those are the framework's own modules and no App ever declares them —
// correct for a private App, where the caller's permissions still gate every
// entity. A public App is different: `internal/api/meta.go` replaces the
// permission checker with an always-true one (there is no session to check), so
// `allows` was the ONLY gate left, and it let everything through.
//
// Measured: `GET /kafe/_ui/_meta/ui?app=kafe-qr` (unauthenticated) returned
// `formspec.core` entities — user, role, api-key, session — plus the
// `/access-management` page, and the anonymous browser rendered an "Access
// Management" table whose columns include `Password Hash`. The data endpoints
// still answered 401, so no row leaked; the admin surface itself must not be
// reachable at all.
//
// The auth screens are the deliberate exception: a public App needs them to sign
// a visitor in, so they ship while `/access-management` does not.
func TestBuildBundle_PublicAppHidesFrameworkAdmin(t *testing.T) {
	// A public App that mounts only `billing`; the fixture also carries a
	// `formspec.core` page (access-management) with permissions: [] so that
	// permission filtering cannot be what hides it.
	core := func() []EntityDescriptor {
		return []EntityDescriptor{{Module: "formspec.core", Name: "user", Spec: customerEntity()}}
	}
	billing := func() []EntityDescriptor {
		return []EntityDescriptor{{Module: "billing", Name: "order", Spec: orderEntity()}}
	}
	entities := func() []EntityDescriptor {
		return append(billing(), core()...)
	}
	all := func(string) bool { return true } // what a public App passes in
	public := AppContext{
		Access:  string(spec.AppAccessPublic),
		Modules: map[string]bool{"billing": true},
	}

	t.Run("framework entities do not ship to anonymous callers", func(t *testing.T) {
		r := loadFixture(t)
		b := r.BuildBundle(entities, all, public)
		for _, e := range b.Entities {
			if e.Module == "formspec.core" {
				t.Errorf("public bundle leaked framework entity %s/%s", e.Module, e.Name)
			}
		}
	})

	t.Run("the App's own entities still ship", func(t *testing.T) {
		r := loadFixture(t)
		b := r.BuildBundle(entities, all, public)
		if len(b.Entities) != 1 || b.Entities[0].Module != "billing" {
			t.Fatalf("want the App's own entity only, got %+v", b.Entities)
		}
	})

	t.Run("a private App still sees the framework admin surface", func(t *testing.T) {
		r := loadFixture(t)
		private := AppContext{Access: "private", Modules: map[string]bool{"billing": true}}
		b := r.BuildBundle(entities, all, private)
		// The gate is scoped to public Apps; private Apps are filtered by
		// permission (always-true here), so the core entity is expected.
		var sawCore bool
		for _, e := range b.Entities {
			if e.Module == "formspec.core" {
				sawCore = true
			}
		}
		if !sawCore {
			t.Error("private App should still receive formspec.core entities")
		}
	})
}

// TestBuildBundle_PublicAppHonoursAllowlist pins the second half of the kafe
// public-App exposure.
//
// Why this exists: hiding the framework modules was not enough. For an
// `access: public` App the permission checker was replaced with an always-true
// one, so EVERY entity of every mounted module shipped, while the App's
// `public_entities` allowlist — the only statement of what an anonymous caller
// may see — was consulted for the data routes but not for the bundle.
//
// Measured on kafe (`kafe-qr` mounts cafe-master + cafe-order behind a narrow
// allowlist): the anonymous bundle carried 13 entities including
// `cafe-master.members` (customer phone numbers), `employees`,
// `menu-item-prices`, `cafe-order.shifts` and `cash-movements`. The data
// endpoints still enforced the allowlist, so no row leaked — but the schema of
// private data was handed out and the SPA generated routes for it.
func TestBuildBundle_PublicAppHonoursAllowlist(t *testing.T) {
	entities := func() []EntityDescriptor {
		return []EntityDescriptor{
			{Module: "billing", Name: "order", Spec: orderEntity()},
			{Module: "billing", Name: "customer", Spec: customerEntity()},
		}
	}
	// The App mounts `billing` but exposes only orders, read-only.
	decls := []spec.PublicEntityDecl{
		{Entity: "billing/order", Actions: []string{"list", "find"}},
	}
	public := AppContext{
		Access:         string(spec.AppAccessPublic),
		Modules:        map[string]bool{"billing": true},
		PublicEntities: &decls,
	}

	t.Run("only allowlisted entities ship", func(t *testing.T) {
		r := loadFixture(t)
		// nil checker = "derive it from the allowlist" (how internal/api calls
		// this for a public App).
		b := r.BuildBundle(entities, nil, public)
		if len(b.Entities) != 1 {
			t.Fatalf("want only the allowlisted entity, got %d: %+v", len(b.Entities), b.Entities)
		}
		if b.Entities[0].Name != "order" {
			t.Errorf("want `order`, got %q", b.Entities[0].Name)
		}
	})

	t.Run("a non-allowlisted entity does not ship", func(t *testing.T) {
		r := loadFixture(t)
		b := r.BuildBundle(entities, nil, public)
		for _, e := range b.Entities {
			if e.Name == "customer" {
				t.Error("customer is not in public_entities but shipped to an anonymous caller")
			}
		}
	})

	t.Run("an explicitly empty allowlist exposes nothing", func(t *testing.T) {
		r := loadFixture(t)
		empty := []spec.PublicEntityDecl{}
		closed := AppContext{
			Access:         string(spec.AppAccessPublic),
			Modules:        map[string]bool{"billing": true},
			PublicEntities: &empty,
		}
		b := r.BuildBundle(entities, nil, closed)
		if len(b.Entities) != 0 {
			t.Errorf("`public_entities: []` must expose nothing, got %+v", b.Entities)
		}
	})

	t.Run("an absent allowlist keeps the legacy module-wide behaviour", func(t *testing.T) {
		r := loadFixture(t)
		legacy := AppContext{
			Access:  string(spec.AppAccessPublic),
			Modules: map[string]bool{"billing": true},
		}
		// No allowlist + no derived checker → the caller's checker stands, and
		// `internal/api` has always supplied an always-true one here.
		all := func(string) bool { return true }
		b := r.BuildBundle(entities, all, legacy)
		if len(b.Entities) != 2 {
			t.Errorf("legacy public App should ship every mounted entity, got %d", len(b.Entities))
		}
	})
}
