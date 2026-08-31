package starlark

import (
	"context"
	"fmt"
	"time"

	"go.starlark.net/starlark"
)

// ─── Capability interfaces ───
//
// A resolved ctx primitive connection (returned by the datastore resolver)
// may implement any of these capability interfaces. Operations on a
// connection that does not implement the matching interface fail with a
// clear "not yet implemented for this backend" error — the same surface the
// sidecar exposes (docs/runtimes/04-formspec-sidecar.md §4.3). Go interfaces
// are structural, so a single backend adapter can satisfy both this contract
// and the sidecar's identical one.
type (
	// Querier serves ctx.db().query(sql, args...).
	Querier interface {
		Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error)
	}
	// KVGetter serves ctx.cache().get(key) / ctx.kvstore().get(key).
	KVGetter interface {
		Get(ctx context.Context, key string) (any, error)
	}
	// KVSetter serves ctx.cache().set(key, value, ttl=...) / ctx.kvstore().set(...).
	KVSetter interface {
		Set(ctx context.Context, key string, value any, ttl time.Duration) error
	}
	// KVDeleter serves ctx.cache().delete(key) / ctx.kvstore().delete(key).
	KVDeleter interface {
		Delete(ctx context.Context, key string) error
	}
	// Locker serves ctx.lock().acquire(key, ttl) / ctx.lock().release(key).
	Locker interface {
		Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
		Release(ctx context.Context, key string) error
	}
	// Queue serves ctx.queue().enqueue(name, payload) / ctx.queue().dequeue(name).
	Queue interface {
		Enqueue(ctx context.Context, name string, payload any) error
		Dequeue(ctx context.Context, name string) (any, error)
	}
	// PubSub serves ctx.pubsub().publish(channel, payload) / ctx.pubsub().subscribe(channel, cb).
	PubSub interface {
		Publish(ctx context.Context, channel string, payload any) error
		Subscribe(ctx context.Context, channel string, cb func(payload any)) error
	}
	// Storage serves ctx.storage().upload(path, data) / ctx.storage().download(path).
	Storage interface {
		Upload(ctx context.Context, path string, data []byte) error
		Download(ctx context.Context, path string) ([]byte, error)
	}
	// Config serves ctx.config().get(key) for non-secret config keys.
	Config interface {
		Get(ctx context.Context, key string) (any, error)
	}
	// Logger serves the ctx.log primitive when it is routed through the
	// datastore resolver (plan fase D): a centralized log backend (e.g.
	// Redis) for multi-instance deployments. The builtin ctx.log (in-memory
	// entries returned via ScriptResult.LogEntries) remains the fallback.
	Logger interface {
		Log(ctx context.Context, level, event string, meta map[string]any) error
	}
)

// ctxKey is the thread-local key under which the Go context.Context for the
// current script execution is stored (set by ExecuteScript). primitiveRunner
// operations retrieve it so backend calls carry the request context.
const ctxKey = "formspec.go.context"

// threadContext returns the Go context stored on the Starlark thread, or
// context.Background() when none was set (e.g. direct unit-test callers).
func threadContext(thread *starlark.Thread) context.Context {
	if v := thread.Local(ctxKey); v != nil {
		if c, ok := v.(context.Context); ok {
			return c
		}
	}
	return context.Background()
}

// ─── primitiveHandle — callable ctx primitive with .named() support ───
//
// primitiveHandle implements starlark.Value and starlark.Callable.
// It represents a ctx primitive (db, cache, lock, etc.) that can be called
// directly (uses default datastore) or via .named("name") for a specific datastore.
//
// Usage in Starlark:
//
//	result = ctx.db().query("SELECT ...")               # caller's bound datastore
//	result = ctx.db.named("analytics-db").query(...)    # explicit named datastore
//	ctx.cache().set("key", value, ttl=3600)
//	ctx.cache.named("session-cache").get("key")
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
			// Resolve immediately so the chain ctx.db.named("x").query(...)
			// works: .named() returns the resolved connection runner. The
			// "named:" prefix routes the request to the named-logical-
			// primitive path in the resolver wrapper (plan fase C) — the
			// alias gate (checkDatastoreAlias) fires there.
			conn, err := p.resolver(p.primType, namedPrefix+dsName)
			if err != nil {
				return nil, fmt.Errorf("ctx.%s: %w", p.primType, err)
			}
			return &primitiveRunner{
				primType: p.primType,
				name:     dsName,
				conn:     conn,
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
	// An empty name means "the caller's bound datastore" (todo 2.9.4) —
	// pass "" through so the registry can apply module-scoped binding;
	// only .named("x") carries an explicit name.
	name := p.namedDS

	conn, err := p.resolver(p.primType, name)
	if err != nil {
		return nil, fmt.Errorf("ctx.%s: %w", p.primType, err)
	}

	if name == "" {
		name = "default"
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
	case "enqueue":
		return r.builtinEnqueue(), nil
	case "dequeue":
		return r.builtinDequeue(), nil
	case "publish":
		return r.builtinPublish(), nil
	case "subscribe":
		return r.builtinSubscribe(), nil
	case "upload":
		return r.builtinUpload(), nil
	case "download":
		return r.builtinDownload(), nil
	case "info":
		return r.builtinLog("info"), nil
	case "warn":
		return r.builtinLog("warn"), nil
	case "error":
		return r.builtinLog("error"), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s connection has no .%s", r.primType, name))
	}
}

