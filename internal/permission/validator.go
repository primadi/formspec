package permission

import (
	"fmt"

	"github.com/primadi/formspec/pkg/spec"
)

// ValidateUses validates a UsesDecl declaration.
//
// Checks:
//   - Resource targets have valid format (module.entity.action or entity.action)
//   - Primitives are from the closed set (ctx.*)
//   - Config keys are non-empty
func ValidateUses(uses *spec.UsesDecl, module string) []error {
	if uses == nil {
		return nil
	}

	var errs []error

	// Validate resources
	for i, res := range uses.Resources {
		_, _, _, err := ParseResourceTarget(res, module)
		if err != nil {
			errs = append(errs, fmt.Errorf("uses.resources[%d]: %w", i, err))
		}
	}

	// Validate db category/module names
	if uses.Db != nil {
		for i, m := range uses.Db.Read {
			if m == "" {
				errs = append(errs, fmt.Errorf("uses.db.read[%d]: empty module", i))
			}
		}
		for i, m := range uses.Db.Write {
			if m == "" {
				errs = append(errs, fmt.Errorf("uses.db.write[%d]: empty module", i))
			}
		}
	}

	// Validate kvstore declarations
	for i, kv := range uses.Kvstore {
		switch kv.Access {
		case "", "read", "read_write":
		default:
			errs = append(errs, fmt.Errorf("uses.kvstore[%d]: access must be read or read_write, got %q", i, kv.Access))
		}
	}

	// Validate config
	if uses.Config != nil {
		for i, key := range uses.Config.Read {
			if key == "" {
				errs = append(errs, fmt.Errorf("uses.config.read[%d]: empty key", i))
			}
		}
		for i, key := range uses.Config.Write {
			if key == "" {
				errs = append(errs, fmt.Errorf("uses.config.write[%d]: empty key", i))
			}
		}
	}

	// Validate primitives — must be from the closed set
	validPrimitives := map[string]bool{
		"db": true, "cache": true, "lock": true,
		"queue": true, "pubsub": true, "storage": true,
		"config": true, "kvstore": true, "log": true,
	}
	for i, p := range uses.Primitives {
		if !validPrimitives[p] {
			errs = append(errs, fmt.Errorf("uses.primitives[%d]: %q is not a valid primitive (closed set: db, cache, lock, queue, pubsub, storage, config, kvstore, log)", i, p))
		}
	}

	return errs
}

// ValidateAction validates an action's permission and uses declarations.
//
// Checks:
//   - required_permission format is valid
//   - uses declarations are valid
//   - public actions declare a rate limit (see ValidatePublicAction)
func ValidateAction(action spec.Action, module string) []error {
	var errs []error

	// Validate required_permission
	if action.RequiredPermission != "" {
		if err := ValidatePermissionFormat(action.RequiredPermission); err != nil {
			errs = append(errs, fmt.Errorf("action %q: %w", action.Name, err))
		}
	}

	// Validate uses
	if action.Uses != nil {
		useErrs := ValidateUses(action.Uses, module)
		for _, e := range useErrs {
			errs = append(errs, fmt.Errorf("action %q: %w", action.Name, e))
		}
	}

	return errs
}

// ValidatePublicAction checks an action that declares `public: true`.
//
// `public` is the SERVICE-side allowlist: an anonymous endpoint. Two rules keep
// it safe, and both are hard errors rather than warnings because the failure
// mode is an open door rather than an inconvenience.
//
//  1. A public action MUST declare `rate_limit`. An anonymous endpoint with no
//     rate limit is an abuse vector, and nothing downstream can invent the
//     limit the author did not state.
//  2. `public` is refused on ENTITY actions. Entity actions are reachable
//     anonymously only through the App's DERIVED public grants — computed from
//     the surface the App exposes (plan implicit-public-grants.md), which is
//     the single place an operator reviews what anonymous callers may touch. A
//     per-action switch would route around that review, so an entity must be
//     exposed through a view instead (kafe 10.39).
//
// kind is spec.KindEntity or spec.KindService; the caller knows which it is
// validating.
func ValidatePublicAction(action spec.Action, kind spec.Kind) []error {
	if !action.Public {
		return nil
	}
	var errs []error

	if kind != spec.KindService {
		errs = append(errs, fmt.Errorf(
			"action %q: `public: true` is only valid on a Service action — an entity action reaches anonymous callers only through the public views its App exposes; expose the entity through a page/form instead",
			action.Name))
	}
	if action.RateLimit == nil {
		errs = append(errs, fmt.Errorf(
			"action %q: `public: true` requires `rate_limit` — an anonymous endpoint with no rate limit is an abuse vector (e.g. rate_limit: {max: 10, per: 1m, scope: ip})",
			action.Name))
	}

	return errs
}

// BuildUsesEntry converts a spec.UsesDecl into a permission.UsesEntry.
// Returns nil if uses is nil.
func BuildUsesEntry(module, entity, action string, uses *spec.UsesDecl) *UsesEntry {
	if uses == nil {
		return nil
	}

	entry := &UsesEntry{
		Module:     module,
		Entity:     entity,
		Action:     action,
		Primitives: uses.Primitives,
	}

	// Convert resources
	for _, res := range uses.Resources {
		resourceModule, resourceEntity, resourceAction, _ := ParseResourceTarget(res, module)
		mode := AccessRead
		// Default mode is read; write is explicit in target format
		// For now, all resource uses are read unless specified otherwise
		// In the future: "billing.invoice.write" format
		_ = resourceAction
		entry.Resources = append(entry.Resources, ResourceUse{
			Target: fmt.Sprintf("%s.%s", resourceModule, resourceEntity),
			Mode:   mode,
		})
	}

	// Convert config
	if uses.Config != nil {
		entry.Config = &ConfigUse{
			Read:  uses.Config.Read,
			Write: uses.Config.Write,
		}
	}

	// Convert db (raw SQL category access — part of the consent footprint, D46)
	if uses.Db != nil {
		entry.Db = &DbUse{
			Read:  uses.Db.Read,
			Write: uses.Db.Write,
		}
	}

	return entry
}

// ValidateEntitySpec validates permission/uses for all actions in an entity spec.
func ValidateEntitySpec(meta spec.Metadata, entitySpec *spec.EntitySpec) []error {
	var errs []error

	for i := range entitySpec.Actions {
		action := entitySpec.Actions[i]
		actionErrs := ValidateAction(action, meta.Module)
		errs = append(errs, actionErrs...)
		// `public` is refused on an entity action: anonymous reach for an
		// entity is declared once, in the App's `public_entities` allowlist.
		errs = append(errs, ValidatePublicAction(action, spec.KindEntity)...)
	}

	return errs
}

// ValidateServiceSpec validates permission/uses for all actions in a service spec.
//
// It runs the same per-action checks as an entity PLUS the public-action rules:
// a Service action is the only place `public: true` is legal, so this is where
// that flag is actually vetted (kafe 10.39).
func ValidateServiceSpec(meta spec.Metadata, serviceSpec *spec.ServiceSpec) []error {
	var errs []error

	for i := range serviceSpec.Actions {
		action := serviceSpec.Actions[i]
		errs = append(errs, ValidateAction(action, meta.Module)...)
		errs = append(errs, ValidatePublicAction(action, spec.KindService)...)
	}

	return errs
}
