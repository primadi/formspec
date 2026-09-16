package api

import (
	"context"
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

	got, err := f.applyRowScope(req, &spec.EntitySpec{}, "cafe-order", "order", nil)
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
		RowScope: []spec.FilterSpec{{Field: "branch_id", From: "session", Attr: "branch_id"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order?branch_id[eq]=OTHER", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID:     "u1",
		Attributes: map[string]string{"branch_id": "KFE-JKT-01"},
	}))

	// The client tried to widen the view by filtering on another branch.
	got, err := f.applyRowScope(req, es, "cafe-order", "order", map[string]db.FilterOp{
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
		RowScope: []spec.FilterSpec{{Field: "branch_id", From: "session", Attr: "branch_id"}},
	}
	// Identity without the attribute (no assignment) must not degrade into
	// "list everything".
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "u1"}))

	if _, err := f.applyRowScope(req, es, "cafe-order", "order", nil); err == nil {
		t.Fatal("expected fail-closed error when the identity has no branch_id attribute")
	}
}

func TestApplyRowScope_AnonymousWithSessionScopeFailsClosed(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{Field: "branch_id", From: "session", Attr: "branch_id"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)

	if _, err := f.applyRowScope(req, es, "cafe-order", "order", nil); err == nil {
		t.Fatal("expected fail-closed error for an anonymous caller on a session-scoped entity")
	}
}

func TestApplyRowScope_RouteParamScopesByToken(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{Field: "guest_token", From: "route", Param: "token"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order?token=abc123", nil)

	got, err := f.applyRowScope(req, es, "cafe-order", "order", nil)
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
		RowScope: []spec.FilterSpec{{Field: "guest_token", From: "route", Param: "token"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)

	if _, err := f.applyRowScope(req, es, "cafe-order", "order", nil); err == nil {
		t.Fatal("expected fail-closed error when the declared route parameter is missing")
	}
}

// TestApplyRowScope_ReadAllPermissionExempts pins the decision that "may see
// every branch" is an EXPLICIT permission, not an implicit wildcard rule. Two
// callers that look alike are separated:
//
//   - a workspace owner holding `{module}.{plural}.read_all` has no branch of
//     their own by design, so scoping would fail closed on every read;
//   - a cashier holding only `.list` is still scoped.
//
// `*` satisfies the check too (dev super-admin), which is why the exemption is
// written against the same HasPermission used everywhere else.
func TestApplyRowScope_ReadAllPermissionExempts(t *testing.T) {
	es := &spec.EntitySpec{
		Plural:   "orders",
		Scope:    &spec.ScopeDecl{Dimension: "branch", Field: "branch_id"},
		RowScope: []spec.FilterSpec{{Field: "branch_id", From: "session"}},
	}

	owner := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	owner = owner.WithContext(WithIdentity(owner.Context(), &auth.Identity{
		UserID:      "owner-1",
		Permissions: []string{"cafe-order.orders.read_all"},
	}))
	got, err := (&HandlerFactory{}).applyRowScope(owner, es, "cafe-order", "order", nil)
	if err != nil {
		t.Fatalf("read_all holder must not fail closed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("read_all holder must be unscoped, got %#v", got)
	}

	// Same entity, same missing attribute — but no read_all: still fail closed.
	cashier := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	cashier = cashier.WithContext(WithIdentity(cashier.Context(), &auth.Identity{
		UserID:      "emp-1",
		Permissions: []string{"cafe-order.orders.list"},
	}))
	if _, err := (&HandlerFactory{}).applyRowScope(cashier, es, "cafe-order", "order", nil); err == nil {
		t.Fatal("a cashier without read_all must still be scoped")
	}
}

// The permission name follows the canonical `{module}.{plural}.{action}` form
// (D5), using the entity's declared plural, and the `{entity}s` fallback when it
// is absent — the same rule the registry and route generator use.
func TestReadAllPermission(t *testing.T) {
	if got := ReadAllPermission("cafe-order", "order", &spec.EntitySpec{Plural: "orders"}); got != "cafe-order.orders.read_all" {
		t.Fatalf("got %q", got)
	}
	if got := ReadAllPermission("cafe-order", "shift", &spec.EntitySpec{}); got != "cafe-order.shifts.read_all" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyRowScope_SessionPrincipalId(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{Field: "cashier_id", From: "session"}}, // attr defaults to principal_id
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/shift", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "emp-7"}))

	got, err := f.applyRowScope(req, es, "cafe-order", "order", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["cashier_id"].Value != "emp-7" {
		t.Fatalf("cashier_id scope = %v, want emp-7", got["cashier_id"].Value)
	}
}

// fakeAssignments stands in for *entity.Registry: it answers the S5
// principal→dimension lookups the way a real assignments declaration would.
type fakeAssignments struct {
	sources []spec.AssignmentSource
	values  map[string]string // "entity|principal" → value
}

func (f *fakeAssignments) GetEntityStore(module, name string) (*db.EntityStore, error) {
	return nil, nil // unused — assignments resolve through FindAssignmentValue
}

func (f *fakeAssignments) AssignmentSources() []spec.AssignmentSource { return f.sources }

func (f *fakeAssignments) FindAssignmentValue(_ context.Context, src spec.AssignmentSource, _, principal string) (string, error) {
	return f.values[src.Entity+"|"+principal], nil
}

// TestApplyRowScope_AttributeFromAssignments pins the S5 value source: a
// `from: session` scope whose attribute the token does not carry is resolved
// from the entity that declares the principal→dimension mapping
// (`employee.username` → `employee.branch_id` for kafe). Without this, the kafe
// spec can declare a branch scope but nothing could ever produce the value, so
// every read of a scoped entity would fail closed.
func TestApplyRowScope_AttributeFromAssignments(t *testing.T) {
	resetScopeAttrCache()

	f := &HandlerFactory{registry: &fakeAssignments{
		sources: []spec.AssignmentSource{{
			Module: "cafe-master", Entity: "employee",
			Dimension: "branch", Field: "branch_id", PrincipalField: "username",
		}},
		values: map[string]string{"employee|sari": "KFE-JKT-01"},
	}}
	// No `attr`: the implicit form resolves to the entity's scope field.
	es := &spec.EntitySpec{
		Scope:    &spec.ScopeDecl{Dimension: "branch", Field: "branch_id", Required: true},
		RowScope: []spec.FilterSpec{{Field: "branch_id", From: "session"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{
		UserID: "u1", Username: "sari", WorkspaceID: "kafe",
	}))

	got, err := f.applyRowScope(req, es, "cafe-order", "order", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["branch_id"].Value != "KFE-JKT-01" {
		t.Fatalf("branch_id scope = %v, want KFE-JKT-01", got["branch_id"].Value)
	}

	// A principal with no assignment still fails closed — an unresolvable value
	// must never widen the view.
	other := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)
	other = other.WithContext(WithIdentity(other.Context(), &auth.Identity{
		UserID: "u2", Username: "unassigned", WorkspaceID: "kafe",
	}))
	if _, err := f.applyRowScope(other, es, "cafe-order", "order", nil); err == nil {
		t.Fatal("expected fail-closed for a principal with no assignment")
	}
}
