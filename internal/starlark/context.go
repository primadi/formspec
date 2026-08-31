// Package starlark — Script Context API
//
// This file provides the ctx.* object exposed to Starlark scripts.
// Scripts access runtime primitives via:
//
//	ctx.workspace.id
//	ctx.user.id
//	ctx.now()
//	ctx.log.info("event", {"key": "val"})
//	ctx.next_key("field_name")
//	ctx.db().query("SELECT ...")         // caller's bound datastore (module-scoped)
//	ctx.db.named("analytics-db").query(...)   // explicit named datastore
package starlark

import (
	"fmt"
	"strings"
	"time"

	"go.starlark.net/starlark"

	"github.com/primadi/formspec/pkg/spec"
)

// CtxAPI is the Starlark-callable ctx object.
// It provides access to workspace info, user info, logging, and primitives.
type CtxAPI struct {
	Workspace *workspaceInfo
	User      *userInfo
	Auth      *authInfo
	Now       func() time.Time
	Log       *logAPI
	NextKey   func(fieldName string) (string, error)
	Config    *configAPI
	Secrets   *secretsAPI
	Job       *jobAPI

	// RequestID is the correlation ID of the originating HTTP request
	// (todo 8.2.3, spec platform/09-observability.md §2.3). Read-only in
	// scripts as ctx.request_id — for log/trace correlation, never for
	// business branching.
	RequestID string

	// uses is the caller action's declared uses block (todo 2.6.4). When
	// strictPrimitives is true, accessing a ctx.* primitive not listed in
	// uses.primitives is rejected with a USES_VIOLATION error.
	uses             *spec.UsesDecl
	strictPrimitives bool

	// module is the owning module of the currently-executing script
	// (todo 2.9.4). ctx.* primitives resolve against this module's bound
	// datastore (platform/06-datastore.md §1.1) — a script can never reach
	// another module's datastore, even via .named().
	module         string
	dsResolver     func(primitiveType, name, module string) (interface{}, error)
	dsResolverImpl any // underlying resolver (may implement ResolveNamed — fase C)
	namedResolver  interface {
		ResolveNamed(primitiveType, alias, module string) (interface{}, error)
	}

	// Primitive handles — callable starlark values with .named() support.
	// Each is lazily initialized via the getter methods.
	db      *primitiveHandle
	cache   *primitiveHandle
	lock    *primitiveHandle
	queue   *primitiveHandle
	pubsub  *primitiveHandle
	storage *primitiveHandle
	kvstore *primitiveHandle

	// configPrim/logPrim are the datastore-resolver-backed overrides for
	// ctx.config/ctx.log (plan fase D). nil = builtin path (Config store /
	// in-memory log entries).
	configPrim *primitiveHandle
	logPrim    *primitiveHandle
}

var _ starlark.Value = (*CtxAPI)(nil)

// NewCtxAPI creates a ctx object with the given workspace and user info.
func NewCtxAPI(workspaceID, workspaceName, userID, userRole string, userPerms []string) *CtxAPI {
	return &CtxAPI{
		Workspace: &workspaceInfo{ID: workspaceID, Name: workspaceName},
		User:      &userInfo{ID: userID, Role: userRole, Permissions: userPerms},
		Auth:      &authInfo{},
		Now:       now,
		Log:       newLogAPI(),
	}
}

// SetUses records the caller action's declared uses block so ctx.* primitive
// access can be checked against it (todo 2.6.4).
func (c *CtxAPI) SetUses(uses *spec.UsesDecl) {
	c.uses = uses
}

// SetStrictPrimitives toggles strict enforcement of ctx.* primitive access
// against uses.primitives. Off by default (dev mode); enabled in
// ProdMode/StrictMode.
func (c *CtxAPI) SetStrictPrimitives(strict bool) {
	c.strictPrimitives = strict
}

// checkPrimitive enforces that a ctx.* primitive is declared in
// uses.primitives when strict mode is on. Returns nil when relaxed or when
// the primitive is declared.
func (c *CtxAPI) checkPrimitive(name string) error {
	if !c.strictPrimitives {
		return nil
	}
	if c.uses == nil {
		return fmt.Errorf("USES_VIOLATION: ctx.%s used but the action declares no uses block — add uses.primitives: [%s]", name, name)
	}
	for _, p := range c.uses.Primitives {
		if p == name {
			return nil
		}
	}
	return fmt.Errorf("USES_VIOLATION: ctx.%s used but not declared in uses.primitives — add %q to the action's uses.primitives", name, name)
}

