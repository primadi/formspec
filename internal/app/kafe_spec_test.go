package app

import (
	"sort"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
)

// TestResolve_KafeSpec loads the real kafe tree and resolves its Apps. This is
// the end-to-end half of registered-views validation (plan registered-views.md):
// every `registered_views` entry must name a view/entity that EXISTS in the kafe
// tree, or Resolve fails at boot. A unit test with a synthetic registry cannot
// catch a typo in the example manifests; this can.
func TestResolve_KafeSpec(t *testing.T) {
	const specPath = "../../examples/kafe/spec"

	loaded, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load kafe spec tree: %v", err)
	}
	for _, perr := range loaded.Errors {
		t.Fatalf("parse error: %v", &perr)
	}

	uiReg := ui.NewRegistry()
	if errs := uiReg.LoadDir(specPath); len(errs) > 0 {
		t.Fatalf("load kafe UI manifests: %v", errs)
	}

	apps, err := Resolve(loaded.Manifests, uiReg)
	if err != nil {
		t.Fatalf("resolve kafe apps: %v", err)
	}

	// The two menu-less Apps are exactly the ones that NEED registered_views:
	// nothing else declares what they expose.
	for _, name := range []string{"kafe-qr", "kafe-kds"} {
		a, ok := apps[name]
		if !ok {
			t.Fatalf("app %q not resolved", name)
		}
		if len(a.Spec.RegisteredViews) == 0 {
			t.Errorf("app %q mounts modules but declares no registered_views — its surface would be empty", name)
		}
	}

	// kafe-qr is the anonymous customer surface: it must expose the three pages
	// the QR flow walks through.
	saw := map[string]bool{}
	for _, rv := range apps["kafe-qr"].Spec.RegisteredViews {
		saw[rv.View] = true
	}
	for _, want := range []string{
		"cafe-order/table-open",
		"cafe-order/menu-catalog",
		"cafe-order/order-status-page",
	} {
		if !saw[want] {
			t.Errorf("kafe-qr registered_views missing %q (got %v)", want, saw)
		}
	}
}

// TestDerivePublicGrants_KafeQR pins the DERIVED anonymous allowlist for the
// real kafe tree (plan docs_internal/plan/implicit-public-grants.md). The
// manifest no longer declares `public_entities`; the grant is a consequence of
// the views the App exposes, so this test is the contract for that derivation.
//
// The measurement that matters: derivation must expose exactly what the
// customer surface fetches — and must NOT expose the staff entities that
// previously leaked when the grant was derived from `modules` alone (member
// phone numbers, employees, shifts, cash movements).
func TestDerivePublicGrants_KafeQR(t *testing.T) {
	const specPath = "../../examples/kafe/spec"

	loaded, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load kafe spec tree: %v", err)
	}
	uiReg := ui.NewRegistry()
	if errs := uiReg.LoadDir(specPath); len(errs) > 0 {
		t.Fatalf("load kafe UI manifests: %v", errs)
	}
	apps, err := Resolve(loaded.Manifests, uiReg)
	if err != nil {
		t.Fatalf("resolve kafe apps: %v", err)
	}

	qr := apps["kafe-qr"]
	if qr == nil {
		t.Fatal("kafe-qr not resolved")
	}

	// An EntityLister over the same tree the App mounts.
	lister := func() []ui.EntityDescriptor {
		var out []ui.EntityDescriptor
		for _, m := range loaded.Manifests {
			if spec.Kind(m.Kind) != spec.KindEntity {
				continue
			}
			sm, ok := m.Spec.(map[string]any)
			if !ok {
				continue
			}
			es, err := manifest.RawSpecToEntitySpec(sm)
			if err != nil {
				continue
			}
			out = append(out, ui.EntityDescriptor{Module: m.Metadata.Module, Name: m.Metadata.Name, Spec: es})
		}
		return out
	}

	decls := uiReg.DerivePublicGrants(lister, ui.PublicGrantInput{
		Modules:         qr.Modules,
		Menu:            qr.Menu,
		RegisteredViews: qr.Spec.RegisteredViews,
	})

	got := map[string][]string{}
	for _, d := range decls {
		actions := append([]string(nil), d.Actions...)
		sort.Strings(actions)
		got[d.Entity] = actions
	}

	// Whatever the surface fetches must be granted.
	for _, want := range []struct {
		entity  string
		actions []string
	}{
		// The QR form creates orders; the status page lists them by token.
		{"cafe-order/order", []string{"create", "list"}},
		// The check-in form creates a session and the menu form reads it.
		{"cafe-order/table-session", []string{"create", "find"}},
		// The check-in form resolves the table from the QR token.
		{"cafe-master/dining-table", []string{"find"}},
		// The menu picker lists items and joins their per-branch price.
		{"cafe-master/menu-item", []string{"list"}},
		{"cafe-master/menu-item-price", []string{"list"}},
	} {
		have, ok := got[want.entity]
		if !ok {
			t.Errorf("derived grant missing %s (got %v)", want.entity, got)
			continue
		}
		if len(have) != len(want.actions) {
			t.Errorf("%s actions = %v, want %v", want.entity, have, want.actions)
			continue
		}
		for i := range have {
			if have[i] != want.actions[i] {
				t.Errorf("%s actions = %v, want %v", want.entity, have, want.actions)
				break
			}
		}
	}

	// Staff-only entities must NOT be granted — the whole point of the narrow
	// surface. These are the ones the old module-wide derivation leaked.
	for _, ent := range []string{
		"cafe-master/member",
		"cafe-master/employee",
		"cafe-master/branch",
		"cafe-master/promo",
		"cafe-order/shift",
		"cafe-order/cash-movement",
		"cafe-order/payment",
	} {
		if _, ok := got[ent]; ok {
			t.Errorf("%s must NOT be anonymous — it shares a module with the surface but nothing fetches it", ent)
		}
	}

	// `menu-category` has no client fetch: the picker reads the category label
	// off the already-loaded rows (`picker.display.category_field`), so a grant
	// would be unused. This is the precision the derivation buys over the old
	// hand-written allowlist, which listed it.
	if _, ok := got["cafe-master/menu-category"]; ok {
		t.Error("menu-category must NOT be granted — no client request fetches it")
	}

	// `find` and `delete` are never implied together with a scope, and delete is
	// never implied at all.
	for _, d := range decls {
		if len(d.Scope) > 0 {
			for _, a := range d.Actions {
				if a == "find" {
					t.Errorf("%s: `find` granted together with a scope — find resolves by id and cannot be guarded", d.Entity)
				}
			}
		}
		for _, a := range d.Actions {
			if a == "delete" {
				t.Errorf("%s: `delete` must never be granted anonymously", d.Entity)
			}
		}
	}

	// The order list is scoped by the guest token carried in the route, so an
	// anonymous list without a token is refused rather than unfiltered.
	var orderScope []spec.FilterSpec
	for _, d := range decls {
		if d.Entity == "cafe-order/order" {
			orderScope = d.Scope
		}
	}
	if len(orderScope) != 1 || orderScope[0].Field != "guest_token" || orderScope[0].From != "route" {
		t.Fatalf("order scope = %#v, want a single guest_token row scope from the route", orderScope)
	}
}
