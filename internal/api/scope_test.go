package api

import (
	"net/http/httptest"
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// applyRowScope is the server-side guarantee behind S2 (#6/#9): unlike a kind's
// `fixed_filters`, which the browser merges and a client can omit, the entity's
// `scope` is resolved from the request context and overrides client values.
// These tests pin the two properties that make it an authorization control
// rather than a convenience: session values never come from the client, and an
// unresolvable scope fails closed instead of listing unscoped rows.

func TestApplyRowScope_NoScopeLeavesFiltersUntouched(t *testing.T) {
	f := &HandlerFactory{}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)

	got, err := f.applyRowScope(req, &spec.EntitySpec{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected filters untouched, got %#v", got)
	}
}

func TestApplyRowScope_SessionOverridesClientValue(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		Scope: []spec.FilterSpec{{Field: "branch_id", From: "session", Attr: "branch_id"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order?branch_id[eq]=OTHER", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID:     "u1",
		Attributes: map[string]string{"branch_id": "KFE-JKT-01"},
	}))

	// The client tried to widen the view by filtering on another branch.
	got, err := f.applyRowScope(req, es, map[string]db.FilterOp{
		"branch_id": {Op: "eq", Value: "OTHER"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["branch_id"].Value != "KFE-JKT-01" {
		t.Fatalf("client value was not overridden: got %v, want KFE-JKT-01", got["branch_id"].Value)
	}
}

func TestApplyRowScope_SessionAttributeMissingFailsClosed(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		Scope: []spec.FilterSpec{{Field: "branch_id", From: "session", Attr: "branch_id"}},
	}
	// Identity without the attribute (no assignment) must not degrade into
	// "list everything".
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "u1"}))

	if _, err := f.applyRowScope(req, es, nil); err == nil {
		t.Fatal("expected fail-closed error when the identity has no branch_id attribute")
	}
}

func TestApplyRowScope_AnonymousWithSessionScopeFailsClosed(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		Scope: []spec.FilterSpec{{Field: "branch_id", From: "session", Attr: "branch_id"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)

	if _, err := f.applyRowScope(req, es, nil); err == nil {
		t.Fatal("expected fail-closed error for an anonymous caller on a session-scoped entity")
	}
}

func TestApplyRowScope_RouteParamScopesByToken(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		Scope: []spec.FilterSpec{{Field: "guest_token", From: "route", Param: "token"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order?token=abc123", nil)

	got, err := f.applyRowScope(req, es, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["guest_token"].Value != "abc123" {
		t.Fatalf("guest_token scope = %v, want abc123", got["guest_token"].Value)
	}
}

func TestApplyRowScope_RouteParamMissingFailsClosed(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		Scope: []spec.FilterSpec{{Field: "guest_token", From: "route", Param: "token"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)

	if _, err := f.applyRowScope(req, es, nil); err == nil {
		t.Fatal("expected fail-closed error when the declared route parameter is missing")
	}
}

func TestApplyRowScope_SessionPrincipalId(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		Scope: []spec.FilterSpec{{Field: "cashier_id", From: "session"}}, // attr defaults to principal_id
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/shift", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "emp-7"}))

	got, err := f.applyRowScope(req, es, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["cashier_id"].Value != "emp-7" {
		t.Fatalf("cashier_id scope = %v, want emp-7", got["cashier_id"].Value)
	}
}