// checkDatastoreAccess enforces the uses.datastores declaration (plan
// docs/plan/infra-registry-3-level.md fase B): when the action declares a
// datastores map, every ctx.<primitive> access must be covered by a key
// "primitive" or a named key "primitive/alias" (fase C). Actions without a
// datastores declaration are unrestricted (the primitives list remains the
// coarse gate). Returns DATASTORE_ACCESS_DENIED on violation — the error
// code from platform/06-datastore.md §6.
func (c *CtxAPI) checkDatastoreAccess(primitive string) error {
	if c.uses == nil || len(c.uses.Datastores) == 0 {
		return nil
	}
	if _, ok := c.uses.Datastores[primitive]; ok {
		return nil
	}
	// A named key ("db/analytics") also grants the base primitive — the
	// alias gate (checkDatastoreAlias) enforces the exact alias at
	// resolution time.
	for key := range c.uses.Datastores {
		if base, _, found := strings.Cut(key, "/"); found && base == primitive {
			return nil
		}
	}
	return fmt.Errorf("DATASTORE_ACCESS_DENIED: ctx.%s used but not declared in uses.datastores — add %q to the action's uses.datastores (platform/06-datastore.md §6)", primitive, primitive)
}

// namedPrefix marks a .named(alias) resolution request flowing through the
// same 2-arg resolver signature (plan fase C): ctx.db.named("analytics")
// resolves with name "named:analytics".
const namedPrefix = "named:"

// checkDatastoreAlias enforces the uses.datastores declaration for named
// logical primitives (plan fase C): ctx.db.named("analytics") requires the
// key "db/analytics" in the action's uses.datastores. Actions without a
// datastores declaration are unrestricted.
func (c *CtxAPI) checkDatastoreAlias(primitive, alias string) error {
	if c.uses == nil || len(c.uses.Datastores) == 0 {
		return nil
	}
	if _, ok := c.uses.Datastores[primitive+"/"+alias]; ok {
		return nil
	}
	return fmt.Errorf("DATASTORE_ACCESS_DENIED: ctx.%s.named(%q) used but %q is not declared in uses.datastores (platform/06-datastore.md §6)", primitive, alias, primitive+"/"+alias)
}

// SetModule records the module that owns the currently-executing script
// (todo 2.9.4). Primitive resolution reads it at call time, so ctx.db()
// without arguments resolves to the module's bound datastore
// (platform/06-datastore.md §1.1) even when SetDatastoreResolver ran first.
func (c *CtxAPI) SetModule(module string) {
	c.module = module
}

// SetDatastoreResolver sets the resolver function for datastore lookups.
// The resolver receives (primitiveType, name, module) and returns the
// underlying connection or error. `module` is the owning module of the
// executing script (set via SetModule) — the registry uses it to enforce
// module-scoped datastore binding (todo 2.9.4). This is called by the
// runtime after boot to wire the datastore registry into ctx.*.
//
// A name with the "named:" prefix (sent by .named(alias), plan fase C) is
// routed to the registry's named-logical-primitive resolution
// (ResolveNamed) with the alias gate (checkDatastoreAlias); plain names go
// through the module-scoped chain (Resolve).
func (c *CtxAPI) SetDatastoreResolver(resolver func(primitiveType, name, module string) (interface{}, error)) {
	c.dsResolver = resolver
	c.dsResolverImpl = resolver
	// Wrap the 3-arg resolver into the handle's 2-arg signature; the module
	// is read from c at resolution time (handles resolve lazily per call).
	wrapped := func(primitiveType, name string) (interface{}, error) {
		if alias, ok := strings.CutPrefix(name, namedPrefix); ok {
			if err := c.checkDatastoreAlias(primitiveType, alias); err != nil {
				return nil, err
			}
			return c.resolveNamed(primitiveType, alias)
		}
		return resolver(primitiveType, name, c.module)
	}
	c.db = newPrimitiveHandle("db", wrapped)
	c.cache = newPrimitiveHandle("cache", wrapped)
	c.lock = newPrimitiveHandle("lock", wrapped)
	c.queue = newPrimitiveHandle("queue", wrapped)
	c.pubsub = newPrimitiveHandle("pubsub", wrapped)
	c.storage = newPrimitiveHandle("storage", wrapped)
	c.kvstore = newPrimitiveHandle("kvstore", wrapped)
	// Fase D: config/log become routable — the builtin ctx.config/ctx.log
	// remain the fallback when no service serves them.
	c.configPrim = newPrimitiveHandle("config", wrapped)
	c.logPrim = newPrimitiveHandle("log", wrapped)
}

