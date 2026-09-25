package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// The anonymous allowlist (S3). Before it, `access: public` granted anonymous
// callers list/find/create on EVERY entity of a mounted module — so a cafe's
// public QR App also exposed member phone numbers, employees, shifts and cash
// movements (#6). These tests pin the narrower semantics:
//
//   - declared allowlist → exactly those entity/action pairs, nothing else in
//     the same module;
//   - `public_entities: []` → nothing anonymous;
//   - absent → legacy module-wide list/find/create, but never update/delete.

func publicRouter(t *testing.T, specApps ...*spec.AppSpec) *RouterBuilder {
	t.Helper()
	apps := map[string]*app.ResolvedApp{}
	for i, s := range specApps {
		name := s.Title
		if name == "" {
			name = "app"
		}
		modules := map[string]bool{}
		for _, m := range s.Modules {
			modules[m] = true
		}
		apps[itoa(i)] = &app.ResolvedApp{Name: name, Spec: s, Modules: modules}
	}
	b := setupMetaTestRouter(t)
	b.SetApps(apps)
	return b
}

func itoa(i int) string { return string(rune('a' + i)) }

// TestPublicGrantScope_TokenScopedAnonList pins the #45 mechanism: granting
// `list` on a public surface without a scope hands every row to an anonymous
// caller, so the grant declares the token the caller must present. The scope is
// per-surface — the same entity read by a cashier is not filtered by it.
func TestPublicGrantScope_TokenScopedAnonList(t *testing.T) {
	allow := []spec.PublicEntityDecl{
		{
			Entity:  "cafe-order/order",
			Actions: []string{"create", "list"},
			Scope:   []spec.FilterSpec{{Field: "guest_token", Op: "eq", From: "route", Param: "token"}},
		},
		{Entity: "cafe-master/menu-item", Actions: []string{"list"}},
	}
	b := publicRouter(t, &spec.AppSpec{
		Title:          "qr",
		AppRenderer:    "no-nav",
		Access:         spec.AppAccessPublic,
		Modules:        []string{"cafe-master", "cafe-order"},
		PublicEntities: &allow,
	})

	scope := b.publicScope("cafe-order", "order")
	if len(scope) != 1 || scope[0].Field != "guest_token" || scope[0].Param != "token" {
		t.Fatalf("publicScope(order) = %#v, want the declared guest_token scope", scope)
	}
	// An entity in the same App without a scope must not inherit one.
	if got := b.publicScope("cafe-master", "menu-item"); len(got) != 0 {
		t.Fatalf("publicScope(menu-item) = %#v, want none", got)
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

// The grant governs ANONYMOUS access. An authenticated caller is governed by its
// permissions plus the entity's own row_scope — applying the grant's token scope
// to it would filter the cashier's POS list by a token it never carries.
func TestApplyPublicScope_SkipsAuthenticatedCallers(t *testing.T) {
	f := &HandlerFactory{}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(context.WithValue(req.Context(), publicScopeContextKey{}, []spec.FilterSpec{
		{Field: "guest_token", Op: "eq", From: "route"},
	}))
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "emp-1", WorkspaceID: "kafe",
	}))

	got, err := f.applyPublicScope(req, nil)
	if err != nil {
		t.Fatalf("authenticated caller must not fail closed on a public grant: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("authenticated caller was filtered by the anonymous grant: %#v", got)
	}
}

// A caller authorized BY the grant (signed in, but holding no permission of its
// own for this route) IS scoped by it. That is the whole point of the fallback:
// the fallback hands out exactly what a guest gets, never more — the guest's row
// scope travels with it.
func TestApplyPublicScope_GrantAuthorisedCallerIsScoped(t *testing.T) {
	f := &HandlerFactory{}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order?guest_token=abc123", nil)
	req = req.WithContext(context.WithValue(req.Context(), publicScopeContextKey{}, []spec.FilterSpec{
		{Field: "guest_token", Op: "eq", From: "route"},
	}))
	req = req.WithContext(withPublicGrantAuth(req.Context()))
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "u1", WorkspaceID: "kafe",
	}))

	got, err := f.applyPublicScope(req, nil)
	if err != nil {
		t.Fatalf("grant-authorised caller: %v", err)
	}
	if got["guest_token"].Value != "abc123" {
		t.Fatalf("grant-authorised caller must be scoped by the grant: %#v", got)
	}

	// …and still fails closed without the token, exactly like a guest.
	req = httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(context.WithValue(req.Context(), publicScopeContextKey{}, []spec.FilterSpec{
		{Field: "guest_token", Op: "eq", From: "route"},
	}))
	req = req.WithContext(withPublicGrantAuth(req.Context()))
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "u1"}))
	if _, err := f.applyPublicScope(req, nil); err == nil {
		t.Fatal("grant-authorised caller must fail closed without the token")
	}
}

