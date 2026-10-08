package api

import (
	"net/http/httptest"
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
)

// `from: route` with `via` (kafe 10.76): the parameter is a REFERENCE to a
// record, and the scope value is read from that record. This is what lets a
// guest catalog be filtered by the branch of the table they scanned instead of
// by a `?branch_id=` the client computed.
func TestRowScopeVia_ResolvesThroughTheReferencedRecord(t *testing.T) {
	fx := newCreateScopeFixture(t)

	es := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{
			Field: "branch_id", Op: "eq", From: "route",
			Param: "session_id",
			Via:   "cafe-order.table-session", ViaField: "branch_id",
		}},
	}
	req := httptest.NewRequest("GET",
		"/kafe/_ui/entity/cafe-master/menu-item-price?session_id="+fx.sessionID, nil)
	req = req.WithContext(WithWorkspace(req.Context(), fx.ws))

	preds, err := fx.f.filterSpecsToPredicates(req, es, es.RowScope, "row scope")
	if err != nil {
		t.Fatalf("resolving the scope through the session failed: %v", err)
	}
	if len(preds) != 1 {
		t.Fatalf("predicates = %#v, want one", preds)
	}
	// The value must be the SESSION's branch — not the session id, and not
	// anything the caller supplied.
	if got := preds[0].Value; got != fx.branchA {
		t.Fatalf("scope value = %v, want the session's branch %s (branch B is %s)",
			got, fx.branchA, fx.branchB)
	}
	t.Logf("resolved %v -> branch %v", fx.sessionID, preds[0].Value)
}

// Fail-closed cases: a missing parameter, a reference that does not resolve, and
// a reference whose record carries no value for the field. Each must ERROR —
// never degrade into "no filter", which is the failure that makes a scope
// decorative.
func TestRowScopeVia_FailsClosed(t *testing.T) {
	fx := newCreateScopeFixture(t)
	es := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{
			Field: "branch_id", From: "route", Param: "session_id",
			Via: "cafe-order.table-session", ViaField: "branch_id",
		}},
	}

	cases := []struct{ name, query string }{
		{"missing parameter", ""},
		{"reference does not resolve", "?session_id=00000000-0000-7000-8000-000000000000"},
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", "/x"+c.query, nil)
		req = req.WithContext(WithWorkspace(req.Context(), fx.ws))
		if _, err := fx.f.filterSpecsToPredicates(req, es, es.RowScope, "row scope"); err == nil {
			t.Errorf("%s: expected a fail-closed error, got none", c.name)
		}
	}

	// A record that carries no value for the field cannot be verified either.
	empty := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{
			Field: "guest_token", From: "route", Param: "session_id",
			Via: "cafe-order.table-session", ViaField: "nonexistent_field",
		}},
	}
	req := httptest.NewRequest("GET", "/x?session_id="+fx.sessionID, nil)
	req = req.WithContext(WithWorkspace(req.Context(), fx.ws))
	if _, err := fx.f.filterSpecsToPredicates(req, empty, empty.RowScope, "row scope"); err == nil {
		t.Error("a record carrying no value for via_field must fail closed, got no error")
	}
}

// A session-sourced scope has no reference to follow, so `via` is meaningless
// there — the validator refuses it rather than letting it silently do nothing.
func TestRowScopeVia_RejectedOutsideRouteSource(t *testing.T) {
	if err := spec.ValidateRowScopeFilters("row_scope", []spec.FilterSpec{{
		Field: "branch_id", From: "session",
		Via: "cafe-order.table-session", ViaField: "branch_id",
	}}, nil); err == nil {
		t.Error("via on a session-sourced scope must be refused")
	}
	if err := spec.ValidateRowScopeFilters("row_scope", []spec.FilterSpec{{
		Field: "branch_id", From: "route", Param: "session_id", Via: "cafe-order.table-session",
	}}, nil); err == nil {
		t.Error("via without via_field must be refused")
	}
	if err := spec.ValidateRowScopeFilters("row_scope", []spec.FilterSpec{{
		Field: "branch_id", From: "route", Param: "session_id", ViaField: "branch_id",
	}}, nil); err == nil {
		t.Error("via_field without via must be refused")
	}
}

// The other half of 10.76: the SAME entity read by a signed-in cashier is scoped
// by their own session attribute, so the POS picker shows their branch's prices.
//
// Both halves are needed because one declaration cannot serve both surfaces:
// the guest's branch comes from the table they scanned (a `route` reference the
// entity cannot know), while the cashier's comes from their assignment. The
// entity's own `row_scope` is deliberately SKIPPED for an anonymous caller
// carrying a public grant scope — otherwise the guest could never be filtered,
// since there is no session attribute to read.
func TestRowScope_MenuItemPriceIsScopedForStaff(t *testing.T) {
	fx := newCreateScopeFixture(t)

	info, ok := fx.registry().GetEntity("cafe-master", "menu-item-price")
	if !ok || info.EntitySpec == nil {
		t.Fatal("kafe menu-item-price spec not found")
	}
	// The declaration itself: without a `row_scope`, the price list has NO
	// server-side narrowing for staff — a client-side filter was the
	// only thing there, and it does not even resolve on the POS surface.
	if len(info.EntitySpec.RowScope) == 0 {
		t.Fatal("menu-item-price must declare row_scope for staff — otherwise a cashier sees every branch's prices")
	}

	// A cashier: holds `list` (not `read_all`), with a branch attribute.
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-master/menu-item-price", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID:      "emp-1",
		WorkspaceID: fx.ws,
		Permissions: []string{"cafe-master.menu-item-prices.list"},
		Attributes:  map[string]string{"branch_id": fx.branchB},
	}))

	preds, err := fx.f.entityRowScopePredicates(req, info.EntitySpec, "cafe-master", "menu-item-price")
	if err != nil {
		t.Fatalf("the cashier's own branch must resolve: %v", err)
	}
	if len(preds) != 1 || preds[0].Value != fx.branchB {
		t.Fatalf("predicates = %#v, want a single branch_id = %s", preds, fx.branchB)
	}

	// Same entity, same request, but holding `read_all`: exempt by design —
	// "may see every branch" is an explicit grant, not an accident of the
	// wildcard (the exemption is asserted in TestApplyRowScope_ReadAllPermissionExempts;
	// this pins that the price entity is wired to it too).
	owner := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-master/menu-item-price", nil)
	owner = owner.WithContext(WithIdentity(owner.Context(), &auth.Identity{
		UserID:      "owner-1",
		WorkspaceID: fx.ws,
		Permissions: []string{"cafe-master.menu-item-prices.read_all"},
	}))
	preds, err = fx.f.entityRowScopePredicates(owner, info.EntitySpec, "cafe-master", "menu-item-price")
	if err != nil {
		t.Fatalf("a read_all holder must not fail closed: %v", err)
	}
	if len(preds) != 0 {
		t.Fatalf("a read_all holder must be unscoped, got %#v", preds)
	}
}