// resolveNamed dispatches a named logical primitive to the underlying
// resolver's named path. The registry exposes ResolveNamed; other resolver
// implementations (tests, sidecar) only implement the plain path — for
// those, the named request is forwarded verbatim so the backend can decide.
func (c *CtxAPI) resolveNamed(primitiveType, alias string) (interface{}, error) {
	type namedResolver interface {
		ResolveNamed(primitiveType, alias, module string) (interface{}, error)
	}
	if c.namedResolver != nil {
		return c.namedResolver.ResolveNamed(primitiveType, alias, c.module)
	}
	if nr, ok := c.dsResolverImpl.(namedResolver); ok {
		return nr.ResolveNamed(primitiveType, alias, c.module)
	}
	return c.dsResolver(primitiveType, namedPrefix+alias, c.module)
}

// SetDatastoreResolverNamed wires a dedicated named-logical-primitive
// resolver (plan fase C). When set, ctx.<primitive>.named(alias) resolves
// through it instead of probing the plain resolver for a ResolveNamed
// method. The alias gate (checkDatastoreAlias) still applies.
func (c *CtxAPI) SetDatastoreResolverNamed(nr interface {
	ResolveNamed(primitiveType, alias, module string) (interface{}, error)
}) {
	c.namedResolver = nr
}

// ─── starlark.Value interface ───

func (c *CtxAPI) String() string        { return "<ctx>" }
func (c *CtxAPI) Type() string          { return "ctx" }
func (c *CtxAPI) Freeze()               {}
func (c *CtxAPI) Truth() starlark.Bool  { return starlark.True }
func (c *CtxAPI) Hash() (uint32, error) { return 0, fmt.Errorf("ctx is not hashable") }

// Attr returns ctx attributes: .workspace, .user, .auth, .now, .log, .next_key, .config,
// plus primitives: .db, .cache, .lock, .queue, .pubsub, .storage, .kvstore
func (c *CtxAPI) Attr(name string) (starlark.Value, error) {
	switch name {
	case "workspace":
		return c.Workspace, nil
	case "user":
		return c.User, nil
	case "auth":
		return c.Auth, nil
	case "request_id":
		// Correlation ID of the originating request (todo 8.2.3). Empty
		// string (not None) when outside a request context (REPL, scheduler).
		return starlark.String(c.RequestID), nil
	case "now":
		return c.builtinNow(), nil
	case "today":
		return c.builtinToday(), nil
	case "log":
		// Fase D: when a service serves `log`, route through the resolver
		// (centralized sink); otherwise the builtin in-memory logAPI. A nil
		// connection or resolve error falls back to the builtin. The resolved
		// runner is returned directly — it exposes .info/.warn/.error (the
		// handle only exposes .named).
		if c.logPrim != nil {
			if runner, err := c.logPrim.CallInternal(nil, nil, nil); err == nil && runner != nil {
				if pr, ok := runner.(*primitiveRunner); ok && pr.conn != nil {
					return runner, nil
				}
			}
		}
		return c.Log, nil
	case "next_key":
		return c.builtinNextKey(), nil
	case "config":
		// Fase D: when a service serves `config`, route through the resolver
		// (centralized store); otherwise the builtin Config store. The
		// resolved runner is returned directly — it exposes .get.
		if c.configPrim != nil {
			if runner, err := c.configPrim.CallInternal(nil, nil, nil); err == nil && runner != nil {
				if pr, ok := runner.(*primitiveRunner); ok && pr.conn != nil {
					return runner, nil
				}
			}
		}
		return c.Config, nil
	case "job":
		if c.Job == nil {
			return starlark.None, fmt.Errorf("ctx.job: not inside a tracked async job (call: async + track: true)")
		}
		return c.Job, nil
	case "db":
		if err := c.checkPrimitive("db"); err != nil {
			return nil, err
		}
		if err := c.checkDatastoreAccess("db"); err != nil {
			return nil, err
		}
		if c.db == nil {
			return starlark.None, fmt.Errorf("ctx.db: datastore resolver not configured")
		}
		return c.db, nil
	case "cache":
		if err := c.checkPrimitive("cache"); err != nil {
			return nil, err
		}
		if err := c.checkDatastoreAccess("cache"); err != nil {
			return nil, err
		}
		if c.cache == nil {
			return starlark.None, fmt.Errorf("ctx.cache: datastore resolver not configured")
		}
		return c.cache, nil
	case "lock":
		if err := c.checkPrimitive("lock"); err != nil {
			return nil, err
		}
		if err := c.checkDatastoreAccess("lock"); err != nil {
			return nil, err
		}
		if c.lock == nil {
			return starlark.None, fmt.Errorf("ctx.lock: datastore resolver not configured")
		}
		return c.lock, nil
	case "queue":
		if err := c.checkPrimitive("queue"); err != nil {
			return nil, err
		}
		if err := c.checkDatastoreAccess("queue"); err != nil {
			return nil, err
		}
		if c.queue == nil {
			return starlark.None, fmt.Errorf("ctx.queue: datastore resolver not configured")
		}
		return c.queue, nil
	case "pubsub":
		if err := c.checkPrimitive("pubsub"); err != nil {
			return nil, err
		}
		if err := c.checkDatastoreAccess("pubsub"); err != nil {
			return nil, err
		}
		if c.pubsub == nil {
			return starlark.None, fmt.Errorf("ctx.pubsub: datastore resolver not configured")
		}
		return c.pubsub, nil
	case "storage":
		if err := c.checkPrimitive("storage"); err != nil {
			return nil, err
		}
		if err := c.checkDatastoreAccess("storage"); err != nil {
			return nil, err
		}
		if c.storage == nil {
			return starlark.None, fmt.Errorf("ctx.storage: datastore resolver not configured")
		}
		return c.storage, nil
	case "kvstore":
		if err := c.checkPrimitive("kvstore"); err != nil {
			return nil, err
		}
		if err := c.checkDatastoreAccess("kvstore"); err != nil {
			return nil, err
		}
		if c.kvstore == nil {
			return starlark.None, fmt.Errorf("ctx.kvstore: datastore resolver not configured")
		}
		return c.kvstore, nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("ctx has no .%s attribute", name))
	}
}

