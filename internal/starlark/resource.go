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
	// It returns the call result or an error. id is the target record's ID when
	// the script addressed a specific instance ("module.entity.<id>"), empty
	// otherwise (a collection-level action on the entity itself).
	callFn func(module, entity, id, action string, params map[string]any) (any, error)
	// loadFn loads another entity by ID, returning its data, version, and the
	// resolved record ID. The resolved ID is returned separately rather than
	// read from the data map: a record's id lives in its own table column, so
	// the data map has no "id" key — and a lookup by natural key resolves to a
	// different (UUID) identity than the value passed in.
	loadFn func(module, entity, id string) (map[string]any, int, string, error)
	// findFn finds another entity by field values, returning its data, version,
	// and the resolved record ID (or nil data when no row matches). It is the
	// high-level alternative to raw SQL for guards that must check "does a row
	// with these field values already exist" (#31) — the query respects tenant
	// isolation and row scope, which a hand-written ctx.db().query() cannot.
	findFn func(module, entity string, match map[string]any) (map[string]any, int, string, error)
	// upsertFn writes a row of a `characteristic: summary` projection, matching
	// on `match` and merging `data` (item 4.1). It is the ONE supported write
	// path for summary entities; the caller-identity check (that the running
	// script is the entity's maintained_by) lives in the wiring layer.
	upsertFn func(module, entity string, match, data map[string]any) (string, bool, error)
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
func (r *ResourceAPI) SetCallFunc(fn func(module, entity, id, action string, params map[string]any) (any, error)) {
	r.callFn = fn
}

// SetLoadFunc sets the entity load callback.
func (r *ResourceAPI) SetLoadFunc(fn func(module, entity, id string) (map[string]any, int, string, error)) {
	r.loadFn = fn
}

// SetFindFunc sets the entity find-by-field callback (#31).
func (r *ResourceAPI) SetFindFunc(fn func(module, entity string, match map[string]any) (map[string]any, int, string, error)) {
	r.findFn = fn
}

// SetUpsertFunc sets the summary-projection upsert callback (item 4.1).
func (r *ResourceAPI) SetUpsertFunc(fn func(module, entity string, match, data map[string]any) (string, bool, error)) {
	r.upsertFn = fn
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
	case "find":
		return r.builtinFind(), nil
	case "upsert":
		return r.builtinUpsert(), nil
	case "create":
		return r.builtinCreate(), nil
	case "new":
		return r.builtinNew(), nil
	default:
		// Allow dot-notation field access as a fallback (resource.field_name)
		return nil, starlark.NoSuchAttrError(
			fmt.Sprintf("resource has no .%s attribute or field", name),
		)
	}
}

// AttrNames lists the attribute names.
func (r *ResourceAPI) AttrNames() []string {
	return []string{"id", "field", "set", "save", "call", "fetch", "find", "upsert", "create", "new"}
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
		module, entityName, id := splitCallTarget(r.Module, target)
		result, err := r.callFn(module, entityName, id, action, paramsMap)
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
		data, version, resolvedID, err := r.loadFn(module, entityName, id)
		if err != nil {
			return nil, fmt.Errorf("resource.fetch(%s, %s): %w", entity, id, err)
		}
		if resolvedID == "" {
			// Handlers that predate the resolved-ID return still work: fall
			// back to the requested id, then to whatever the data map carries.
			resolvedID = id
		}

		loaded := NewResourceAPI(module, entityName, resolvedID, version, data)
		// Propagate handlers so the loaded resource can itself be .set()/.save()d,
		// .call()ed, .fetch()ed, or .create()d from — e.g. rx_dispense.star loads
		// a medicine record, decrements stock, and saves it back.
		loaded.saveFn = r.saveFn
		loaded.callFn = r.callFn
		loaded.loadFn = r.loadFn
		loaded.findFn = r.findFn
		loaded.upsertFn = r.upsertFn
		loaded.createFn = r.createFn
		return loaded, nil
	})
}

