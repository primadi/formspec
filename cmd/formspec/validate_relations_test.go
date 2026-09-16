package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// Gaps #11/#12 (kafe 3.7): a relation that cannot resolve must be refused with
// the whole tree in view, because the runtime used to *skip* such a reference
// rather than reject it.
func TestValidateRelations(t *testing.T) {
	entity := func(module, name, category, relationResource string) manifest.RawManifest {
		specBody := map[string]any{"version": "v1", "characteristic": "transaction"}
		if category != "" {
			specBody["persist"] = map[string]any{"category": category}
		}
		fields := []any{map[string]any{"name": "id_target", "type": "string"}}
		if relationResource != "" {
			fields = append(fields, map[string]any{
				"name":     "link_id",
				"type":     "relation",
				"relation": map[string]any{"type": "belongs_to", "resource": relationResource},
			})
		}
		specBody["fields"] = fields
		return manifest.RawManifest{
			APIVersion: "formspec.dev/v1",
			Kind:       "Entity",
			Source:     module + "/" + name + ".yaml",
			Metadata:   manifest.RawMetadata{Name: name, Module: module},
			Spec:       specBody,
		}
	}

	t.Run("cross-module target that resolves is accepted", func(t *testing.T) {
		rejects := validateRelations([]manifest.RawManifest{
			entity("cafe-master", "branch", "", ""),
			entity("cafe-order", "order", "", "cafe-master.branch"), // dotted form
		})
		if len(rejects) != 0 {
			t.Fatalf("expected no rejects, got %v", rejects)
		}
	})

	t.Run("unregistered target is refused (gap #12)", func(t *testing.T) {
		rejects := validateRelations([]manifest.RawManifest{
			entity("cafe-order", "order", "", "cafe-master.branch"), // nothing declares it
		})
		msg, ok := rejects["cafe-order/order.yaml"]
		if !ok {
			t.Fatal("expected a reject for an unresolvable relation target")
		}
		for _, want := range []string{"cafe-master.branch", "not a registered entity", "silently"} {
			if !strings.Contains(msg, want) {
				t.Errorf("message should mention %q, got: %s", want, msg)
			}
		}
	})

	t.Run("cross-category relation is refused (gap #11)", func(t *testing.T) {
		rejects := validateRelations([]manifest.RawManifest{
			entity("gl", "journal-entry", "financial", ""),
			entity("cafe-order", "order", "operational", "gl.journal-entry"),
		})
		msg, ok := rejects["cafe-order/order.yaml"]
		if !ok {
			t.Fatal("expected a reject for a cross-category relation")
		}
		if !strings.Contains(msg, "crosses persist categories") {
			t.Errorf("message should name the cross-category problem, got: %s", msg)
		}
	})

	t.Run("an entity without a category may still point at one", func(t *testing.T) {
		rejects := validateRelations([]manifest.RawManifest{
			entity("gl", "account", "financial", ""),
			entity("cafe-order", "order", "", "gl.account"), // no category declared
		})
		if len(rejects) != 0 {
			t.Fatalf("a category-less entity must not be treated as a different category, got %v", rejects)
		}
	})
}