func (c *CtxAPI) AttrNames() []string {
	return []string{"workspace", "user", "auth", "now", "today", "log", "next_key", "config", "job",
		"db", "cache", "lock", "queue", "pubsub", "storage", "kvstore"}
}

func (c *CtxAPI) builtinNow() *starlark.Builtin {
	return starlark.NewBuiltin("ctx.now", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		if c.Now == nil {
			return starlark.String(time.Now().UTC().Format(time.RFC3339)), nil
		}
		return starlark.String(c.Now().UTC().Format(time.RFC3339)), nil
	})
}

// builtinToday returns ctx.today(), a date-only (YYYY-MM-DD) counterpart to
// ctx.now() for scripts that set `date`-typed fields (e.g. transaction_date).
func (c *CtxAPI) builtinToday() *starlark.Builtin {
	return starlark.NewBuiltin("ctx.today", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		if c.Now == nil {
			return starlark.String(time.Now().UTC().Format("2006-01-02")), nil
		}
		return starlark.String(c.Now().UTC().Format("2006-01-02")), nil
	})
}

func (c *CtxAPI) builtinNextKey() *starlark.Builtin {
	return starlark.NewBuiltin("ctx.next_key", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var fieldName string
		if err := starlark.UnpackArgs("next_key", args, kwargs, "field", &fieldName); err != nil {
			return nil, err
		}

		if c.NextKey == nil {
			return nil, fmt.Errorf("ctx.next_key: no key generator registered")
		}

		key, err := c.NextKey(fieldName)
		if err != nil {
			return nil, fmt.Errorf("ctx.next_key: %w", err)
		}
		return starlark.String(key), nil
	})
}

// ─── Sub-types ───

type workspaceInfo struct {
	ID   string
	Name string
}

var _ starlark.Value = (*workspaceInfo)(nil)

func (t *workspaceInfo) String() string        { return fmt.Sprintf("<workspace %s>", t.ID) }
func (t *workspaceInfo) Type() string          { return "workspace" }
func (t *workspaceInfo) Freeze()               {}
func (t *workspaceInfo) Truth() starlark.Bool  { return starlark.True }
func (t *workspaceInfo) Hash() (uint32, error) { return 0, fmt.Errorf("workspace is not hashable") }

func (t *workspaceInfo) Attr(name string) (starlark.Value, error) {
	switch name {
	case "id":
		return starlark.String(t.ID), nil
	case "name":
		return starlark.String(t.Name), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("workspace has no .%s", name))
	}
}

func (t *workspaceInfo) AttrNames() []string { return []string{"id", "name"} }

