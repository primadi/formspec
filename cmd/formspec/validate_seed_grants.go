package main

import (
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// validateSeedRoleGrants is the DEPLOY-TIME gate for role grants written in a
// `kind: Seed` manifest — the shape kafe actually runs (its grants come from
// `seeds/roles.yaml`, not from the admin editor).
//
// It exists because `grants` is free JSON in a jsonb column with no schema
// behind it, and the runtime reader is LOSSY on purpose: `json.Unmarshal` into
// the `Grant` struct silently drops a key it does not know. So `row_scopes:`
// instead of `row_scope:` produced a role that looked configured, kept its
// permission, and carried no restriction at all — the 10.67 measurement, where
// the kitchen read drafts again. Nothing else in the pipeline could see that:
// validate passed, check passed, and the running system behaved as if the
// manifest had never declared a restriction.
//
// The rules themselves live in `auth.ValidateGrantListShape` — the SAME function
// the runtime role loader uses — so the gate and the enforcement can never
// disagree about what a readable grant is. A second copy of this rule is exactly
// the drift this repo keeps paying for (10.46).
//
// Registry-free by design: it can say "this key is misspelled" or "this
// row_scope entry has no value source" without the spec tree. Whether the page
// resolves and whether the field exists on the target entity needs the
// registries, and is checked by the Materializer (whose problems carry the same
// row-scope flag).
func validateSeedRoleGrants(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}
	record := func(source string, format string, args ...any) {
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
				name, _ := rec["name"].(string)
				if strings.TrimSpace(name) == "" {
					name = "<unnamed>"
				}
				raw, ok := rec["grants"]
				if !ok || raw == nil {
					continue
				}
				for _, p := range auth.ValidateGrantListShape(raw) {
					// The row-restriction class is called out in the message: it is
					// the one that is fail-OPEN at runtime (the permission survives
					// without its restriction), so an operator must not read it as
					// merely "this grant was dropped".
					if p.RowScope {
						record(m.Source, "seed role %q: %s — %s [row restriction]", name, p.Path, p.Message)
					} else {
						record(m.Source, "seed role %q: %s — %s", name, p.Path, p.Message)
					}
				}
			}
		}
	}
	return rejects
}

// isRoleEntityName matches the role entity, which the framework module owns
// (`formspec.core.role`) but a project may also name with its module prefix.
func isRoleEntityName(name string) bool {
	return name == "role" || strings.HasSuffix(name, ".role")
}
