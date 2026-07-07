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

	// saveFn is called when script calls resource.save().
	saveFn func(data map[string]any) error
	// callFn is called for cross-resource calls in scripts: entity.call("action", params).
	// It returns the call result or an error.
	callFn func(module, entity, action string, params map[string]any) (any, error)
	// loadFn loads another entity by ID.
	loadFn func(module, entity, id string) (map[string]any, error)

	// frozen is set when the resource transitions to the save phase.
	frozen bool
}

// compile-time check
var _ starlark.Value = (*ResourceAPI)(nil)

// NewResourceAPI creates a resource object bound to a specific entity record.
func NewResourceAPI(module, entity, id string, data map[string]any) *ResourceAPI {
	return &ResourceAPI{
		Data:   data,
		Entity: entity,
		Module: module,
		ID:     id,
	}
}

// SetSaveFunc sets the save callback.
func (r *ResourceAPI) SetSaveFunc(fn func(data map[string]any) error) { r.saveFn = fn }

// SetCallFunc sets the cross-resource call callback.
func (r *ResourceAPI) SetCallFunc(fn func(module, entity, action string, params map[string]any) (any, error)) {
	r.callFn = fn
}

// SetLoadFunc sets the entity load callback.
func (r *ResourceAPI) SetLoadFunc(fn func(module, entity, id string) (map[string]any, error)) {
	r.loadFn = fn
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

// Attr returns a resource attribute: .field, .set(), .save(), .call(), .load(), .id
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
	case "load":
		return r.builtinLoad(), nil
	default:
		// Allow dot-notation field access as a fallback (resource.field_name)
		return nil, starlark.NoSuchAttrError(
			fmt.Sprintf("resource has no .%s attribute or field", name),
		)
	}
}

// AttrNames lists the attribute names.
func (r *ResourceAPI) AttrNames() []string {
	return []string{"id", "field", "set", "save", "call", "load"}
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
		if err := r.saveFn(r.Data); err != nil {
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
		result, err := r.callFn(r.Module, target, action, paramsMap)
		if err != nil {
			return nil, fmt.Errorf("resource.call(%s.%s): %w", target, action, err)
		}

		return toStarlark(result)
	})
}

func (r *ResourceAPI) builtinLoad() *starlark.Builtin {
	return starlark.NewBuiltin("resource.load", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var entity string
		var id string
		if err := starlark.UnpackArgs("load", args, kwargs,
			"entity", &entity,
			"id", &id,
		); err != nil {
			return nil, err
		}

		if r.loadFn == nil {
			return nil, fmt.Errorf("resource.load: no load handler registered")
		}

		data, err := r.loadFn(r.Module, entity, id)
		if err != nil {
			return nil, fmt.Errorf("resource.load(%s, %s): %w", entity, id, err)
		}

		return NewResourceAPI(r.Module, entity, id, data), nil
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

// now is a package-level clock for testability.
var now = time.Now