type userInfo struct {
	ID          string
	Role        string
	Permissions []string
}

var _ starlark.Value = (*userInfo)(nil)

func (u *userInfo) String() string        { return fmt.Sprintf("<user %s>", u.ID) }
func (u *userInfo) Type() string          { return "user" }
func (u *userInfo) Freeze()               {}
func (u *userInfo) Truth() starlark.Bool  { return starlark.True }
func (u *userInfo) Hash() (uint32, error) { return 0, fmt.Errorf("user is not hashable") }

func (u *userInfo) Attr(name string) (starlark.Value, error) {
	switch name {
	case "id":
		return starlark.String(u.ID), nil
	case "role":
		return starlark.String(u.Role), nil
	case "permissions":
		perms := make([]starlark.Value, len(u.Permissions))
		for i, p := range u.Permissions {
			perms[i] = starlark.String(p)
		}
		return starlark.NewList(perms), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("user has no .%s", name))
	}
}

func (u *userInfo) AttrNames() []string { return []string{"id", "role", "permissions"} }

type authInfo struct{}

var _ starlark.Value = (*authInfo)(nil)

func (a *authInfo) String() string        { return "<auth>" }
func (a *authInfo) Type() string          { return "auth" }
func (a *authInfo) Freeze()               {}
func (a *authInfo) Truth() starlark.Bool  { return starlark.True }
func (a *authInfo) Hash() (uint32, error) { return 0, fmt.Errorf("auth is not hashable") }

func (a *authInfo) Attr(name string) (starlark.Value, error) {
	if name == "has" {
		return starlark.NewBuiltin("auth.has", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			return starlark.True, nil // TODO: permission check via ctx.auth.has(perm)
		}), nil
	}
	return nil, starlark.NoSuchAttrError(fmt.Sprintf("auth has no .%s", name))
}

func (a *authInfo) AttrNames() []string { return []string{"has"} }

// ─── ctx.log ───

type logAPI struct {
	entries []LogEntry
}

// LogEntry is a recorded log message from a script execution.
type LogEntry struct {
	Level string
	Event string
	Meta  map[string]any
}

func newLogAPI() *logAPI {
	return &logAPI{}
}

var _ starlark.Value = (*logAPI)(nil)

func (l *logAPI) String() string        { return "<log>" }
func (l *logAPI) Type() string          { return "log" }
func (l *logAPI) Freeze()               {}
func (l *logAPI) Truth() starlark.Bool  { return starlark.True }
func (l *logAPI) Hash() (uint32, error) { return 0, fmt.Errorf("log is not hashable") }

func (l *logAPI) Attr(name string) (starlark.Value, error) {
	switch name {
	case "info":
		return l.builtinLog("info"), nil
	case "warn":
		return l.builtinLog("warn"), nil
	case "error":
		return l.builtinLog("error"), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("log has no .%s", name))
	}
}

func (l *logAPI) AttrNames() []string { return []string{"info", "warn", "error"} }

func (l *logAPI) builtinLog(level string) *starlark.Builtin {
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

		l.entries = append(l.entries, LogEntry{
			Level: level,
			Event: event,
			Meta:  starlarkValueToMap(meta),
		})
		return starlark.None, nil
	})
}

// Entries returns all recorded log entries.
func (l *logAPI) Entries() []LogEntry { return l.entries }

// ─── ctx.job ───

// JobReporter reports progress for the currently-executing tracked async job
// (02-core-extended.md §13, todo 7.13). Wired by the runtime with the job's
// workspace + id captured; nil when the action is not a tracked async job.
type JobReporter func(pct int, message string) error

// jobAPI exposes ctx.job.progress(pct, message) to scripts running inside a
// tracked async job. When no reporter is wired (not a tracked job), progress
// is a no-op.
type jobAPI struct {
	reporter JobReporter
}

// NewJobAPI creates a ctx.job handle backed by the given reporter.
func NewJobAPI(reporter JobReporter) *jobAPI {
	return &jobAPI{reporter: reporter}
}

var _ starlark.Value = (*jobAPI)(nil)

func (j *jobAPI) String() string        { return "<job>" }
func (j *jobAPI) Type() string          { return "job" }
func (j *jobAPI) Freeze()               {}
func (j *jobAPI) Truth() starlark.Bool  { return starlark.True }
func (j *jobAPI) Hash() (uint32, error) { return 0, fmt.Errorf("job is not hashable") }

