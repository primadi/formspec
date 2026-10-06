package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// The anonymous allowlist used to be declared in the manifest
// (`App.spec.public_entities`). It is now DERIVED from the App's surface (plan
// docs_internal/plan/implicit-public-grants.md), so these tests drive the real
// kafe tree through the router builder and assert what an anonymous caller may
// reach.
//
// Why the kafe tree rather than a synthetic fixture: derivation walks Pages,
// Forms, Table blocks and child-field pickers, so a hand-built fixture would
// have to reproduce all of that — and could pass while the real manifests expose
// something else. The example is the contract.

// kafePublicRouter builds a RouterBuilder over examples/kafe/spec with the
// resolved Apps wired, so `publicGrants`/`publicScope` derive from the real
// surface.
func kafePublicRouter(t *testing.T) *RouterBuilder {
	t.Helper()
	const specPath = "../../examples/kafe/spec"

	d, err := db.OpenSQLite(filepath.Join(t.TempDir(), "kafe_public.db"), nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, specPath)
	if err := reg.LoadEntities(); err != nil {
		t.Fatalf("load kafe entities: %v", err)
	}

	uiReg := ui.NewRegistry()
	if errs := uiReg.LoadDir(specPath); len(errs) > 0 {
		t.Fatalf("load kafe UI manifests: %v", errs)
	}

	loaded, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load kafe spec tree: %v", err)
	}
	resolved, err := formspec_app.Resolve(loaded.Manifests, uiReg)
	if err != nil {
		t.Fatalf("resolve kafe apps: %v", err)
	}

	b := NewRouterBuilder(reg)
	b.SetUIRegistry(uiReg)
	b.SetApps(resolved)
	return b
}

// TestPublicGrants_KafeQR_DerivedFromSurface pins the entity/action granularity
// an anonymous caller of the customer App gets.
func TestPublicGrants_KafeQR_DerivedFromSurface(t *testing.T) {
	b := kafePublicRouter(t)

	// Granted: what the QR surface actually fetches.
	for _, c := range []struct {
		module, entity, action string
	}{
		{"cafe-master", "menu-item", "list"},
		{"cafe-master", "menu-item-price", "list"},
		{"cafe-master", "dining-table", "find"},
		{"cafe-order", "table-session", "create"},
		{"cafe-order", "table-session", "find"},
		{"cafe-order", "order", "create"},
		{"cafe-order", "order", "list"},
	} {
		if !b.isPublicAction(c.module, c.entity, c.action) {
			t.Errorf("%s/%s %s should be anonymous (the QR surface fetches it)", c.module, c.entity, c.action)
		}
	}

	// Action granularity. `find` on order is refused because the grant carries a
	// row scope, and find resolves by id — a scope cannot guard it.
	if b.isPublicAction("cafe-order", "order", "find") {
		t.Error("order find must NOT be anonymous — the grant is row-scoped and find resolves by id")
	}
	if b.isPublicAction("cafe-master", "dining-table", "list") {
		t.Error("dining-table list must NOT be anonymous — the QR only finds one table by token")
	}
	if b.isPublicAction("cafe-master", "menu-item", "find") {
		t.Error("menu-item find must NOT be anonymous — the picker only lists")
	}
	if b.isPublicAction("cafe-master", "menu-item", "delete") {
		t.Error("delete must never be anonymous")
	}

	// Entity granularity: the staff entities that share a module with the
	// surface. These are what the pre-allowlist module-wide grant leaked.
	for _, ent := range []string{"member", "employee", "branch", "promo"} {
		if b.isPublicAction("cafe-master", ent, "list") {
			t.Errorf("cafe-master/%s list must NOT be anonymous (shares a module with the granted entities)", ent)
		}
	}
	for _, ent := range []string{"shift", "cash-movement", "payment"} {
		if b.isPublicAction("cafe-order", ent, "list") {
			t.Errorf("cafe-order/%s list must NOT be anonymous", ent)
		}
	}
}

// TestPublicGrantScope_KafeQR_GuestToken pins the #45 mechanism: granting `list`
// on a public surface without a scope would hand every row to an anonymous
// caller, so the derivation reads the table block's route parameter and turns it
// into a server-enforced row filter.
func TestPublicGrantScope_KafeQR_GuestToken(t *testing.T) {
	b := kafePublicRouter(t)

	scope := b.publicScope("cafe-order", "order")
	if len(scope) != 1 {
		t.Fatalf("publicScope(order) = %#v, want a single guest_token scope", scope)
	}
	if scope[0].Field != "guest_token" || scope[0].From != "route" || scope[0].Op != "eq" {
		t.Fatalf("order scope = %#v, want {field: guest_token, op: eq, from: route}", scope[0])
	}

	// An entity the surface reads without a route parameter must not inherit a
	// scope — the scope is per-grant, not per-App.
	if got := b.publicScope("cafe-master", "menu-item"); len(got) != 0 {
		t.Fatalf("publicScope(menu-item) = %#v, want none (the picker lists the whole catalog)", got)
	}
}

