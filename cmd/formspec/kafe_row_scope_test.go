package main

import (
	"testing"

	"github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Item 3.5 turns row scoping ON in the kafe tree, and two properties have to hold
// for that to be safe rather than merely strict:
//
//   - every session-scoped entity must have a resolvable source, or every read
//     fails closed (the 1.8 gate refuses that shape, and this test keeps the
//     kafe tree itself honest);
//   - an entity that is ALSO read by an anonymous surface needs BOTH halves: the
//     session scope for staff, and a scope the public grant inherits for guests.
//
// The second rule replaced an older "never both" rule, which treated the two
// surfaces as mutually exclusive. That reading is what let `menu-item-price` be
// listed anonymously with NO server-side scope at all: the entity was scoped
// nowhere, because scoping it on the session looked like a rule violation and
// the guest's branch lived in a client-side filter (kafe 10.76). The
// runtime supports having both — a session `row_scope` is skipped for an
// anonymous caller carrying a public grant scope, since there is no session
// attribute to read — so the tree is expected to declare both.
func TestKafeRowScopeSpec_ScopeAndSource(t *testing.T) {
	const specPath = "../../examples/kafe/spec"
	res, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load spec tree: %v", err)
	}

	// Entities that are legitimately scoped on the session: read through
	// authenticated surfaces (POS, KDS, admin/report) — and, for a price list,
	// also read anonymously with a scope the public grant carries.
	wantScoped := map[string]bool{
		"order": true, "payment": true, "shift": true, "cash-movement": true,
		"stock-level": true, "stock-movement": true, "purchase-order": true,
		"stock-opname": true, "waste-entry": true, "menu-cost": true,
		"menu-item-price": true,
	}

	scoped := map[string]bool{}
	branchSource := false
	for _, m := range res.Manifests {
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
		for _, a := range es.Assignments {
			if a.Dimension == "branch" {
				branchSource = true
			}
		}
		if len(es.RowScope) == 0 {
			continue
		}
		scoped[m.Metadata.Name] = true
		if es.Scope == nil {
			t.Errorf("%s: row_scope without a `scope:` descriptor — the session attribute name becomes undefined", m.Metadata.Name)
		}
		for _, sc := range es.RowScope {
			if sc.From != "session" {
				t.Errorf("%s: row_scope %s uses from=%q; session is what a branch scope means", m.Metadata.Name, sc.Field, sc.From)
			}
		}
	}

	if len(scoped) == 0 {
		t.Fatal("no entity enables row_scope — item 3.5 is not actually on")
	}
	for name := range scoped {
		if !wantScoped[name] {
			t.Errorf("entity %q is scoped but is not in the authenticated-only set — check whether an anonymous surface reads it", name)
		}
	}
	for name := range wantScoped {
		if !scoped[name] {
			t.Errorf("entity %q should be scoped (item 3.5) but is not", name)
		}
	}

	// The anonymous half. `menu-item-price` is read by the guest catalog, so its
	// session scope is not enough on its own: the picker that fetches it must
	// declare the scope the guest read is filtered by, or the anonymous list is
	// unscoped (kafe 10.76). Asserted on the DECLARATION here; the derived grant
	// it feeds is asserted in internal/api (TestPublicGrantScope_KafeQR_*).
	if !scoped["menu-item-price"] {
		t.Error("menu-item-price must be scoped on the session for staff — a cashier would otherwise see every branch's prices")
	}
	if got := kafePickerLookupScope(res.Manifests); got != "branch_id" {
		t.Errorf("the QR picker must declare a branch lookup.scope (got %q) — without it the anonymous lookup list has no server-side scope", got)
	}
	if !branchSource {
		t.Fatal("row_scope is on, but nothing declares an `assignments` mapping for `branch` — every scoped read would fail closed")
	}
}

// kafePickerLookupScope returns the field a child-field picker declares in its
// `lookup.scope`, or "" when none does. It walks the manifests rather
// than the spec types so a declaration that failed to survive YAML decoding
// still shows up as missing.
func kafePickerLookupScope(manifests []manifest.RawManifest) string {
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Metadata.Name != "order" {
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
		for i := range es.Fields {
			f := &es.Fields[i]
			if f.Child == nil || f.Child.Picker == nil {
				continue
			}
			if lk := f.Child.Picker.Lookup; lk != nil {
				for i := range lk.Scope {
					if lk.Scope[i].Field != "" {
						return lk.Scope[i].Field
					}
				}
			}
		}
	}
	return ""
}

// The runtime resolves `from: session` through the registry's assignment
// sources; this pins the one the kafe tree relies on.
func TestKafeAssignmentSources_EmployeeMapsUsernameToBranch(t *testing.T) {
	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = database.Close() }()

	reg := entity.NewRegistry(database, db.DriverSQLite, "../../examples/kafe/spec")
	if err := reg.LoadEntities(); err != nil {
		t.Fatalf("load entities: %v", err)
	}

	for _, s := range reg.AssignmentSources() {
		if s.Module == "cafe-master" && s.Entity == "employee" &&
			s.Dimension == "branch" && s.Field == "branch_id" &&
			s.PrincipalField == "username" {
			return
		}
	}
	t.Fatalf("cafe-master.employee must map username → branch_id, got %#v", reg.AssignmentSources())
}

// A public grant that constrains rows carries its own scope; anything else
// anonymous would be unfiltered, which is what the derivation exists to prevent.
//
// The allowlist is no longer written in the manifest — it is derived from the
// App's surface (plan docs_internal/plan/implicit-public-grants.md), so this
// test derives from the same kafe tree and checks the invariants on the result.
func TestKafePublicGrants_ScopedWhereRowsMatter(t *testing.T) {
	const specPath = "../../examples/kafe/spec"

	loaded, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	uiReg := ui.NewRegistry()
	if errs := uiReg.LoadDir(specPath); len(errs) > 0 {
		t.Fatalf("load UI manifests: %v", errs)
	}
	apps, err := app.Resolve(loaded.Manifests, uiReg)
	if err != nil {
		t.Fatalf("resolve apps: %v", err)
	}

	// An EntityLister over the same tree.
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

	sawScoped := false
	for name, a := range apps {
		if a.Spec == nil || a.Spec.Access != spec.AppAccessPublic {
			continue
		}
		for _, decl := range uiReg.DerivePublicGrants(lister, ui.PublicGrantInput{
			Modules:         a.Modules,
			Menu:            a.Menu,
			RegisteredViews: a.Spec.RegisteredViews,
		}) {
			for _, sc := range decl.Scope {
				if sc.From != "route" {
					t.Errorf("%s: public grant %s must scope from a route parameter, got from=%q", name, decl.Entity, sc.From)
				}
				sawScoped = true
			}
			for _, act := range decl.Actions {
				if act == "find" && len(decl.Scope) > 0 {
					t.Errorf("%s: grant %s both declares a scope and grants find — find resolves by id and cannot be guarded", name, decl.Entity)
				}
			}
		}
	}
	if !sawScoped {
		t.Fatal("expected at least one public grant to constrain its rows (order by guest token)")
	}
}
