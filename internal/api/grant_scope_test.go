package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Grant row scope is the enforcement that makes a per-role row rule real
// (kafe 10.67: "hanya pesanan lunas yang masuk dapur"; GAP-08: the kitchen sees
// its own branch). The entity's own `row_scope` cannot express it — it filters
// by WHO and applies to every caller — so the rule has to come from the role
// grant, per (role, action).
//
// These tests pin the API layer's half: it must ask for the restriction using
// the SAME permission string the materializer produced (§8.6 canonical
// `{module}.{plural}.{action}`), must resolve the values server-side, and must
// fail closed when a value cannot be resolved.

func grantScopeFactory(t *testing.T, scope []spec.FilterSpec) (*HandlerFactory, *string) {
	t.Helper()
	f := &HandlerFactory{}
	var asked string
	f.SetGrantScopeLookup(func(_ context.Context, _, _ string, _ []string, permission string) ([]spec.FilterSpec, error) {
		asked = permission
		return scope, nil
	})
	return f, &asked
}

// A lookup that REPORTED a defect must deny the request. An error from the
// resolver means a row restriction was declared and could not be applied;
// answering 200 with fewer predicates is the 10.67 leak (a role that looks
// confined to the paid orders while reading every row), so the request is
// refused instead.
func TestGrantRowPredicates_LookupErrorDenies(t *testing.T) {
	f := &HandlerFactory{}
	f.SetGrantScopeLookup(func(_ context.Context, _, _ string, _ []string, _ string) ([]spec.FilterSpec, error) {
		return nil, errors.New("role \"dapur\" declares a row restriction that cannot be read")
	})

	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "u1", WorkspaceID: "kafe", App: "kafe-kds", Roles: []string{"dapur"},
	}))

	preds, err := f.grantRowPredicates(req, &spec.EntitySpec{Plural: "orders"}, "cafe-order", "order", "list")
	if err == nil {
		t.Fatal("a reported row-restriction defect must deny the request, not read unscoped")
	}
	if preds != nil {
		t.Fatalf("denied request must carry no predicates, got %#v", preds)
	}

	// And the denial travels through the combined entry point too, so a list
	// handler cannot accidentally take the entity-only half.
	if _, err := f.rowPredicatesFor(req, &spec.EntitySpec{Plural: "orders"}, "cafe-order", "order", "list"); err == nil {
		t.Fatal("rowPredicatesFor must propagate the denial")
	}
}

func TestGrantRowPredicates_UsesCanonicalPermissionName(t *testing.T) {
	f, asked := grantScopeFactory(t, []spec.FilterSpec{{Field: "status", Op: "in", Value: "paid,in_kitchen"}})
	es := &spec.EntitySpec{Plural: "orders"}

	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "u1", WorkspaceID: "kafe", App: "kafe-kds",
		Roles: []string{"barista"},
	}))

	preds, err := f.grantRowPredicates(req, es, "cafe-order", "order", "list")
	if err != nil {
		t.Fatalf("grantRowPredicates: %v", err)
	}

	// The name the materializer emits for `{name: list}` on `order-page`. If the
	// enforcement side picked a different spelling, the gate could never open —
	// the failure mode kafe 10.47 recorded for transition permissions.
	if *asked != "cafe-order.orders.list" {
		t.Fatalf("asked for %q, want cafe-order.orders.list (the materialized name)", *asked)
	}
	if len(preds) != 1 {
		t.Fatalf("predicates = %#v, want one", preds)
	}
	if preds[0].Field != "status" || preds[0].Op != "in" {
		t.Fatalf("predicate = %#v", preds[0])
	}
	values, ok := preds[0].Value.([]any)
	if !ok || len(values) != 2 || values[0] != "paid" {
		t.Fatalf("values = %#v, want [paid in_kitchen]", preds[0].Value)
	}
}

func TestGrantRowPredicates_SessionValueResolvedServerSide(t *testing.T) {
	// A grant may scope by session attribute too (the branch filter, GAP-08).
	f, _ := grantScopeFactory(t, []spec.FilterSpec{
		{Field: "branch_id", Op: "eq", From: "session", Attr: "branch_id"},
	})

	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "u1", WorkspaceID: "kafe", App: "kafe-kds",
		Roles:      []string{"barista"},
		Attributes: map[string]string{"branch_id": "KFE-JKT-01"},
	}))

	preds, err := f.grantRowPredicates(req, &spec.EntitySpec{Plural: "orders"}, "cafe-order", "order", "list")
	if err != nil {
		t.Fatalf("grantRowPredicates: %v", err)
	}
	if len(preds) != 1 || preds[0].Value != "KFE-JKT-01" {
		t.Fatalf("predicates = %#v, want the session's branch value", preds)
	}
}