// TestPublicGrant_PrivateAppGrantsNothing keeps the secure-by-default rule: a
// private App exposes nothing anonymously, whatever its views look like.
func TestPublicGrant_PrivateAppGrantsNothing(t *testing.T) {
	b := kafePublicRouter(t)
	if !b.isPublicAction("cafe-master", "menu-item", "list") {
		t.Fatal("precondition: kafe-qr is public and grants menu-item list")
	}
	// kafe-pos mounts the same modules but is private; its App contributes no
	// grant. (The union across public Apps is what the map holds, so assert on
	// the per-App derivation instead.)
	pos := b.apps["kafe-pos"]
	if pos == nil || pos.Spec.Access != spec.AppAccessPrivate {
		t.Fatal("kafe-pos must be a private App")
	}
	for _, a := range b.derivedPublicGrants()["kafe-pos"] {
		t.Errorf("private App kafe-pos must derive no grants, got %s", a.Entity)
	}
}

func TestApplyPublicScope_FailsClosedWithoutToken(t *testing.T) {
	f := &HandlerFactory{}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(context.WithValue(req.Context(), publicScopeContextKey{}, []spec.FilterSpec{
		{Field: "guest_token", Op: "eq", From: "route", Param: "token"},
	}))

	if _, err := f.applyPublicScope(req, nil); err == nil {
		t.Fatal("expected fail-closed error when the token parameter is missing")
	}
}

func TestApplyPublicScope_TokenBecomesFilter(t *testing.T) {
	f := &HandlerFactory{}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order?token=abc123", nil)
	req = req.WithContext(context.WithValue(req.Context(), publicScopeContextKey{}, []spec.FilterSpec{
		{Field: "guest_token", Op: "eq", From: "route", Param: "token"},
	}))

	got, err := f.applyPublicScope(req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["guest_token"].Value != "abc123" {
		t.Fatalf("guest_token filter = %v, want abc123", got["guest_token"].Value)
	}
}

// The scope overrides a client-supplied filter on the same field: the client
// cannot widen the read by sending its own value.
func TestApplyPublicScope_OverridesClientFilter(t *testing.T) {
	f := &HandlerFactory{}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order?token=abc123", nil)
	req = req.WithContext(context.WithValue(req.Context(), publicScopeContextKey{}, []spec.FilterSpec{
		{Field: "guest_token", Op: "eq", From: "route", Param: "token"},
	}))

	got, err := f.applyPublicScope(req, map[string]db.FilterOp{
		"guest_token": {Op: "eq", Value: "someone-else"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["guest_token"].Value != "abc123" {
		t.Fatalf("client filter must not override the scope: got %v", got["guest_token"].Value)
	}
}

// A route without a public grant must be untouched — the scope is surface-scoped,
// not entity-scoped.
func TestApplyPublicScope_NoGrantLeavesFiltersUntouched(t *testing.T) {
	f := &HandlerFactory{}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)

	got, err := f.applyPublicScope(req, map[string]db.FilterOp{"branch_id": {Op: "eq", Value: "B1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got["branch_id"].Value != "B1" {
		t.Fatalf("filters changed on a non-public route: %#v", got)
	}
}

// TestPublicGrant_FloorNotBypass pins that the grant is a FLOOR, not an
// anonymous-only lane: /_ui/entity is shared with the POS surface, so a
// signed-in caller holding the permission must pass the real permission check
// (its row_scope is what applies, not the guest token).
func TestPublicGrant_FloorNotBypass(t *testing.T) {
	_ = kafePublicRouter(t)

	handler := RequirePermission("cafe-order.orders.list")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	identity := &auth.Identity{
		UserID: "cashier-1", WorkspaceID: "kafe",
		Permissions: []string{"cafe-order.orders.list"},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), identity))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("a holder of the permission must pass: got %d", rec.Code)
	}

	// Without the permission the identity is not anonymous, so the grant does
	// not apply and the request is refused.
	req = httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "staff-1", WorkspaceID: "kafe", Permissions: []string{},
	}))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("an authenticated caller without the permission must not pass on the entity route")
	}
}
