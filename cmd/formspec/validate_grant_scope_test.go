package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// roleSeedManifest builds a Seed manifest declaring one role with one grant, so
// a test can state only the part it is about.
func roleSeedManifest(source string, grants []any) manifest.RawManifest {
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Seed",
		Metadata:   manifest.RawMetadata{Module: "formspec.core", Name: "roles"},
		Source:     source,
		Spec: map[string]any{
			"entities": []any{
				map[string]any{
					"entity": "role",
					"records": []any{
						map[string]any{"name": "barista", "grants": grants},
					},
				},
			},
		},
	}
}

// entityManifestWithAssignment declares the source of one session attribute, the
// way kafe's employee entity does.
func entityManifestWithAssignment(dimension, field string) manifest.RawManifest {
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Metadata:   manifest.RawMetadata{Module: "cafe-master", Name: "employee"},
		Source:     "employee.yaml",
		Spec: map[string]any{
			"version":        "v1",
			"characteristic": "master",
			"assignments": []any{
				map[string]any{"dimension": dimension, "field": field, "principal_field": "username"},
			},
			"fields": []any{
				map[string]any{"name": "username", "type": "string"},
				map[string]any{"name": field, "type": "string"},
			},
		},
	}
}

func sessionScopedGrant(field, attr string) []any {
	entry := map[string]any{"field": field, "op": "eq", "from": "session"}
	if attr != "" {
		entry["attr"] = attr
	}
	return []any{
		map[string]any{
			"page": "order-page",
			"actions": []any{
				map[string]any{"name": "list", "row_scope": []any{entry}},
			},
		},
	}
}

// The rule this reproduces (kafe 1.8, applied to grants): a session-bounded row
// restriction has to come from SOMEWHERE. Measured before the check existed:
// deleting the only `assignments` mapping from kafe's employee entity left the
// KDS grants with no possible source, the entity-level gate reported its TEN
// rejections, and every barista/dapur/pelayan list would have answered 403 with
// nothing at deploy time pointing at those three grants.
func TestValidateGrantScopeSources_NoSourceIsRejected(t *testing.T) {
	manifests := []manifest.RawManifest{roleSeedManifest("roles.yaml", sessionScopedGrant("branch_id", ""))}

	rejects := validateGrantScopeSources(manifests)
	msg := rejects["roles.yaml"]
	if msg == "" {
		t.Fatal("a grant bounding a row by a session attribute nothing can supply must be rejected at deploy time")
	}
	if !strings.Contains(msg, "barista") || !strings.Contains(msg, "branch_id") {
		t.Errorf("message must name the role and the attribute: %q", msg)
	}
	if !strings.Contains(msg, "assignments") || !strings.Contains(msg, "attr:") {
		t.Errorf("message must name both fixes: %q", msg)
	}
}

func TestValidateGrantScopeSources_DeclaredAssignmentIsAccepted(t *testing.T) {
	manifests := []manifest.RawManifest{
		entityManifestWithAssignment("branch", "branch_id"),
		roleSeedManifest("roles.yaml", sessionScopedGrant("branch_id", "")),
	}
	if rejects := validateGrantScopeSources(manifests); len(rejects) != 0 {
		t.Fatalf("a declared `assignments` mapping must satisfy the rule: %#v", rejects)
	}
}

// The dimension spelling is not the field spelling, and the engine resolves both
// (`dimension: branch` + `field: branch_id`). A gate that only accepted one of
// them would reject a correct manifest.
func TestValidateGrantScopeSources_AcceptsTheDimensionName(t *testing.T) {
	manifests := []manifest.RawManifest{
		entityManifestWithAssignment("branch", "branch_id"),
		roleSeedManifest("roles.yaml", sessionScopedGrant("branch", "")),
	}
	if rejects := validateGrantScopeSources(manifests); len(rejects) != 0 {
		t.Fatalf("the dimension name is a legitimate spelling: %#v", rejects)
	}
}

func TestValidateGrantScopeSources_BuiltinAttrIsAccepted(t *testing.T) {
	manifests := []manifest.RawManifest{roleSeedManifest("roles.yaml", sessionScopedGrant("username", ""))}
	if rejects := validateGrantScopeSources(manifests); len(rejects) != 0 {
		t.Fatalf("a builtin identity attribute needs no declaration: %#v", rejects)
	}
}

// An explicit `attr` is left alone — the same narrowness the entity rule has, and
// for the same reason: the value may be supplied by an external identity provider
// through the token's `attrs` claim, which no spec tree can enumerate.
func TestValidateGrantScopeSources_ExplicitAttrIsLeftAlone(t *testing.T) {
	manifests := []manifest.RawManifest{
		roleSeedManifest("roles.yaml", sessionScopedGrant("branch_id", "dimensi_dari_idp")),
	}
	if rejects := validateGrantScopeSources(manifests); len(rejects) != 0 {
		t.Fatalf("an explicitly named token attribute must not be second-guessed: %#v", rejects)
	}
}

// A literal predicate names no attribute, so the rule must not apply to it — this
// is the kafe kitchen's `status in paid,in_kitchen` half.
func TestValidateGrantScopeSources_LiteralPredicateIsNotAffected(t *testing.T) {
	grants := []any{
		map[string]any{
			"page": "order-page",
			"actions": []any{
				map[string]any{"name": "list", "row_scope": []any{
					map[string]any{"field": "status", "op": "in", "value": "paid,in_kitchen"},
				}},
			},
		},
	}
	if rejects := validateGrantScopeSources([]manifest.RawManifest{roleSeedManifest("roles.yaml", grants)}); len(rejects) != 0 {
		t.Fatalf("a literal row scope needs no assignment source: %#v", rejects)
	}
}

// Tab-level grants carry the same construct one level deeper; a walk that only
// read `actions` would pass vacuously.
func TestValidateGrantScopeSources_TabbedGrantIsChecked(t *testing.T) {
	grants := []any{
		map[string]any{
			"page": "sales",
			"tabs": []any{
				map[string]any{
					"tab": "Order",
					"actions": []any{
						map[string]any{"name": "list", "row_scope": []any{
							map[string]any{"field": "branch_id", "op": "eq", "from": "session"},
						}},
					},
				},
			},
		},
	}
	if rejects := validateGrantScopeSources([]manifest.RawManifest{roleSeedManifest("roles.yaml", grants)}); rejects["roles.yaml"] == "" {
		t.Fatal("a tab-level session scope must be checked too")
	}
}

// Other Seed entities are not role grants and must not be read as such.
func TestValidateGrantScopeSources_IgnoresNonRoleSeeds(t *testing.T) {
	m := roleSeedManifest("master.yaml", sessionScopedGrant("branch_id", ""))
	specMap := m.Spec.(map[string]any)
	entities := specMap["entities"].([]any)
	entities[0].(map[string]any)["entity"] = "branch"

	if rejects := validateGrantScopeSources([]manifest.RawManifest{m}); len(rejects) != 0 {
		t.Fatalf("only role records carry grants: %#v", rejects)
	}
}
