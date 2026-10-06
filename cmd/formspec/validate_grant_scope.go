package main

import (
	"fmt"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// validateGrantScopeSources holds a role grant's `from: session` row scope to the
// SAME rule the entity's own `row_scope` is held to (kafe 1.8, S5).
//
// Why it needs its own check. The entity-level gate (validateScopeSources) walks
// Entity manifests only, so a grant could name a session attribute that nothing
// in the tree can ever produce and both deploy-time commands stayed green —
// measured: deleting the `assignments` mapping from kafe's employee entity left
// the KDS grant's `{field: branch_id, from: session}` with no source at all, and
// `formspec validate` reported the TEN entity row_scope rejections while saying
// nothing about the three barista/dapur/pelayan grants that would now answer 403
// to every list. The operator would have learned the cause only from a runtime
// refusal.
//
// Deliberately NARROW, mirroring the entity rule: only the IMPLICIT form is
// judged (no explicit `attr`). An author who spells out `attr:` knows where the
// attribute comes from — it may be supplied by an external identity provider
// through the token's `attrs` claim — so they are left alone.
//
// The implicit form resolves, at runtime, to the TARGET ENTITY's `scope.field`.
// Deploy time has no page→entity registry here, so the entry's own `field` is
// used as the candidate name: for a session-bounded restriction that is the
// scope field by construction (comparing any other column to the session's
// branch would be meaningless), and when it is not, the message names the fix
// (spell out `attr:`).
func validateGrantScopeSources(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}
	sources := collectSessionAttrSources(manifests)

	record := func(source, format string, args ...any) {
		if _, exists := rejects[source]; !exists {
			rejects[source] = fmt.Sprintf(format, args...)
		}
	}

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindSeed || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		seed, err := manifest.RawSpecTo[spec.SeedSpec](specMap)
		if err != nil || seed == nil {
			continue
		}
		for _, entity := range seed.Entities {
			if !isRoleEntityName(entity.Entity) {
				continue
			}
			for _, rec := range entity.Records {
				roleName, _ := rec["name"].(string)
				if roleName == "" {
					roleName = "<unnamed>"
				}
				for _, action := range grantActionsOf(rec["grants"]) {
					for _, entry := range action.scopes {
						// Explicit attr: the author named the source. Left alone,
						// exactly as the entity rule does.
						if entry.attr != "" {
							continue
						}
						if sources.canSupply(entry.field) {
							continue
						}
						record(m.Source,
							"seed role %q: grant row_scope on %s/%s bounds %q from the session, but nothing in the spec can supply that attribute — every read would fail closed (403). Declare `assignments: [{dimension: <name>, field: %s, principal_field: <field identifying the login>}]` on the entity that carries the value, or name the token attribute explicitly with `attr: <name>`.",
							roleName, action.page, action.name, entry.field, entry.field)
					}
				}
			}
		}
	}
	return rejects
}

// grantScopeEntry is one `from: session` row-scope entry of a grant action,
// reduced to the three things the source rule needs.
type grantScopeEntry struct {
	field string
	attr  string
}

// grantAction is one (page, action) pair of a raw `grants` value.
type grantAction struct {
	page   string
	name   string
	scopes []grantScopeEntry
}

// grantActionsOf flattens a raw `grants` value into the actions it declares.
//
// It walks the raw value rather than the typed `[]Grant` for the same reason the
// shape gate does: the typed read drops keys it does not know, so a misspelled
// `row_scope` would silently produce no entries and this check would pass
// vacuously. Shape defects are reported by validateSeedRoleGrants; here only
// well-formed entries are read, and a malformed one contributes nothing.
func grantActionsOf(raw any) []grantAction {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []grantAction
	for _, rawGrant := range list {
		grant, ok := rawGrant.(map[string]any)
		if !ok {
			continue
		}
		page, _ := grant["page"].(string)

		if actions, ok := grant["actions"].([]any); ok {
			for _, a := range actions {
				if ga, ok := grantActionFrom(a, page); ok {
					out = append(out, ga)
				}
			}
		}
		if tabs, ok := grant["tabs"].([]any); ok {
			for _, rawTab := range tabs {
				tab, ok := rawTab.(map[string]any)
				if !ok {
					continue
				}
				actions, ok := tab["actions"].([]any)
				if !ok {
					continue
				}
				for _, a := range actions {
					if ga, ok := grantActionFrom(a, page); ok {
						out = append(out, ga)
					}
				}
			}
		}
	}
	return out
}

func grantActionFrom(raw any, page string) (grantAction, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return grantAction{}, false
	}
	name, _ := m["name"].(string)
	if name == "" {
		return grantAction{}, false
	}
	ga := grantAction{page: page, name: name}
	scopes, ok := m["row_scope"].([]any)
	if !ok {
		return ga, true
	}
	for _, rawScope := range scopes {
		entry, ok := rawScope.(map[string]any)
		if !ok {
			continue
		}
		if from, _ := entry["from"].(string); from != "session" {
			continue
		}
		field, _ := entry["field"].(string)
		attr, _ := entry["attr"].(string)
		ga.scopes = append(ga.scopes, grantScopeEntry{field: field, attr: attr})
	}
	return ga, true
}
