package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// insertRoleWithGrants writes a role row directly, so the grants value reaches
// the resolver exactly as an operator (or a seed) would store it — free JSON,
// not a typed struct.
func insertRoleWithGrants(t *testing.T, reg interface {
	GetEntityStore(module, name string) (*db.EntityStore, error)
}, name string, grants any) {
	t.Helper()
	roleStore, err := reg.GetEntityStore(CoreModule, "role")
	if err != nil {
		t.Fatalf("role store: %v", err)
	}
	if _, err := roleStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{"name": name, "app": "", "grants": grants},
	}); err != nil {
		t.Fatalf("insert role %s: %v", name, err)
	}
}

// GrantScope must DENY, not degrade, when a row restriction was declared and
// cannot be applied.
//
// This is the kafe 10.67 failure class turned into an assertion. The dangerous
// property is that the two halves fail in opposite directions: an unresolvable
// PAGE/ACTION only costs the caller a permission (fail closed by itself), while
// an unreadable ROW RESTRICTION leaves the permission intact and the boundary
// gone (fail open). Serving the request with fewer predicates is precisely the
// leak the restriction existed to prevent, so an error must come back and the
// HTTP layer must turn it into a refusal.
//
// The negative control is the misspelled key: `row_scopes` is invisible to the
// typed read, so without this path the resolver would return an EMPTY scope —
// indistinguishable from "this role is not restricted".
func TestGrantScope_UnreadableRowRestrictionDenies(t *testing.T) {
	resolver, reg, _ := setupResolver(t)
	ctx := context.Background()

	insertRoleWithGrants(t, reg, "dapur", []map[string]any{
		{"page": "order-list", "actions": []map[string]any{
			{"name": "list", "row_scopes": []map[string]any{
				{"field": "name", "op": "eq", "value": "paid"},
			}},
		}},
	})

	scope, err := resolver.GrantScope(ctx, "demo", "", []string{"dapur"}, "billing.orders.list")
	if err == nil {
		t.Fatalf("expected a denial for an unreadable row restriction, got scope %#v", scope)
	}
	if scope != nil {
		t.Fatalf("a denied lookup must not return a scope, got %#v", scope)
	}
	if !strings.Contains(err.Error(), "dapur") {
		t.Errorf("error must name the offending role: %v", err)
	}
}

// A row restriction on a field the target entity does not declare is the same
// class: the predicate cannot hold, so the permission is refused rather than
// granted with a filter that matches nothing (or, worse, silently no filter).
func TestGrantScope_UnknownFieldDenies(t *testing.T) {
	resolver, reg, _ := setupResolver(t)
	ctx := context.Background()

	insertRoleWithGrants(t, reg, "dapur", []map[string]any{
		{"page": "order-list", "actions": []map[string]any{
			{"name": "list", "row_scope": []map[string]any{
				{"field": "status", "op": "in", "value": "paid"}, // not a field on billing.order here
			}},
		}},
	})

	if _, err := resolver.GrantScope(ctx, "demo", "", []string{"dapur"}, "billing.orders.list"); err == nil {
		t.Fatal("a row restriction on an undeclared field must deny the permission")
	}
}

// The happy path must stay untouched: a readable restriction is returned, and a
// permission nothing restricts returns (nil, nil) — NOT an error, or every
// ordinary caller would be locked out.
func TestGrantScope_ReturnsScopeAndNilWhenUnrestricted(t *testing.T) {
	resolver, reg, _ := setupResolver(t)
	ctx := context.Background()

	insertRoleWithGrants(t, reg, "barista", []map[string]any{
		{"page": "order-list", "actions": []map[string]any{
			{"name": "list", "row_scope": []map[string]any{
				{"field": "name", "op": "eq", "value": "paid"},
			}},
			{"name": "view"},
		}},
	})

	scope, err := resolver.GrantScope(ctx, "demo", "", []string{"barista"}, "billing.orders.list")
	if err != nil {
		t.Fatalf("readable restriction denied: %v", err)
	}
	if len(scope) != 1 || scope[0].Field != "name" || scope[0].Value != "paid" {
		t.Fatalf("scope = %#v, want the declared predicate", scope)
	}

	// `view` carries no scope: no error, no predicates.
	unrestricted, err := resolver.GrantScope(ctx, "demo", "", []string{"barista"}, "billing.orders.view")
	if err != nil {
		t.Fatalf("an unrestricted permission must not error: %v", err)
	}
	if len(unrestricted) != 0 {
		t.Fatalf("view scope = %#v, want none", unrestricted)
	}
}

// A grant that costs only a PERMISSION (a misspelled page, a nameless action)
// must not deny an unrelated permission: that defect fails closed by itself, and
// treating it as a row-scope problem would turn a typo into an outage.
func TestGrantScope_GrantOnlyDefectDoesNotDeny(t *testing.T) {
	resolver, reg, _ := setupResolver(t)
	ctx := context.Background()

	insertRoleWithGrants(t, reg, "typo-role", []map[string]any{
		{"page": "order-list", "actions": []map[string]any{
			{"name": "list", "row_scope": []map[string]any{
				{"field": "name", "op": "eq", "value": "paid"},
			}},
			{"name": "view"},
		}},
		// This page does not exist; it contributes nothing and must not take the
		// readable `order-list` grant down with it (10.53).
		{"page": "tidak-ada-page", "actions": []map[string]any{{"name": "list"}}},
	})

	scope, err := resolver.GrantScope(ctx, "demo", "", []string{"typo-role"}, "billing.orders.list")
	if err != nil {
		t.Fatalf("an unresolvable sibling grant must not deny a readable one: %v", err)
	}
	if len(scope) != 1 {
		t.Fatalf("scope = %#v, want the one declared predicate", scope)
	}
	_ = spec.FilterSpec{}
}