func (j *jobAPI) Attr(name string) (starlark.Value, error) {
	switch name {
	case "progress":
		return starlark.NewBuiltin("job.progress", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			var pct int
			var message string
			if err := starlark.UnpackArgs("job.progress", args, kwargs,
				"pct", &pct,
				"message?", &message,
			); err != nil {
				return nil, err
			}
			if j.reporter != nil {
				if err := j.reporter(pct, message); err != nil {
					return nil, err
				}
			}
			return starlark.None, nil
		}), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("job has no .%s", name))
	}
}

func (j *jobAPI) AttrNames() []string { return []string{"progress"} }

// ─── ctx.config ───

type configAPI struct {
	store map[string]any
}

// NewConfigAPI creates a config context backed by the given key-value store.
func NewConfigAPI(store map[string]any) *configAPI {
	return &configAPI{store: store}
}

var _ starlark.Value = (*configAPI)(nil)

func (c *configAPI) String() string        { return "<config>" }
func (c *configAPI) Type() string          { return "config" }
func (c *configAPI) Freeze()               {}
func (c *configAPI) Truth() starlark.Bool  { return starlark.True }
func (c *configAPI) Hash() (uint32, error) { return 0, fmt.Errorf("config is not hashable") }

func (c *configAPI) Attr(name string) (starlark.Value, error) {
	if name == "get" {
		return c.builtinGet(), nil
	}
	return nil, starlark.NoSuchAttrError(fmt.Sprintf("config has no .%s", name))
}

func (c *configAPI) AttrNames() []string { return []string{"get"} }

func (c *configAPI) builtinGet() *starlark.Builtin {
	return starlark.NewBuiltin("config.get", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var key string
		var defaultVal starlark.Value = starlark.None
		if err := starlark.UnpackArgs("get", args, kwargs,
			"key", &key,
			"default?", &defaultVal,
		); err != nil {
			return nil, err
		}

		if c.store == nil {
			return defaultVal, nil
		}

		val, ok := c.store[key]
		if !ok {
			return defaultVal, nil
		}
		return toStarlark(val)
	})
}

// ─── ctx.secrets ───

// secretsAPI serves ctx.secrets().get(key) — the ONLY path for reading
// `secret: true` Config keys (todo 6.8.1). It enforces that the key is
// declared in the caller action's uses.secrets (todo 6.8.2), never logs the
// value (todo 6.8.3), and audits every read (todo 6.8.4).
type secretsAPI struct {
	store   map[string]string // key → secret value
	allowed map[string]bool   // declared uses.secrets keys
	audit   func(key string)  // audit hook (todo 6.8.4)
}

// NewSecretsAPI creates a secrets context backed by the given store, allowing
// only the declared keys, and calling audit on each successful read.
func NewSecretsAPI(store map[string]string, allowed []string, audit func(key string)) *secretsAPI {
	allowedSet := map[string]bool{}
	for _, k := range allowed {
		allowedSet[k] = true
	}
	return &secretsAPI{store: store, allowed: allowedSet, audit: audit}
}

var _ starlark.Value = (*secretsAPI)(nil)

func (s *secretsAPI) String() string        { return "<secrets>" }
func (s *secretsAPI) Type() string          { return "secrets" }
func (s *secretsAPI) Freeze()               {}
func (s *secretsAPI) Truth() starlark.Bool  { return starlark.True }
func (s *secretsAPI) Hash() (uint32, error) { return 0, fmt.Errorf("secrets is not hashable") }

func (s *secretsAPI) Attr(name string) (starlark.Value, error) {
	if name == "get" {
		return s.builtinGet(), nil
	}
	return nil, starlark.NoSuchAttrError(fmt.Sprintf("secrets has no .%s", name))
}

func (s *secretsAPI) AttrNames() []string { return []string{"get"} }

func (s *secretsAPI) builtinGet() *starlark.Builtin {
	return starlark.NewBuiltin("secrets.get", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var key string
		if err := starlark.UnpackArgs("get", args, kwargs, "key", &key); err != nil {
			return nil, err
		}

		// todo 6.8.2: only declared secrets are accessible.
		if !s.allowed[key] {
			return nil, fmt.Errorf("ctx.secrets: key %q not declared in uses.secrets", key)
		}

		val, ok := s.store[key]
		if !ok {
			return starlark.None, nil
		}

		// todo 6.8.4: audit every secret read. The value itself is never
		// logged (todo 6.8.3).
		if s.audit != nil {
			s.audit(key)
		}
		return starlark.String(val), nil
	})
}
