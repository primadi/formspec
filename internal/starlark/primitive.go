package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
)

// ─── primitiveHandle — callable ctx primitive with .named() support ───
//
// primitiveHandle implements starlark.Value and starlark.Callable.
// It represents a ctx primitive (db, cache, lock, etc.) that can be called
// directly (uses default datastore) or via .named("name") for a specific datastore.
//
// Usage in Starlark:
//
//	result = ctx.db().query("SELECT ...")              # default datastore
//	result = ctx.db().named("analytics-db").query(...)  # named datastore
//	ctx.cache().set("key", value, ttl=3600)
//	ctx.cache().named("session-cache").get("key")
type primitiveHandle struct {
	primType string
	resolver func(primitiveType, name string) (interface{}, error)
	namedDS  string // empty = use default
}

func newPrimitiveHandle(primType string, resolver func(primitiveType, name string) (interface{}, error)) *primitiveHandle {
	return &primitiveHandle{primType: primType, resolver: resolver}
}

var (
	_ starlark.Value    = (*primitiveHandle)(nil)
	_ starlark.Callable = (*primitiveHandle)(nil)
)

func (p *primitiveHandle) String() string {
	if p.namedDS != "" {
		return fmt.Sprintf("<ctx.%s named=%q>", p.primType, p.namedDS)
	}
	return fmt.Sprintf("<ctx.%s>", p.primType)
}
func (p *primitiveHandle) Type() string         { return p.primType }
func (p *primitiveHandle) Freeze()              {}
func (p *primitiveHandle) Truth() starlark.Bool { return starlark.True }
func (p *primitiveHandle) Hash() (uint32, error) {
	return 0, fmt.Errorf("%s is not hashable", p.primType)
}

func (p *primitiveHandle) Attr(name string) (starlark.Value, error) {
	if name == "named" {
		return starlark.NewBuiltin(p.primType+".named", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			var dsName string
			if err := starlark.UnpackArgs("named", args, kwargs, "name", &dsName); err != nil {
				return nil, err
			}
			return &primitiveHandle{
				primType: p.primType,
				resolver: p.resolver,
				namedDS:  dsName,
			}, nil
		}), nil
	}
	return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s", p.primType, name))
}

func (p *primitiveHandle) AttrNames() []string { return []string{"named"} }

// Name returns the same as String for starlark.Callable interface.
func (p *primitiveHandle) Name() string { return p.String() }

// CallInternal makes the primitive callable — returns a primitiveRunner that represents
// the resolved connection. Actual operations (query, get, set, etc.) are dispatched
// via the runner's Attr method.
func (p *primitiveHandle) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	name := p.namedDS
	if name == "" {
		name = "default"
	}

	conn, err := p.resolver(p.primType, name)
	if err != nil {
		return nil, fmt.Errorf("ctx.%s: %w", p.primType, err)
	}

	return &primitiveRunner{
		primType: p.primType,
		name:     name,
		conn:     conn,
	}, nil
}

// ─── primitiveRunner — resolved connection ───

// primitiveRunner represents a resolved connection to a datastore.
// Operations like query(), get(), set() are dispatched via Attr.
type primitiveRunner struct {
	primType string
	name     string
	conn     interface{}
}

var _ starlark.Value = (*primitiveRunner)(nil)

func (r *primitiveRunner) String() string {
	return fmt.Sprintf("<ctx.%s connection=%q>", r.primType, r.name)
}
func (r *primitiveRunner) Type() string         { return r.primType + "_conn" }
func (r *primitiveRunner) Freeze()              {}
func (r *primitiveRunner) Truth() starlark.Bool { return starlark.True }
func (r *primitiveRunner) Hash() (uint32, error) {
	return 0, fmt.Errorf("%s_conn is not hashable", r.primType)
}

func (r *primitiveRunner) Attr(name string) (starlark.Value, error) {
	switch name {
	case "query":
		return r.builtinQuery(), nil
	case "get":
		return r.builtinGet(), nil
	case "set":
		return r.builtinSet(), nil
	case "delete":
		return r.builtinDelete(), nil
	case "acquire":
		return r.builtinAcquire(), nil
	case "release":
		return r.builtinRelease(), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s connection has no .%s", r.primType, name))
	}
}

func (r *primitiveRunner) AttrNames() []string {
	return []string{"query", "get", "set", "delete", "acquire", "release"}
}

func (r *primitiveRunner) builtinQuery() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".query", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var sql string
		if err := starlark.UnpackArgs("query", args, kwargs, "sql", &sql); err != nil {
			return nil, err
		}
		// TODO: execute query against resolved connection
		return starlark.None, fmt.Errorf("ctx.%s.query: not yet implemented (connection=%q)", r.primType, r.name)
	})
}

func (r *primitiveRunner) builtinGet() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".get", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var key string
		if err := starlark.UnpackArgs("get", args, kwargs, "key", &key); err != nil {
			return nil, err
		}
		return starlark.None, fmt.Errorf("ctx.%s.get: not yet implemented (connection=%q)", r.primType, r.name)
	})
}

func (r *primitiveRunner) builtinSet() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".set", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		return starlark.None, fmt.Errorf("ctx.%s.set: not yet implemented (connection=%q)", r.primType, r.name)
	})
}

func (r *primitiveRunner) builtinDelete() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".delete", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		return starlark.None, fmt.Errorf("ctx.%s.delete: not yet implemented (connection=%q)", r.primType, r.name)
	})
}

func (r *primitiveRunner) builtinAcquire() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".acquire", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		return starlark.None, fmt.Errorf("ctx.%s.acquire: not yet implemented (connection=%q)", r.primType, r.name)
	})
}

func (r *primitiveRunner) builtinRelease() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".release", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		return starlark.None, fmt.Errorf("ctx.%s.release: not yet implemented (connection=%q)", r.primType, r.name)
	})
}