func (r *primitiveRunner) AttrNames() []string {
	return []string{"query", "get", "set", "delete", "acquire", "release",
		"enqueue", "dequeue", "publish", "subscribe", "upload", "download",
		"info", "warn", "error"}
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
		// Sandbox limit (7.14.1): max DB queries per script.
		if err := threadLimits(thread).CheckQuery(); err != nil {
			return nil, err
		}
		q, ok := r.conn.(Querier)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.query: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		rows, err := q.Query(threadContext(thread), sql)
		if err != nil {
			return nil, fmt.Errorf("ctx.%s.query: %w", r.primType, err)
		}
		// Sandbox limit (7.14.1): max records read.
		if err := threadLimits(thread).AddRecordsRead(len(rows)); err != nil {
			return nil, err
		}
		return toStarlark(rows)
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
		// Fase D: the config primitive accepts both KVGetter (KV-backed
		// config store) and the Config capability (SQL-backed store).
		if r.primType == "config" {
			if cfg, ok := r.conn.(Config); ok {
				val, err := cfg.Get(threadContext(thread), key)
				if err != nil {
					return nil, fmt.Errorf("ctx.%s.get: %w", r.primType, err)
				}
				if val == nil {
					return starlark.None, nil
				}
				return toStarlark(val)
			}
		}
		g, ok := r.conn.(KVGetter)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.get: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		val, err := g.Get(threadContext(thread), key)
		if err != nil {
			return nil, fmt.Errorf("ctx.%s.get: %w", r.primType, err)
		}
		if val == nil {
			return starlark.None, nil
		}
		return toStarlark(val)
	})
}

// builtinLog serves ctx.log.info/warn/error when the log primitive is
// routed through the datastore resolver (plan fase D) — entries go to the
// centralized backend instead of the in-memory logAPI.
func (r *primitiveRunner) builtinLog(level string) *starlark.Builtin {
	return starlark.NewBuiltin("log."+level, func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var event string
		var meta starlark.Value = starlark.None
		if err := starlark.UnpackArgs("log", args, kwargs,
			"event", &event,
			"meta?", &meta,
		); err != nil {
			return nil, err
		}
		lg, ok := r.conn.(Logger)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.%s: not yet implemented for this backend (connection=%q)", r.primType, level, r.name)
		}
		if err := lg.Log(threadContext(thread), level, event, starlarkValueToMap(meta)); err != nil {
			return nil, fmt.Errorf("ctx.%s.%s: %w", r.primType, level, err)
		}
		return starlark.None, nil
	})
}

func (r *primitiveRunner) builtinSet() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".set", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var key string
		var value starlark.Value
		var ttl int64
		if err := starlark.UnpackArgs("set", args, kwargs, "key", &key, "value", &value, "ttl?", &ttl); err != nil {
			return nil, err
		}
		s, ok := r.conn.(KVSetter)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.set: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		if err := s.Set(threadContext(thread), key, fromStarlark(value), time.Duration(ttl)*time.Second); err != nil {
			return nil, fmt.Errorf("ctx.%s.set: %w", r.primType, err)
		}
		return starlark.None, nil
	})
}

func (r *primitiveRunner) builtinDelete() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".delete", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var key string
		if err := starlark.UnpackArgs("delete", args, kwargs, "key", &key); err != nil {
			return nil, err
		}
		d, ok := r.conn.(KVDeleter)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.delete: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		if err := d.Delete(threadContext(thread), key); err != nil {
			return nil, fmt.Errorf("ctx.%s.delete: %w", r.primType, err)
		}
		return starlark.None, nil
	})
}

