package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

func TestValidateSeedRoleGrants_RejectsWrongKeyAndShape(t *testing.T) {
	seed := map[string]any{
		"entities": []any{
			map[string]any{
				"entity": "role",
				"records": []any{
					map[string]any{
						"name": "kasir",
						"grants": []any{
							map[string]any{
								"page": "order-page",
								"actions": []any{
									map[string]any{
										"name": "list",
										"row_scopes": []any{
											map[string]any{"field": "status", "op": "in", "value": "paid"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	manifests := []manifest.RawManifest{{
		APIVersion: "formspec.dev/v1",
		Kind:       "Seed",
		Metadata:   manifest.RawMetadata{Module: "formspec.core", Name: "test-role-grants"},
		Spec:       seed,
		Source:     "seed.yaml",
	}}

	rejects := validateSeedRoleGrants(manifests)
	if len(rejects) == 0 {
		t.Fatal("validateSeedRoleGrants accepted a malformed role grant with row_scopes")
	}
	// The message has to say WHICH key and that the restriction is the part that
	// was lost — this defect is fail-open (the permission survives), so "a grant
	// was dropped" would point the operator at the wrong problem entirely.
	msg := rejects["seed.yaml"]
	if !strings.Contains(msg, "row_scope") || !strings.Contains(msg, "row restriction") {
		t.Fatalf("message = %q, want it to name `row_scope` and the lost row restriction", msg)
	}
}

// A grant whose action has no name resolves to nothing, but it is NOT a
// row-scope defect: it costs the grant and cannot leave a boundary behind, so
// the runtime must not deny an unrelated permission because of it.
func TestValidateSeedRoleGrants_ReportsNamelessActionWithoutRowScopeFlag(t *testing.T) {
	seed := map[string]any{
		"entities": []any{
			map[string]any{
				"entity": "role",
				"records": []any{
					map[string]any{
						"name": "kasir",
						"grants": []any{
							map[string]any{
								"page":    "order-page",
								"actions": []any{map[string]any{"row_scope": []any{}}},
							},
						},
					},
				},
			},
		},
	}
	manifests := []manifest.RawManifest{{
		APIVersion: "formspec.dev/v1",
		Kind:       "Seed",
		Metadata:   manifest.RawMetadata{Module: "formspec.core", Name: "nameless-action"},
		Spec:       seed,
		Source:     "nameless.yaml",
	}}

	msg := validateSeedRoleGrants(manifests)["nameless.yaml"]
	if msg == "" {
		t.Fatal("a nameless action must be reported")
	}
	if strings.Contains(msg, "row restriction") {
		t.Fatalf("message = %q — a missing action name is not a row-restriction defect", msg)
	}
}

func TestValidateSeedRoleGrants_AcceptsValidRoleGrantScope(t *testing.T) {
	seed := map[string]any{
		"entities": []any{
			map[string]any{
				"entity": "role",
				"records": []any{
					map[string]any{
						"name": "barista",
						"grants": []any{
							map[string]any{
								"page": "order-page",
								"actions": []any{
									map[string]any{
										"name": "list",
										"row_scope": []any{
											map[string]any{"field": "status", "op": "in", "value": "paid"},
											map[string]any{"field": "branch_id", "op": "eq", "from": "session"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	manifests := []manifest.RawManifest{{
		APIVersion: "formspec.dev/v1",
		Kind:       "Seed",
		Metadata:   manifest.RawMetadata{Module: "formspec.core", Name: "valid-role-grants"},
		Spec:       seed,
		Source:     "valid-seed.yaml",
	}}

	if rejects := validateSeedRoleGrants(manifests); len(rejects) != 0 {
		t.Fatalf("validateSeedRoleGrants rejected a valid row_scope grant: %#v", rejects)
	}
}