// A public grant on an entity that also declares `row_scope from: session` must
// not fail closed for a caller running on the grant: the grant already decided
// which rows are in scope, and a session attribute may not exist for them.
func TestApplyRowScope_GrantAuthorisedCallerSkipsSessionScope(t *testing.T) {
	f := &HandlerFactory{}
	// `attr` is explicit and unresolvable here: without the grant exemption the
	// caller fails closed, which is what makes this test able to tell the two
	// branches apart.
	es := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{Field: "branch_id", Op: "eq", From: "session", Attr: "branch_id"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(context.WithValue(req.Context(), publicScopeContextKey{}, []spec.FilterSpec{
		{Field: "guest_token", Op: "eq", From: "route"},
	}))
	req = req.WithContext(withPublicGrantAuth(req.Context()))
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "u1", Username: "kasir"}))

	if _, err := f.applyRowScope(req, es, "cafe-order", "order", nil); err != nil {
		t.Fatalf("grant-authorised caller must not fail closed on row_scope: %v", err)
	}

	// A caller holding its own permission is still scoped by the session
	// attribute — the exemption is tied to the grant, not to being signed in.
	req = httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(context.WithValue(req.Context(), publicScopeContextKey{}, []spec.FilterSpec{
		{Field: "guest_token", Op: "eq", From: "route"},
	}))
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "u1", Username: "kasir"}))
	if _, err := f.applyRowScope(req, es, "cafe-order", "order", nil); err == nil {
		t.Fatal("a permissioned caller must still fail closed without the session attribute")
	}
}

// A public grant must not become a permission bypass for signed-in callers that
// merely hit the same URL (#45) — but it must not make them WORSE OFF than a
// guest either. The grant is a floor: a signed-in caller without the permission
// falls back to it.
func TestRequirePermissionOrAnonymous(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Grant-Auth", map[bool]string{true: "1", false: "0"}[isPublicGrantAuth(r.Context())])
		w.WriteHeader(http.StatusOK)
	})
	guarded := RequirePermissionOrAnonymous("cafe-order.orders.list")(ok)

	// Anonymous → allowed (the grant is the authorization), not marked.
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("anonymous = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-Grant-Auth") != "0" {
		t.Error("anonymous caller must not be marked as grant-authorized")
	}

	// Authenticated WITHOUT the permission → allowed via the grant, and marked
	// so the row-scope layers apply the grant's scope instead of failing closed.
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "u1", Permissions: []string{"other.thing.list"}}))
	rec = httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("signed-in caller without the permission = %d, want 200 (the grant is the floor)", rec.Code)
	}
	if rec.Header().Get("X-Grant-Auth") != "1" {
		t.Error("a caller falling back to the grant must be marked as grant-authorized")
	}

	// Authenticated with the permission → allowed, and NOT marked (it is
	// governed by its own permission and the entity's row scope).
	req = httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "u1", Permissions: []string{"cafe-order.orders.list"}}))
	rec = httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated caller with the permission = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-Grant-Auth") != "0" {
		t.Error("a caller holding the permission must not be marked as grant-authorized")
	}
}

// A signed-in caller on a NON-public route still gets the plain permission
// check: the fallback belongs to the grant, not to being signed in.
func TestRequirePermission_SignedInWithoutPermStillDenied(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	id := &auth.Identity{UserID: "u1", Permissions: []string{"other.thing.list"}}

	// Entity visibility (list/view) on the UI surface → 404, per spec §4: a
	// caller who may not see the entity must not learn it exists.
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), id))
	rec := httptest.NewRecorder()
	RequirePermission("cafe-order.orders.list")(ok).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("UI-surface list without permission = %d, want 404", rec.Code)
	}

	// Any other action (here: update) → 403, the ordinary denial.
	req = httptest.NewRequest("PATCH", "/kafe/_ui/entity/cafe-order/order/1", nil)
	req = req.WithContext(WithIdentity(req.Context(), id))
	rec = httptest.NewRecorder()
	RequirePermission("cafe-order.orders.update")(ok).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("UI-surface update without permission = %d, want 403", rec.Code)
	}
}