func TestGrantRowPredicates_UnresolvableSessionAttributeFailsClosed(t *testing.T) {
	f, _ := grantScopeFactory(t, []spec.FilterSpec{
		{Field: "branch_id", Op: "eq", From: "session", Attr: "branch_id"},
	})
	// Signed in, but with no branch assignment: the scope cannot be resolved.
	// Degrading to "no restriction" is exactly the leak the rule exists to
	// prevent, so this must error.
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "u1", WorkspaceID: "kafe", Roles: []string{"barista"},
	}))

	if _, err := f.grantRowPredicates(req, &spec.EntitySpec{Plural: "orders"}, "cafe-order", "order", "list"); err == nil {
		t.Fatal("expected fail-closed when the grant's session attribute cannot be resolved")
	}
}

func TestGrantRowPredicates_NoIdentityOrNoGrantsIsNoop(t *testing.T) {
	f, asked := grantScopeFactory(t, []spec.FilterSpec{{Field: "status", Value: "paid"}})
	es := &spec.EntitySpec{Plural: "orders"}

	// Anonymous: there are no roles to resolve, so nothing is asked and nothing
	// is added. (Anonymous row authority comes from the public grant, not here.)
	anon := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	if preds, err := f.grantRowPredicates(anon, es, "cafe-order", "order", "list"); err != nil || len(preds) != 0 {
		t.Fatalf("anonymous caller: preds=%#v err=%v, want none", preds, err)
	}
	if *asked != "" {
		t.Fatalf("anonymous caller caused a grant lookup for %q", *asked)
	}
}

// rowPredicatesFor combines both halves — the entity's scope AND the grant's —
// so a restriction on the same field narrows instead of one replacing the other.
func TestRowPredicatesFor_CombinesEntityAndGrantScope(t *testing.T) {
	f, _ := grantScopeFactory(t, []spec.FilterSpec{{Field: "status", Op: "eq", Value: "paid"}})
	es := &spec.EntitySpec{
		Plural:   "orders",
		RowScope: []spec.FilterSpec{{Field: "branch_id", Op: "eq", From: "session", Attr: "branch_id"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "u1", WorkspaceID: "kafe", Roles: []string{"barista"},
		Attributes: map[string]string{"branch_id": "B1"},
	}))

	preds, err := f.rowPredicatesFor(req, es, "cafe-order", "order", "list")
	if err != nil {
		t.Fatalf("rowPredicatesFor: %v", err)
	}
	if len(preds) != 2 {
		t.Fatalf("predicates = %#v, want BOTH the entity scope and the grant scope", preds)
	}
	fields := map[string]any{}
	for _, p := range preds {
		fields[p.Field] = p.Value
	}
	if fields["branch_id"] != "B1" || fields["status"] != "paid" {
		t.Fatalf("predicates = %#v, want branch_id=B1 and status=paid", preds)
	}
}

// The read_all exemption is about ROWS, not about the role's own restrictions:
// an owner reads across branches, but a grant that says "only paid orders" is
// still that grant's business. Pinned so the two rules cannot be conflated.
func TestRowPredicatesFor_ReadAllExemptsEntityScopeOnly(t *testing.T) {
	f, asked := grantScopeFactory(t, []spec.FilterSpec{{Field: "status", Op: "eq", Value: "paid"}})
	es := &spec.EntitySpec{
		Plural:   "orders",
		RowScope: []spec.FilterSpec{{Field: "branch_id", Op: "eq", From: "session", Attr: "branch_id"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "owner", WorkspaceID: "kafe", Roles: []string{"manajer"},
		Permissions: []string{"cafe-order.orders.read_all", "cafe-order.orders.list"},
	}))

	preds, err := f.rowPredicatesFor(req, es, "cafe-order", "order", "list")
	if err != nil {
		t.Fatalf("rowPredicatesFor: %v", err)
	}
	if *asked == "" {
		t.Fatal("the grant scope must still be asked for an owner — read_all exempts the ENTITY scope, not the role's own grant")
	}
	if len(preds) != 1 || preds[0].Field != "status" {
		t.Fatalf("predicates = %#v, want only the grant's status predicate", preds)
	}
}

// Guards the type relationship the API relies on: predicates are FilterOp-shaped
// so the storage layer has ONE operator implementation for client filters, entity
// scopes and grant scopes alike.
func TestRowPredicate_FilterOpFromDefaultsToEq(t *testing.T) {
	p := db.RowPredicate{Field: "status", Value: "paid"}
	if got := p.FilterOpFrom(); got.Op != "eq" || got.Value != "paid" {
		t.Fatalf("FilterOpFrom = %#v, want eq/paid", got)
	}
}
