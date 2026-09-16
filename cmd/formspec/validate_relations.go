// Cross-manifest relation validation (kafe ledger 3.7 / gaps #11 + #12).
//
// Two silent failures live here, and both come from the same root: a relation
// whose target does not resolve was treated as "nothing to check" rather than as
// a defect.
//
//  1. (#12) Relation target resolution used the *owning* entity's module plus a
//     naive `+ "s"` plural. For a cross-module relation (`cafe-order` →
//     `cafe-master`), that produces a table name that does not exist — and the
//     referenceability guard then silently skipped the record instead of
//     rejecting it. Order rows could point at a branch that does not exist.
//  2. (#11) A relation crossing `persist.category` boundaries (operational →
//     financial) is blocked at runtime by design, but only with a log line: the
//     alias simply is not populated, and a list that "works" hides that the
//     relation never resolves.
//
// Both are answerable statically, with the whole spec tree in view — so they are
// refused here, before a deployment depends on them.
package main

import (
	"fmt"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// relationTargets indexes the registered entities by "module/entity" so a
// relation reference can be resolved for real instead of guessed.
type relationTargets map[string]relationTargetInfo

type relationTargetInfo struct {
	// Category is the target's persist category ("" when it declares none).
	Category string
	// Source is the manifest that declared it, for the error message.
	Source string
}

func buildRelationTargets(manifests []manifest.RawManifest) relationTargets {
	out := relationTargets{}
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(sm)
		if err != nil || es == nil {
			continue
		}
		category := ""
		if es.Persist != nil {
			category = es.Persist.Category
		}
		out[m.Metadata.Module+"/"+m.Metadata.Name] = relationTargetInfo{Category: category, Source: m.Source}
	}
	return out
}

// validateRelations resolves every `relation.resource` against the registered
// entities and refuses a target that is missing or in another category.
// Returns manifest source → message (one per manifest, as the report prints one).
func validateRelations(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}
	targets := buildRelationTargets(manifests)

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(sm)
		if err != nil || es == nil {
			continue
		}
		ownCategory := ""
		if es.Persist != nil {
			ownCategory = es.Persist.Category
		}

		if msg := checkRelationFields(m, es, ownCategory, targets); msg != "" {
			rejects[m.Source] = msg
		}
	}
	return rejects
}

// checkRelationFields walks the entity's own fields *and* its child fields —
// a child row carries relations too, and those are the ones a picker writes.
func checkRelationFields(m manifest.RawManifest, es *spec.EntitySpec, ownCategory string, targets relationTargets) string {
	check := func(f spec.Field, where string) string {
		if f.Relation == nil || f.Relation.Resource == "" {
			return ""
		}
		ref, ok := spec.NormalizeEntityRef(f.Relation.Resource)
		if !ok {
			// Same-module short form: qualify with the owning module, exactly like
			// the runtime does.
			ref = m.Metadata.Module + "/" + f.Relation.Resource
		}
		target, found := targets[ref]
		if !found {
			return fmt.Sprintf(
				"%s: relation %s targets %q, which is not a registered entity in this spec tree — the referenceability guard cannot resolve it, so a dangling reference would pass silently",
				where, f.Name, f.Relation.Resource)
		}
		if ownCategory != "" && target.Category != "" && target.Category != ownCategory {
			return fmt.Sprintf(
				"%s: relation %s crosses persist categories (%s → %s) — a cross-category JOIN is blocked by design, so this relation can never resolve",
				where, f.Name, ownCategory, target.Category)
		}
		return ""
	}

	for _, f := range es.Fields {
		if msg := check(f, m.Metadata.Name); msg != "" {
			return msg
		}
		if f.Child != nil {
			for _, cf := range f.Child.Fields {
				if msg := check(cf, m.Metadata.Name+"."+f.Name); msg != "" {
					return msg
				}
			}
		}
	}
	return ""
}
