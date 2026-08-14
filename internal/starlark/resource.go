// Package starlark — Script Resource API
//
// This file provides the Go-side types that represent the `resource` object
// accessible from Starlark scripts. Scripts interact with entities via:
//
//	resource.set("status", "posted")
//	resource.save()
//	resource.field.status
//
// The resource object wraps a single entity record and provides methods for
// reading and writing fields, calling other actions, and loading related entities.
package starlark

import (
	"fmt"
	"strings"
	"time"

	"go.starlark.net/starlark"
)

// ResourceAPI is the Starlark-callable resource object.
// It wraps an entity record's data and provides .set(), .save(), .field access.
type ResourceAPI struct {
	// Data holds the current field values of the record.
	Data map[string]any
	// Entity is the entity name (e.g. "order").
	Entity string
	// Module is the owning module (e.g. "billing").
	Module string
	// ID is the record UUID.
	ID string
	// Version is the record's current known version, for CAS on save().
	Version int

	// saveFn is called when script calls resource.save(). It's generic over
	// module/entity/id/version (not pre-bound to this resource) so the same
	// handler can be reused by resources obtained via .fetch(), which have a
	// different module/entity/id/version than the resource that loaded them.
	saveFn func(module, entity, id string, version int, data map[string]any) error
	// callFn is called for cross-resource calls in scripts: entity.call("action", params).
	// It returns the call result or an error.
	callFn func(module, entity, action string, params map[string]any) (any, error)
	// loadFn loads another entity by ID, returning its data and version.
	loadFn func(module, entity, id string) (map[string]any, int, error)
	// createFn creates a new record of another entity, returning its ID.
	createFn func(module, entity string, data map[string]any) (string, error)

	// frozen is set when the resource transitions to the save phase.
	frozen bool
}

// compile-time check
var _ starlark.Value = (*ResourceAPI)(nil)

// NewResourceAPI creates a resource object bound to a specific entity record.
func NewResourceAPI(module, entity, id string, version int, data map[string]any) *ResourceAPI {
	return &ResourceAPI{
		Data:    data,
		Entity:  entity,
		Module:  module,
		ID:      id,
		Version: version,
	}
}

// SetSaveFunc sets the save callback.
func (r *ResourceAPI) SetSaveFunc(fn func(module, entity, id string, version int, data map[string]any) error) {
	r.saveFn = fn
}

// SetCallFunc sets the cross-resource call callback.
func (r *ResourceAPI) SetCallFunc(fn func(module, entity, action string, params map[string]any) (any, error)) {
	r.callFn = fn
}

// SetLoadFunc sets the entity load callback.
func (r *ResourceAPI) SetLoadFunc(fn func(module, entity, id string) (map[string]any, int, error)) {
	r.loadFn = fn
}

// SetCreateFunc sets the entity create callback.
func (r *ResourceAPI) SetCreateFunc(fn func(module, entity string, data map[string]any) (string, error)) {
	r.createFn = fn
}

// ─── starlark.Value interface ───

func (r *ResourceAPI) String() string {
	return fmt.Sprintf("<resource %s.%s id=%s>", r.Module, r.Entity, r.ID)
}

func (r *ResourceAPI) Type() string { return "resource" }

func (r *ResourceAPI) Freeze() {}

func (r *ResourceAPI) Truth() starlark.Bool { return starlark.Bool(r.Data != nil) }

// Hash is not supported.
func (r *ResourceAPI) Hash() (uint32, error) {
	return 0, fmt.Errorf("resource type is not hashable")
}

// ─── Starlark methods ───

// Attr returns a resource attribute: .field, .set(), .save(), .call(), .fetch(), .id
//
// Note: the Starlark-facing name is "fetch", not "load" — "load" is a
// reserved keyword in Starlark's grammar (the `load(...)` import statement),
// so `resource.load(...)` fails to parse ("not an identifier") even though
// it's a perfectly normal attribute access syntactically everywhere else.
func (r *ResourceAPI) Attr(name string) (starlark.Value, error) {
	switch name {
	case "id":
		return starlark.String(r.ID), nil
	case "field":
		return &resourceFieldAccess{r: r}, nil
	case "set":
		return r.builtinSet(), nil
	case "save":
		return r.builtinSave(), nil
	case "call":
		return r.builtinCall(), nil
	case "fetch":
		return r.builtinLoad(), nil
	case "create":
		return r.builtinCreate(), nil
	default:
		// Allow dot-notation field access as a fallback (resource.field_name)
		return nil, starlark.NoSuchAttrError(
			fmt.Sprintf("resource has no .%s attribute or field", name),
		)
	}
}

// AttrNames lists the attribute names.
func (r *ResourceAPI) AttrNames() []string {
	return []string{"id", "field", "set", "save", "call", "fetch", "create"}
}

// ─── resource.field ───

// resourceFieldAccess provides access to entity fields via dot notation:
// resource.field.total, resource.field.status, etc.
type resourceFieldAccess struct {
	r *ResourceAPI
}

var _ starlark.Value = (*resourceFieldAccess)(nil)

func (fa *resourceFieldAccess) String() string       { return "<resource.fields>" }
func (fa *resourceFieldAccess) Type() string         { return "resource_fields" }
func (fa *resourceFieldAccess) Freeze()              {}
func (fa *resourceFieldAccess) Truth() starlark.Bool { return starlark.True }
func (fa *resourceFieldAccess) Hash() (uint32, error) {
	return 0, fmt.Errorf("resource_fields is not hashable")
}

func (fa *resourceFieldAccess) Attr(name string) (starlark.Value, error) {
	val, ok := fa.r.Data[name]
	if !ok {
		return starlark.None, nil // field not set → None
	}
	return toStarlark(val)
}

