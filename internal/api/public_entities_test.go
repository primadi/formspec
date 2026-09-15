package api

import (
	"testing"

	"github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/pkg/spec"
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
