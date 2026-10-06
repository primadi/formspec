// Cross-manifest scope-source validation (S5, kafe ledger 1.8).
//
// `scope:` declares that an entity's rows are partitioned along a dimension, and
// `row_scope: {from: session}` is what actually filters reads. The value of that
// filter comes from the caller's session — either a built-in identity attribute,
// an explicit `attr` (supplied by the token's `attrs` claim), or an `assignments`
// mapping declared somewhere in the spec tree (S5).
//
// This layer exists because the failure mode it prevents is invisible: a
// `from: session` entry written without `attr` on a scoped entity resolves to the
// scope field, and if nothing in the spec can ever produce that value then every
// read of the entity fails closed with 403 — forever, for everybody, with the
// manifest looking perfectly correct. There is no runtime signal to distinguish
// that from "the caller happens to be outside their branch".
//
// The rule is deliberately narrow to avoid false positives: an author who knows
// the attribute comes from the token spells out `attr:` and is left alone.
package main

import (
	"fmt"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// builtinSessionAttrs are identity attributes the engine can always resolve from
// the authenticated principal, without any application declaration.
var builtinSessionAttrs = map[string]bool{
	"principal_id": true,
	"user_id":      true,
	"username":     true,
	"workspace":    true,
	"workspace_id": true,
}

// sessionAttrSources is the set of attribute names some declaration in the tree
// can actually produce at request time.
//
// It exists because "a session attribute" is not self-evident: the value has to
// come from SOMEWHERE, and if nothing in the tree can supply it, then every read
// bounded by it fails closed 403 forever with a manifest that looks correct. Two
// declarations can supply one, and both spellings count because the engine
// resolves them interchangeably: an entity's `assignments` names a DIMENSION
// (`branch`) and the FIELD carrying its value (`branch_id`).
type sessionAttrSources struct {
	// names holds every dimension and field named by an `assignments` entry.
	names map[string]bool
}

// canSupply reports whether attr is a builtin identity attribute, or a name
// declared by an `assignments` mapping.
func (s sessionAttrSources) canSupply(attr string) bool {
	if attr == "" {
		return false
	}
	return builtinSessionAttrs[attr] || s.names[attr]
}

// collectSessionAttrSources scans the tree for every declared attribute source.
func collectSessionAttrSources(manifests []manifest.RawManifest) sessionAttrSources {
	src := sessionAttrSources{names: map[string]bool{}}
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || es == nil {
			continue // per-manifest validation reports the parse error
		}
		for _, a := range es.Assignments {
			if a.Dimension != "" {
				src.names[a.Dimension] = true
			}
			if a.Field != "" {
				src.names[a.Field] = true
			}
		}
	}
	return src
}

// validateScopeSources checks that every implicitly-resolved `from: session`
// row-scope attribute has a declared source. Returns manifest source → message.
func validateScopeSources(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}

	// Dimensions for which some entity declares a principal→value mapping.
	assigned := map[string]bool{}
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || es == nil {
			continue // per-manifest validation reports the parse error
		}
		for _, a := range es.Assignments {
			assigned[a.Dimension] = true
		}
	}

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || es == nil {
			continue
		}

		for i := range es.RowScope {
			sc := &es.RowScope[i]
			if sc.From != "session" || sc.Attr != "" {
				continue // explicit attr, or not a session scope
			}
			// Implicit form: the attribute is the entity's own scope field.
			attr := ""
			dimension := ""
			if es.Scope != nil {
				attr = es.Scope.Field
				dimension = es.Scope.Dimension
			}
			if attr == "" || builtinSessionAttrs[attr] {
				continue // resolves to principal_id, or a built-in
			}
			if dimension != "" && assigned[dimension] {
				continue // an `assignments` mapping supplies it
			}

			// Only one reject per manifest: the report shows a single message per
			// source, and a second one would be dropped anyway.
			if dimension != "" {
				rejects[m.Source] = fmt.Sprintf(
					"row_scope on %q resolves attribute %q from dimension %q, but no entity declares `assignments` for that dimension — every read would fail closed (403). Declare `assignments: [{dimension: %s, field: <field carrying the value>, principal_field: <field identifying the login>}]`, or name the token attribute explicitly with `attr: <name>`.",
					sc.Field, attr, dimension, dimension)
			} else {
				rejects[m.Source] = fmt.Sprintf(
					"row_scope on %q resolves attribute %q, which no entity provides: the entity declares no `scope` dimension and no `assignments` mapping exists. Name the token attribute explicitly (`attr: <name>`) or declare `scope` + `assignments`.",
					sc.Field, attr)
			}
			break
		}
	}

	return rejects
}
