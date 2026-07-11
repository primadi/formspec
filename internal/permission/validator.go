package permission

import (
	"fmt"

	"github.com/primadi/forma/pkg/spec"
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
	}

	return errs
}