func (fa *resourceFieldAccess) AttrNames() []string {
	names := make([]string, 0, len(fa.r.Data))
	for k := range fa.r.Data {
		names = append(names, k)
	}
	return names
}

// ─── Built-in methods ───

func (r *ResourceAPI) builtinSet() *starlark.Builtin {
	return starlark.NewBuiltin("resource.set", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var name string
		var value starlark.Value
		if err := starlark.UnpackArgs("set", args, kwargs, "name", &name, "value", &value); err != nil {
			return nil, err
		}

		if r.frozen {
			return nil, fmt.Errorf("resource.set: resource is frozen after save()")
		}

		goVal := fromStarlark(value)
		r.Data[name] = goVal
		return starlark.None, nil
	})
}

func (r *ResourceAPI) builtinSave() *starlark.Builtin {
	return starlark.NewBuiltin("resource.save", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		if r.saveFn == nil {
			return nil, fmt.Errorf("resource.save: no save handler registered")
		}
		if err := r.saveFn(r.Module, r.Entity, r.ID, r.Version, r.Data); err != nil {
			return nil, fmt.Errorf("resource.save: %w", err)
		}
		r.frozen = true
		return starlark.None, nil
	})
}

func (r *ResourceAPI) builtinCall() *starlark.Builtin {
	return starlark.NewBuiltin("resource.call", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var target string
		var action string
		var params starlark.Value = starlark.None

		if err := starlark.UnpackArgs("call", args, kwargs,
			"target", &target,
			"action", &action,
			"params?", &params,
		); err != nil {
			return nil, err
		}

		if r.callFn == nil {
			return nil, fmt.Errorf("resource.call: no call handler registered")
		}

		paramsMap := starlarkValueToMap(params)
		module, entityName := splitModuleEntity(r.Module, target)
		result, err := r.callFn(module, entityName, action, paramsMap)
		if err != nil {
			return nil, fmt.Errorf("resource.call(%s.%s): %w", target, action, err)
		}

		return toStarlark(result)
	})
}

func (r *ResourceAPI) builtinLoad() *starlark.Builtin {
	return starlark.NewBuiltin("resource.fetch", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var entity string
		var id string
		if err := starlark.UnpackArgs("fetch", args, kwargs,
			"entity", &entity,
			"id", &id,
		); err != nil {
			return nil, err
		}

		if r.loadFn == nil {
			return nil, fmt.Errorf("resource.fetch: no fetch handler registered")
		}

		module, entityName := splitModuleEntity(r.Module, entity)
		data, version, err := r.loadFn(module, entityName, id)
		if err != nil {
			return nil, fmt.Errorf("resource.fetch(%s, %s): %w", entity, id, err)
		}

		loaded := NewResourceAPI(module, entityName, id, version, data)
		// Propagate handlers so the loaded resource can itself be .set()/.save()d,
		// .call()ed, .fetch()ed, or .create()d from — e.g. rx_dispense.star loads
		// a medicine record, decrements stock, and saves it back.
		loaded.saveFn = r.saveFn
		loaded.callFn = r.callFn
		loaded.loadFn = r.loadFn
		loaded.createFn = r.createFn
		return loaded, nil
	})
}

func (r *ResourceAPI) builtinCreate() *starlark.Builtin {
	return starlark.NewBuiltin("resource.create", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var entity string
		var data starlark.Value = starlark.None
		if err := starlark.UnpackArgs("create", args, kwargs,
			"entity", &entity,
			"data", &data,
		); err != nil {
			return nil, err
		}

		if r.createFn == nil {
			return nil, fmt.Errorf("resource.create: no create handler registered")
		}

		dataMap := starlarkValueToMap(data)
		module, entityName := splitModuleEntity(r.Module, entity)
		id, err := r.createFn(module, entityName, dataMap)
		if err != nil {
			return nil, fmt.Errorf("resource.create(%s): %w", entity, err)
		}

		// The new record only has what we just wrote, and version 1 (the
		// DB's default on insert, per renderers/jsonb-persist/ddl.go) — the caller can
		// resource.fetch() it back if it needs server-computed fields or the
		// authoritative version.
		created := make(map[string]any, len(dataMap))
		for k, v := range dataMap {
			created[k] = v
		}
		newRes := NewResourceAPI(module, entityName, id, 1, created)
		newRes.saveFn = r.saveFn
		newRes.callFn = r.callFn
		newRes.loadFn = r.loadFn
		newRes.createFn = r.createFn
		return newRes, nil
	})
}

// starlarkValueToMap converts a Starlark value to a Go map[string]any.
func starlarkValueToMap(v starlark.Value) map[string]any {
	if v == nil || v == starlark.None {
		return nil
	}
	dict, ok := v.(*starlark.Dict)
	if !ok {
		return nil
	}
	result := make(map[string]any, dict.Len())
	for _, item := range dict.Items() {
		key, _ := starlark.AsString(item[0])
		result[key] = fromStarlark(item[1])
	}
	return result
}

// splitModuleEntity splits a script-supplied target string like
// "pharmacy.medicine" into (module="pharmacy", entity="medicine"). A bare
// name with no dot — the same-module case, e.g. "medicine" — returns
// (defaultModule, target) unchanged, so every existing same-module script
// (e.g. rx_dispense.star's resource.fetch("medicine", ...)) keeps working
// byte-for-byte. Module/entity identifiers are kebab-case and never contain
// ".", so splitting on the first dot is unambiguous.
func splitModuleEntity(defaultModule, target string) (module, entity string) {
	if i := strings.IndexByte(target, '.'); i >= 0 {
		return target[:i], target[i+1:]
	}
	return defaultModule, target
}

// now is a package-level clock for testability.
var now = time.Now