func (r *primitiveRunner) builtinAcquire() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".acquire", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var key string
		var ttl int64
		if err := starlark.UnpackArgs("acquire", args, kwargs, "key", &key, "ttl?", &ttl); err != nil {
			return nil, err
		}
		l, ok := r.conn.(Locker)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.acquire: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		acquired, err := l.Acquire(threadContext(thread), key, time.Duration(ttl)*time.Second)
		if err != nil {
			return nil, fmt.Errorf("ctx.%s.acquire: %w", r.primType, err)
		}
		return starlark.Bool(acquired), nil
	})
}

func (r *primitiveRunner) builtinRelease() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".release", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var key string
		if err := starlark.UnpackArgs("release", args, kwargs, "key", &key); err != nil {
			return nil, err
		}
		l, ok := r.conn.(Locker)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.release: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		if err := l.Release(threadContext(thread), key); err != nil {
			return nil, fmt.Errorf("ctx.%s.release: %w", r.primType, err)
		}
		return starlark.None, nil
	})
}

func (r *primitiveRunner) builtinEnqueue() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".enqueue", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var name string
		var payload starlark.Value
		if err := starlark.UnpackArgs("enqueue", args, kwargs, "name", &name, "payload", &payload); err != nil {
			return nil, err
		}
		q, ok := r.conn.(Queue)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.enqueue: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		if err := q.Enqueue(threadContext(thread), name, fromStarlark(payload)); err != nil {
			return nil, fmt.Errorf("ctx.%s.enqueue: %w", r.primType, err)
		}
		return starlark.None, nil
	})
}

func (r *primitiveRunner) builtinDequeue() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".dequeue", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var name string
		if err := starlark.UnpackArgs("dequeue", args, kwargs, "name", &name); err != nil {
			return nil, err
		}
		q, ok := r.conn.(Queue)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.dequeue: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		payload, err := q.Dequeue(threadContext(thread), name)
		if err != nil {
			return nil, fmt.Errorf("ctx.%s.dequeue: %w", r.primType, err)
		}
		sv, err := toStarlark(payload)
		if err != nil {
			return nil, err
		}
		return sv, nil
	})
}

func (r *primitiveRunner) builtinPublish() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".publish", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var channel string
		var payload starlark.Value
		if err := starlark.UnpackArgs("publish", args, kwargs, "channel", &channel, "payload", &payload); err != nil {
			return nil, err
		}
		ps, ok := r.conn.(PubSub)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.publish: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		if err := ps.Publish(threadContext(thread), channel, fromStarlark(payload)); err != nil {
			return nil, fmt.Errorf("ctx.%s.publish: %w", r.primType, err)
		}
		return starlark.None, nil
	})
}

func (r *primitiveRunner) builtinSubscribe() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".subscribe", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var channel string
		var cb starlark.Callable
		if err := starlark.UnpackArgs("subscribe", args, kwargs, "channel", &channel, "callback", &cb); err != nil {
			return nil, err
		}
		ps, ok := r.conn.(PubSub)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.subscribe: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		err := ps.Subscribe(threadContext(thread), channel, func(payload any) {
			sv, convErr := toStarlark(payload)
			if convErr != nil {
				return
			}
			_, _ = starlark.Call(thread, cb, starlark.Tuple{sv}, nil)
		})
		if err != nil {
			return nil, fmt.Errorf("ctx.%s.subscribe: %w", r.primType, err)
		}
		return starlark.None, nil
	})
}

func (r *primitiveRunner) builtinUpload() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".upload", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var path string
		var data starlark.Bytes
		if err := starlark.UnpackArgs("upload", args, kwargs, "path", &path, "data", &data); err != nil {
			return nil, err
		}
		s, ok := r.conn.(Storage)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.upload: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		if err := s.Upload(threadContext(thread), path, []byte(data)); err != nil {
			return nil, fmt.Errorf("ctx.%s.upload: %w", r.primType, err)
		}
		return starlark.None, nil
	})
}

func (r *primitiveRunner) builtinDownload() *starlark.Builtin {
	return starlark.NewBuiltin(r.primType+".download", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var path string
		if err := starlark.UnpackArgs("download", args, kwargs, "path", &path); err != nil {
			return nil, err
		}
		s, ok := r.conn.(Storage)
		if !ok {
			return starlark.None, fmt.Errorf("ctx.%s.download: not yet implemented for this backend (connection=%q)", r.primType, r.name)
		}
		data, err := s.Download(threadContext(thread), path)
		if err != nil {
			return nil, fmt.Errorf("ctx.%s.download: %w", r.primType, err)
		}
		return starlark.Bytes(data), nil
	})
}
