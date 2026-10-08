package main

import (
	"fmt"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// validateIntakeOptIns enforces that an intake opt-in has an owner
// (plan docs_internal/plan/intake-challenge-pow.md).
//
// `challenge: true` on an action is a REQUEST to be gated; only an App can
// answer it. The policy — difficulty, when to escalate, TTL — lives in
// `App.spec.intake`, and resolution happens at router build from the public
// App's derived grants. An opt-in no public App covers can therefore never
// activate: the manifest would read as "protected" while the action ran with no
// gate at all, which is exactly the shape of failure this file exists to catch.
//
// It is a hard error rather than a warning for the same reason `public` without
// a rate limit is one: a declared-but-inert gate is indistinguishable from a
// working one at runtime, and nothing downstream can invent the policy the
// author did not write. (The per-app schema rules live in
// spec.ValidateAppSpec; this is the cross-manifest half.)
//
// The check is deliberately coarse — "does ANY public App declare an intake
// policy" — because deriving which App exposes which entity needs the UI
// registry, which this layer does not build. The imprecision is safe in one
// direction: an opt-in covered only by an App that does not actually expose the
// entity is harmless, since no anonymous route exists for it either way.
func validateIntakeOptIns(manifests []manifest.RawManifest) map[string]string {
	out := map[string]string{}

	hasPolicy := false
	for _, raw := range manifests {
		if spec.Kind(raw.Kind) != spec.KindApp {
			continue
		}
		specMap, ok := raw.Spec.(map[string]any)
		if !ok {
			continue
		}
		as, err := manifest.RawSpecToAppSpec(specMap)
		if err != nil || as == nil {
			continue
		}
		if as.Access == spec.AppAccessPublic && as.Intake != nil && as.Intake.Challenge != nil {
			hasPolicy = true
			break
		}
	}
	if hasPolicy {
		return out
	}

	for _, raw := range manifests {
		if spec.Kind(raw.Kind) != spec.KindEntity {
			continue
		}
		specMap, ok := raw.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || es == nil {
			continue
		}
		for i := range es.Actions {
			if es.Actions[i].Challenge {
				out[raw.Source] = fmt.Sprintf(
					"action %q declares `challenge: true` but no public App declares an `intake.challenge` policy — the opt-in can never activate; add `intake` to the exposed App, or drop the flag",
					es.Actions[i].Name)
				break
			}
		}
	}
	return out
}
