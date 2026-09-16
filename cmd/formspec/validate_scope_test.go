package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// scopedEntity builds a raw Entity manifest that declares a scope dimension and
// a session row-scope on it — the shape S5 is about.
func scopedEntity(source, module, name, dimension, field string, rowScope []map[string]any) manifest.RawManifest {
	raw := make([]any, 0, len(rowScope))
	for _, rs := range rowScope {
		raw = append(raw, rs)
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: name, Module: module},
		Spec: map[string]any{
			"version": "v1",
			"fields": []any{
				map[string]any{"name": "branch_id", "type": "relation", "required": true},
				map[string]any{"name": "username", "type": "string"},
			},
			"scope":     map[string]any{"dimension": dimension, "field": field, "required": true},
			"row_scope": raw,
		},
	}
}

// TestValidateScopeSources_ImplicitAttributeNeedsASource pins the S5 honesty
// gate: a `from: session` row-scope written without `attr` resolves to the
// entity's scope field, and if no entity declares an `assignments` mapping for
// that dimension then every read fails closed 403 — forever, with the manifest
// looking correct. This is the failure the ledger keeps finding in other
// guises, so validation refuses it instead of trusting the author.
func TestValidateScopeSources_ImplicitAttributeNeedsASource(t *testing.T) {
	rowScope := map[string]any{"field": "branch_id", "from": "session", "op": "eq"}
	order := scopedEntity("order.yaml", "cafe-order", "order", "branch", "branch_id",
		[]map[string]any{rowScope})

	// 1. No assignments anywhere → reject, naming the missing declaration.
	rejects := validateScopeSources([]manifest.RawManifest{order})
	msg, ok := rejects["order.yaml"]
	if !ok {
		t.Fatal("expected a reject when no entity provides the dimension value")
	}
	for _, want := range []string{"branch", "assignments", "403"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message should mention %q, got: %s", want, msg)
		}
	}

	// 2. An employee entity declaring the mapping → accepted.
	employee := manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     "employee.yaml",
		Metadata:   manifest.RawMetadata{Name: "employee", Module: "cafe-master"},
		Spec: map[string]any{
			"version": "v1",
			"fields": []any{
				map[string]any{"name": "branch_id", "type": "relation", "required": true},
				map[string]any{"name": "username", "type": "string"},
			},
			"assignments": []any{
				map[string]any{"dimension": "branch", "field": "branch_id", "principal_field": "username"},
			},
		},
	}
	if rejects := validateScopeSources([]manifest.RawManifest{order, employee}); len(rejects) != 0 {
		t.Errorf("expected no reject once the dimension has a source, got %v", rejects)
	}

	// 3. An explicit `attr` is the author's assertion that the token supplies it —
	//    left alone, since the token is outside the spec tree.
	explicit := scopedEntity("order.yaml", "cafe-order", "order", "branch", "branch_id",
		[]map[string]any{{"field": "branch_id", "from": "session", "attr": "outlet_code"}})
	if rejects := validateScopeSources([]manifest.RawManifest{explicit}); len(rejects) != 0 {
		t.Errorf("explicit attr should not be second-guessed, got %v", rejects)
	}

	// 4. A route-scoped entry needs no declaration at all: the request carries it.
	routeScoped := scopedEntity("table-session.yaml", "cafe-order", "table-session", "branch", "branch_id",
		[]map[string]any{{"field": "guest_token", "from": "route", "param": "token"}})
	if rejects := validateScopeSources([]manifest.RawManifest{routeScoped}); len(rejects) != 0 {
		t.Errorf("route scope should not require assignments, got %v", rejects)
	}

	// 5. No scope declaration at all → the implicit attribute is principal_id,
	//    a built-in the engine always resolves.
	builtin := manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     "shift.yaml",
		Metadata:   manifest.RawMetadata{Name: "shift", Module: "cafe-order"},
		Spec: map[string]any{
			"version": "v1",
			"fields": []any{
				map[string]any{"name": "cashier_id", "type": "relation"},
			},
			"row_scope": []any{
				map[string]any{"field": "cashier_id", "from": "session"},
			},
		},
	}
	if rejects := validateScopeSources([]manifest.RawManifest{builtin}); len(rejects) != 0 {
		t.Errorf("built-in identity attribute should resolve without assignments, got %v", rejects)
	}
}