func TestPublicAccess_AllowlistScopesPerEntityAndAction(t *testing.T) {
	allow := []spec.PublicEntityDecl{
		{Entity: "cafe-master/menu-item", Actions: []string{"list", "find"}},
		{Entity: "cafe-master.menu-item-price", Actions: []string{"list"}}, // dotted form
		{Entity: "cafe-order/order", Actions: []string{"create"}},
	}
	b := publicRouter(t, &spec.AppSpec{
		Title:          "qr",
		AppRenderer:    "no-nav",
		Access:         spec.AppAccessPublic,
		Modules:        []string{"cafe-master", "cafe-order"},
		PublicEntities: &allow,
	})

	// Granted.
	if !b.isPublicAction("cafe-master", "menu-item", "list") {
		t.Error("menu-item list should be anonymous")
	}
	if !b.isPublicAction("cafe-master", "menu-item", "find") {
		t.Error("menu-item find should be anonymous")
	}
	if !b.isPublicAction("cafe-order", "order", "create") {
		t.Error("order create should be anonymous")
	}

	// Action granularity: same entity, action not granted.
	if b.isPublicAction("cafe-master", "menu-item", "create") {
		t.Error("menu-item create must NOT be anonymous (not granted)")
	}
	if b.isPublicAction("cafe-order", "order", "list") {
		t.Error("order list must NOT be anonymous (only create was granted)")
	}
	if b.isPublicAction("cafe-master", "menu-item", "delete") {
		t.Error("menu-item delete must NOT be anonymous")
	}

	// Entity granularity: same module, entity not listed — the actual #6 fix.
	for _, ent := range []string{"member", "employee", "branch", "promo"} {
		if b.isPublicAction("cafe-master", ent, "list") {
			t.Errorf("cafe-master/%s list must NOT be anonymous (shares a module with a granted entity)", ent)
		}
	}
	for _, ent := range []string{"shift", "cash-movement", "payment"} {
		if b.isPublicAction("cafe-order", ent, "list") {
			t.Errorf("cafe-order/%s list must NOT be anonymous", ent)
		}
	}
}

func TestPublicAccess_ExplicitlyEmptyGrantsNothing(t *testing.T) {
	empty := []spec.PublicEntityDecl{}
	b := publicRouter(t, &spec.AppSpec{
		Title:          "closed",
		Access:         spec.AppAccessPublic,
		Modules:        []string{"cafe-master"},
		PublicEntities: &empty,
	})
	if b.isPublicEntity("cafe-master", "menu-item") {
		t.Error("`public_entities: []` must grant nothing (not fall back to module-wide)")
	}
	if b.isPublicAction("cafe-master", "menu-item", "list") {
		t.Error("`public_entities: []` must grant no actions")
	}
}

func TestPublicAccess_AbsentKeepsLegacyModuleWide(t *testing.T) {
	b := publicRouter(t, &spec.AppSpec{
		Title:   "legacy",
		Access:  spec.AppAccessPublic,
		Modules: []string{"sales"},
	})
	// Legacy default: list/find/create for every entity of the mounted module.
	for _, act := range []string{"list", "find", "create"} {
		if !b.isPublicAction("sales", "product", act) {
			t.Errorf("legacy public App should allow %s anonymously", act)
		}
	}
	// Admin ops were never granted by the legacy path either.
	for _, act := range []string{"update", "delete"} {
		if b.isPublicAction("sales", "product", act) {
			t.Errorf("legacy public App must not allow %s anonymously", act)
		}
	}
}

func TestPublicAccess_PrivateAppGrantsNothing(t *testing.T) {
	allow := []spec.PublicEntityDecl{{Entity: "sales/product", Actions: []string{"list"}}}
	b := publicRouter(t, &spec.AppSpec{
		Title:          "backoffice",
		Access:         spec.AppAccessPrivate,
		Modules:        []string{"sales"},
		PublicEntities: &allow, // ignored: only public Apps grant
	})
	if b.isPublicEntity("sales", "product") {
		t.Error("a private App must not expose anything anonymously")
	}
}
