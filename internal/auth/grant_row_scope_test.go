package auth

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// A role grant carries a ROW SCOPE per action. This is the half a per-entity
// `row_scope` cannot express: the entity declaration filters by WHO (session /
// route attributes) and applies to every caller, so using it to say "only paid
// orders" would blind the cashier to the drafts they are composing.
//
// Kafe 10.67 recorded what its absence cost: the rule "hanya pesanan lunas yang
// masuk dapur" held only because the Kanban happened to declare three columns —
// a direct API call read the draft anyway.
//
// These tests pin the materializer's contract: the row scope survives
// materialization and is attached to the PERMISSION it belongs to, so the API
// layer can enforce it from the permission string alone (it never has to re-read
// and re-match the grant tree).

func TestMaterializeDetailed_AttachesRowScopeToItsPermission(t *testing.T) {
	m, _ := setupMaterializer(t)

	detailed, problems := m.MaterializeDetailed([]Grant{{
		Page: "order-list",
		Actions: []ActionGrant{
			{
				Name:     "list",
				RowScope: []spec.FilterSpec{{Field: "status", Op: "in", Value: "paid,in_kitchen"}},
			},
			{Name: "view"},
		},
	}})
	if len(problems) != 0 {
		t.Fatalf("unexpected grant problems: %v", problems)
	}

	byPerm := map[string][]spec.FilterSpec{}
	for _, d := range detailed {
		byPerm[d.Permission] = d.RowScope
		if d.Page != "order-list" {
			t.Errorf("%s: page = %q, want order-list (needed to name the declaration in an error)", d.Permission, d.Page)
		}
	}

	scoped := byPerm["billing.orders.list"]
	if len(scoped) != 1 {
		t.Fatalf("billing.orders.list row scope = %#v, want exactly one predicate", scoped)
	}
	if scoped[0].Field != "status" || scoped[0].Op != "in" || scoped[0].Value != "paid,in_kitchen" {
		t.Fatalf("row scope mangled: %#v", scoped[0])
	}

	// The restriction must NOT leak onto a sibling action of the same page: a
	// kitchen role that may only LIST paid orders can still view one it holds
	// the id of, and a grant that silently widened `view` would be a boundary
	// the operator never declared.
	if got := byPerm["billing.orders.view"]; len(got) != 0 {
		t.Fatalf("billing.orders.view inherited a row scope it never declared: %#v", got)
	}
}

// Tabbed pages carry their actions one level deeper; the row scope has to make
// the same trip there, or a restriction declared on a tab silently evaporates.
func TestMaterializeDetailed_RowScopeOnTabbedPage(t *testing.T) {
	m, _ := setupMaterializer(t)

	detailed, problems := m.MaterializeDetailed([]Grant{{
		Page: "sales",
		Tabs: []TabGrant{
			{Tab: "Order", Actions: []ActionGrant{
				{Name: "list", RowScope: []spec.FilterSpec{{Field: "status", Value: "paid"}}},
			}},
		},
	}})
	if len(problems) != 0 {
		t.Fatalf("unexpected grant problems: %v", problems)
	}
	if len(detailed) != 1 || detailed[0].Permission != "billing.orders.list" {
		t.Fatalf("detailed = %#v, want the single billing.orders.list grant", detailed)
	}
	if len(detailed[0].RowScope) != 1 || detailed[0].RowScope[0].Value != "paid" {
		t.Fatalf("tab-level row scope did not survive: %#v", detailed[0].RowScope)
	}
}

// Materialize (strict) and MaterializePartial (per-grant) must agree on the
// PERMISSION SET — they are two error-handling strategies over one matching
// rule, and the ledger has already paid once for two copies of a rule drifting
// apart (10.46).
func TestMaterializeDetailed_AgreesWithMaterializeOnPermissions(t *testing.T) {
	m, _ := setupMaterializer(t)
	grants := []Grant{
		{Page: "order-list", Actions: []ActionGrant{
			{Name: "list", RowScope: []spec.FilterSpec{{Field: "status", Value: "paid"}}},
			{Name: "view"},
		}},
		{Page: "customer-page", Actions: []ActionGrant{{Name: "list"}}},
	}

	strict, err := m.Materialize(grants)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	partial, problems := m.MaterializePartial(grants)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	detailed, dproblems := m.MaterializeDetailed(grants)
	if len(dproblems) != 0 {
		t.Fatalf("unexpected problems: %v", dproblems)
	}

	if len(strict) != len(partial) {
		t.Fatalf("strict=%v partial=%v — the two strategies must agree on the permission set", strict, partial)
	}
	if len(strict) != len(detailed) {
		t.Fatalf("strict=%v detailed has %d entries", strict, len(detailed))
	}
	for i, d := range detailed {
		if strict[i] != d.Permission {
			t.Errorf("detailed[%d] = %q, strict[%d] = %q", i, d.Permission, i, strict[i])
		}
	}
}
