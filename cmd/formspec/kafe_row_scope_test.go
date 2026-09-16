package main

import (
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Item 3.5 turns row scoping ON in the kafe tree, and two properties have to hold
// for that to be safe rather than merely strict:
//
//   - every session-scoped entity must have a resolvable source, or every read
//     fails closed (the 1.8 gate refuses that shape, and this test keeps the
//     kafe tree itself honest);
//   - the entities read by ANONYMOUS surfaces must not be scoped on the session
//     at all — an anonymous caller has no branch, and those reads are constrained
//     by the public grant's own scope instead.
//
// The runtime half (kasir B1 vs B2, `read_all` owner, fail-closed probe) is
// recorded in the ledger with the exact requests and results.
func TestKafeRowScopeSpec_ScopeAndSource(t *testing.T) {
	const specPath = "../../examples/kafe/spec"
	res, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load spec tree: %v", err)
	}

	// Entities that are legitimately scoped on the session: read only through
	// authenticated surfaces (POS, KDS, admin/report).
	wantScoped := map[string]bool{
		"order": true, "payment": true, "shift": true, "cash-movement": true,
		"stock-level": true, "stock-movement": true, "purchase-order": true,
		"stock-opname": true, "waste-entry": true, "menu-cost": true,
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

	// Anonymous-read entities are constrained by their public grant instead.
	if scoped["menu-item-price"] {
		t.Error("menu-item-price is listed anonymously by the QR catalog picker: scope it in the public grant, not on the session")
	}
	if !branchSource {
		t.Fatal("row_scope is on, but nothing declares an `assignments` mapping for `branch` — every scoped read would fail closed")
	}
}

// The runtime resolves `from: session` through the registry's assignment
// sources; this pins the one the kafe tree relies on.
func TestKafeAssignmentSources_EmployeeMapsUsernameToBranch(t *testing.T) {
	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

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
// anonymous would be unfiltered, which is what the allowlist exists to prevent.
func TestKafePublicGrants_ScopedWhereRowsMatter(t *testing.T) {
	res, err := manifest.NewLoader("../../examples/kafe/spec").LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	sawScoped := false
	for _, m := range res.Manifests {
		if spec.Kind(m.Kind) != spec.KindApp {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		as, err := manifest.RawSpecToAppSpec(sm)
		if err != nil || as.PublicEntities == nil {
			continue
		}
		for _, decl := range *as.PublicEntities {
			if len(decl.Scope) == 0 {
				continue
			}
			sawScoped = true
			for _, sc := range decl.Scope {
				if sc.From != "route" {
					t.Errorf("%s: public grant %s must scope from a route parameter, got from=%q", m.Metadata.Name, decl.Entity, sc.From)
				}
			}
			for _, act := range decl.Actions {
				if act == "find" {
					t.Errorf("%s: grant %s both declares a scope and grants find — find resolves by id and cannot be guarded", m.Metadata.Name, decl.Entity)
				}
			}
		}
	}
	if !sawScoped {
		t.Fatal("expected at least one public grant to constrain its rows (order by guest token, prices by branch)")
	}
}