// builtinFind serves resource.find(entity, match) — the high-level alternative
// to raw SQL for guards that must check whether a row with the given field
// values already exists (#31). `match` is a dict of field→value; all pairs must
// match (AND). Unlike ctx.db().query(), the lookup goes through the entity
// layer, so tenant isolation and row scope apply and the caller never has to
// know the physical table/column names.
//
// Returns the matching resource, or None when no row matches — so a guard can
// write `if resource.find(...): fail(...)` without a length check.
func (r *ResourceAPI) builtinFind() *starlark.Builtin {
	return starlark.NewBuiltin("resource.find", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var entity string
		var match starlark.Value
		if err := starlark.UnpackArgs("find", args, kwargs,
			"entity", &entity,
			"match", &match,
		); err != nil {
			return nil, err
		}

		if r.findFn == nil {
			return nil, fmt.Errorf("resource.find: no find handler registered")
		}

		matchMap := starlarkValueToMap(match)
		if len(matchMap) == 0 {
			return nil, fmt.Errorf("resource.find: match must be a non-empty dict of field→value")
		}

		module, entityName := splitModuleEntity(r.Module, entity)
		data, version, resolvedID, err := r.findFn(module, entityName, matchMap)
		if err != nil {
			return nil, fmt.Errorf("resource.find(%s): %w", entity, err)
		}
		if data == nil {
			return starlark.None, nil
		}

		if resolvedID == "" {
			// Defensive fallback for a data map that happens to carry an id.
			resolvedID, _ = data["id"].(string)
		}
		found := NewResourceAPI(module, entityName, resolvedID, version, data)
		found.saveFn = r.saveFn
		found.callFn = r.callFn
		found.loadFn = r.loadFn
		found.findFn = r.findFn
		found.upsertFn = r.upsertFn
		found.createFn = r.createFn
		return found, nil
	})
}

// builtinUpsert serves resource.upsert(entity, match, data) — the ONE supported
// write path for a `characteristic: summary` projection (item 4.1). It matches
// on `match` (all pairs, AND) and merges `data` onto the existing row or inserts
// a new one. The API-facing create/update/delete reject summary entities; this
// exists so a maintainer script named by `maintained_by` can keep its projection
// current.
//
// The caller-identity check (that the running script IS the entity's
// maintained_by) lives in the wiring layer, not here — this builtin only
// forwards the call. Returns the row id.
func (r *ResourceAPI) builtinUpsert() *starlark.Builtin {
	return starlark.NewBuiltin("resource.upsert", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var entity string
		var match, data starlark.Value
		if err := starlark.UnpackArgs("upsert", args, kwargs,
			"entity", &entity,
			"match", &match,
			"data", &data,
		); err != nil {
			return nil, err
		}

		if r.upsertFn == nil {
			return nil, fmt.Errorf("resource.upsert: no upsert handler registered")
		}

		matchMap := starlarkValueToMap(match)
		if len(matchMap) == 0 {
			return nil, fmt.Errorf("resource.upsert: match must be a non-empty dict of field→value")
		}
		dataMap := starlarkValueToMap(data)

		module, entityName := splitModuleEntity(r.Module, entity)
		id, _, err := r.upsertFn(module, entityName, matchMap, dataMap)
		if err != nil {
			return nil, fmt.Errorf("resource.upsert(%s): %w", entity, err)
		}
		return starlark.String(id), nil
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

// builtinNew returns a new, unsaved handle for the SAME entity as this
// resource (todo 7.14.4, 06-script-runtime.md §2). The caller fills it via
// .set(...) then .save() — because the handle has ID "", save() performs an
// INSERT (not an update). This is distinct from resource.create(), which
// immediately persists a record of another entity.
func (r *ResourceAPI) builtinNew() *starlark.Builtin {
	return starlark.NewBuiltin("resource.new", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		newRes := NewResourceAPI(r.Module, r.Entity, "", 0, make(map[string]any))
		// Propagate handlers so the new handle can .set()/.save()/.call()/
		// .fetch()/.create()/.new() from — save() with ID "" inserts.
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

// splitCallTarget splits a resource.call target into (module, entity, id).
//
// Three accepted forms — the third segment is always a record ID, so calling
// an action on ONE record (e.g. "gl.journal-entry.<uuid>" → "post") works the
// same as calling one on the collection:
//
//	"medicine"                      → (defaultModule, "medicine",  "")
//	"pharmacy.medicine"             → ("pharmacy",     "medicine",  "")
//	"gl.journal-entry.01a0c332-..." → ("gl",           "journal-entry", "01a0c332-...")
//
// Because module and entity identifiers are kebab-case and never contain ".",
// the segment boundaries are unambiguous. A trailing dot with nothing after it
// ("gl.journal-entry.") is treated as the collection form rather than an
// empty ID — that shape is what an interpolating caller produces when the ID
// it meant to append is missing, and a collection call gives a clearer error
// than a lookup of the empty string.
func splitCallTarget(defaultModule, target string) (module, entity, id string) {
	if cut := strings.IndexByte(target, '.'); cut >= 0 {
		module, entity = target[:cut], target[cut+1:]
	} else {
		module, entity = defaultModule, target
	}
	if i := strings.IndexByte(entity, '.'); i >= 0 {
		id = entity[i+1:]
		entity = entity[:i]
	}
	return module, entity, id
}

// now is a package-level clock for testability.
var now = time.Now
