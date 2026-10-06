// Cross-manifest Subscription event validation.
//
// A `kind: Subscription` names the events it reacts to as fully-qualified
// `{module}.{entity}.{event}` strings. Nothing checked those names, so a typo —
// or a renamed event — produced a subscription that NEVER fires: the manifest
// looks configured, `formspec validate` stays green, and the reaction simply
// never happens. That is the same silent-nothing class the integrator and
// delivery-target checks already close, applied to the consumer side.
//
// Two things keep it from crying wolf:
//
//  1. The reserved lifecycle events (`before_{action}` / `on_{action}` for the
//     eight reserved actions) are IMPLIED per entity, not declared — a
//     subscription may legitimately listen to `on_submit` without anyone writing
//     it in `events:`. They are added to the known set.
//  2. If the referenced entity is not in this spec tree (a module shipped
//     separately), the name cannot be verified — skipped rather than rejected,
//     the same rule the delivery-target check uses.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// validateSubscriptionEvents resolves every Subscription's `events:` entry
// against the events the tree can emit. Returns source path -> message.
func validateSubscriptionEvents(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}

	// Known event names, and the set of entities present (so the "not in this
	// tree" case can be told apart from "entity is here but the event is not").
	known := map[string]bool{}
	entities := map[string]bool{}

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil {
			continue
		}
		prefix := m.Metadata.Module + "." + m.Metadata.Name
		entities[prefix] = true

		for _, e := range es.Events {
			known[prefix+"."+e.Name] = true
		}
		// Reserved lifecycle events every entity has, without declaring them
		// (Core §7: "tiap reserved action punya before_{action} dan on_{action}
		// berpasangan tanpa perlu dideklarasikan manual").
		for _, a := range spec.ReservedActionNames {
			known[prefix+".before_"+a] = true
			known[prefix+".on_"+a] = true
		}
	}

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindSubscription || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		sub, err := manifest.RawSpecTo[spec.SubscriptionSpec](specMap)
		if err != nil || sub == nil {
			continue
		}
		for _, ref := range sub.Events {
			name := strings.TrimSpace(ref)
			if name == "" {
				rejects[m.Source] = "subscription has an empty entry in `events:` — it would never fire"
				break
			}
			// The entity part is everything before the last segment.
			i := strings.LastIndex(name, ".")
			if i <= 0 {
				rejects[m.Source] = fmt.Sprintf(
					"subscription event %q must be fully qualified `{module}.{entity}.{event}` — a short name cannot be resolved, so the subscription would never fire", name)
				break
			}
			entity := name[:i]
			if !entities[entity] {
				continue // entity not in this tree — cannot verify, do not reject
			}
			if !known[name] {
				rejects[m.Source] = fmt.Sprintf(
					"subscription listens to %q, but %s declares no event named %q — the subscription would never fire (declared: %s)",
					name, entity, name[i+1:], describeEntityEvents(known, entity))
				break
			}
		}
	}

	return rejects
}

// describeEntityEvents lists the event names known for one entity, so the error
// shows what could have been meant instead of only what was wrong.
func describeEntityEvents(known map[string]bool, entity string) string {
	prefix := entity + "."
	var names []string
	for k := range known {
		if strings.HasPrefix(k, prefix) {
			names = append(names, strings.TrimPrefix(k, prefix))
		}
	}
	if len(names) == 0 {
		return "none"
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
