package auth

import (
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// kafe 10.60 (plan docs_internal/plan/via-sebagai-action-penuh.md L9): the
// grant footprint must derive its custom actions from the UNION
// (`ActionSources()`), not `es.Actions`.
//
// Why this is load-bearing (measured during L5): a transition's `via` IS an
// action since L3, and once the duplicated `actions:` entry is deleted (L4)
// a footprint reading `es.Actions` stops offering the action — so a role can
// no longer be granted the very action the UI shows as a button (kafe 10.47).
// This test pins the footprint read on a via-only action.
//
// Run with: go test ./internal/auth/ -run TestEntityFootprint_TransitionViaIsGrantable
func TestEntityFootprint_TransitionViaIsGrantable(t *testing.T) {
	m, reg := setupMaterializer(t)

	// Register an entity whose only custom action is a transition `via`. No
	// SyncSchema is needed: the footprint reads the registry's in-memory spec.
	if err := reg.RegisterArtifactManifest(manifest.RawManifest{
		Kind:     "Entity",
		Metadata: manifest.RawMetadata{Name: "dining-table", Module: "billing", Description: "test"},
		Source:   "test",
	}, &spec.EntitySpec{
		Version:        "v1",
		Plural:         "dining-tables",
		Characteristic: spec.CharMaster,
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString},
			{Name: "table_status", Type: spec.FieldString},
		},
		StateMachine: &spec.StateMachine{
			Field:   "table_status",
			Initial: "available",
			States:  []spec.StateDecl{{Name: "available"}, {Name: "occupied"}},
			Transitions: []spec.TransitionDecl{
				{From: spec.StateList{"available"}, To: "occupied", Action: "occupy",
					Impl: &spec.ImplDecl{Type: "script_ref", Ref: "cafe/occupy"}},
			},
		},
	}); err != nil {
		t.Fatalf("RegisterArtifactManifest: %v", err)
	}

	fp, err := m.entityFootprint("billing", "dining-table")
	if err != nil {
		t.Fatalf("entityFootprint: %v", err)
	}
	byAction := map[string]string{}
	for _, f := range fp {
		byAction[f.Action] = f.Permission
	}
	if got := byAction["occupy"]; got != "billing.dining-tables.occupy" {
		t.Fatalf("a transition-only `via` must be a grantable footprint action, got %q (footprint %v)", got, byAction)
	}

	// End-to-end through the strict materializer: a grant naming the `via` must
	// materialize to the permission the route requires.
	perms, err := m.Materialize([]Grant{{
		Page:    "dining-table-page",
		Actions: []ActionGrant{{Name: "occupy"}},
	}})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(perms) != 1 || perms[0] != "billing.dining-tables.occupy" {
		t.Fatalf("a grant naming a `via` must materialize, got %v", perms)
	}

	// Calibration: an action the entity does NOT expose must materialize to
	// nothing — otherwise the assertion above would pass for any name.
	none, err := m.Materialize([]Grant{{
		Page:    "dining-table-page",
		Actions: []ActionGrant{{Name: "nope"}},
	}})
	if err != nil {
		t.Fatalf("Materialize(nope): %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("an unknown action must not materialize, got %v", none)
	}
}
