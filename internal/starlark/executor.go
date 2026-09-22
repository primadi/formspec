// Package starlark — Script Executor
//
// This file provides ExecuteScript, which loads a Starlark .star file, binds
// the resource and ctx objects, and executes the `def execute(resource, params, ctx)`
// function. The result is either ok() or fail(msg).
package starlark

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/primadi/formspec/internal/observability"
	"github.com/primadi/formspec/pkg/spec"
)

// declaredUsesResources returns the caller action's declared uses.resources
// as a []string, or nil when the action declares no uses block (todo 2.6.4).
func declaredUsesResources(uses *spec.UsesDecl) []string {
	if uses == nil {
		return nil
	}
	return uses.Resources
}

// declaredUsesSecrets returns the caller action's declared uses.secrets keys
// as a []string, or nil when the action declares no uses block (todo 6.8.2).
func declaredUsesSecrets(uses *spec.UsesDecl) []string {
	if uses == nil {
		return nil
	}
	return uses.Secrets
}

// FailError is the error a script's fail(msg) raises. It carries the author's
// own message so the caller reports that message verbatim rather than a
// wrapped Starlark runtime error.
type FailError struct {
	Message string
}

func (e *FailError) Error() string { return e.Message }

// ScriptResult is the outcome of a script execution.
type ScriptResult struct {
	// OK is true if the script returned ok().
	OK bool
	// Data is the return value from ok(data).
	Data map[string]any
	// Error is the error message if the script returned fail(msg) or crashed.
	Error string
	// LogEntries are all log messages from ctx.log during execution.
	LogEntries []LogEntry
	// Elapsed is the wall-clock duration of execution.
	Elapsed time.Duration
}

// ExecuteScript runs a Starlark script from a .star file.
//
// Parameters:
//   - ctx: the Go context for the execution; stored on the Starlark thread so
//     ctx.* primitive operations (ctx.db().query, ctx.cache().get, ...) carry
//     the request context into backend calls.
//   - scriptPath: absolute path to the .star file
//   - resource: the ResourceAPI for the current entity record
//   - params: input parameters from the action request
//   - ctx: the CtxAPI providing workspace, user, log, etc.
//
// The script MUST define: def execute(resource, params, ctx)
// It signals success via: return ok(data)  or  return ok()
// It signals failure via: return fail("message")  or  return fail({"field": "error"})
func ExecuteScript(ctx context.Context, scriptPath string, resource *ResourceAPI, params map[string]any, ctxObj *CtxAPI) (*ScriptResult, error) {
	start := time.Now()

	// Validate the script file exists
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("script not found: %s: %w", scriptPath, err)
	}

	// Sandbox hard limits (todo 7.14): wall-clock timeout + iteration cap.
	// The timeout context is threaded into primitive operations so backend
	// calls respect it; SetMaxExecutionSteps bounds CPU-bound loops.
	execCtx, cancel := context.WithTimeout(ctx, DefaultWallClockTimeout*time.Millisecond)
	defer cancel()

	// Load and execute the Starlark module
	thread := &starlark.Thread{
		Name:  filepath.Base(scriptPath),
		Print: func(_ *starlark.Thread, msg string) {},
		Load:  nil, // no imports in sandbox
	}
	// Iteration cap (7.14.1): 100K bytecode steps — aborts runaway loops.
	thread.SetMaxExecutionSteps(DefaultMaxExecutionSteps)
	// Store the Go context on the thread so ctx.* primitive operations can
	// retrieve it (see threadContext in primitive.go).
	thread.SetLocal(ctxKey, execCtx)
	// Store the resource-usage limits on the thread (see limits.go).
	thread.SetLocal(limitsKey, NewScriptLimits())

	// Predeclare the built-in result helpers: ok() and fail()
	predeclared := starlark.StringDict{
		"ok": starlark.NewBuiltin("ok", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			var data starlark.Value = starlark.None
			if err := starlark.UnpackArgs("ok", args, kwargs, "data?", &data); err != nil {
				return nil, err
			}
			// Return a marker struct that indicates success
			return starlarkstruct.FromStringDict(starlark.String("ok_result"), starlark.StringDict{
				"ok":   starlark.True,
				"data": data,
			}), nil
		}),
		"fail": starlark.NewBuiltin("fail", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			var msg string
			if err := starlark.UnpackArgs("fail", args, kwargs, "msg", &msg); err != nil {
				return nil, err
			}
			// fail() must ABORT the script, not merely return a value: a
			// returning builtin lets execution continue on the next statement,
			// so a guard-style `if bad: fail(...)` fell through and the script
			// kept going with its data still unset — creating a half-built
			// record and reporting ok(). Returning an error unwinds the
			// Starlark stack immediately, which is what the contract
			// (06-script-runtime.md §1: the entrypoint returns ok() or fail(msg))
			// has always meant.
			return nil, &FailError{Message: msg}
		}),
	}

	// Money accessors (S7): `resource.total` is the object {amount, currency},
	// and money supports direct arithmetic (+ - * / and comparisons) so totals
	// do not need to be unwrapped. amount(x)/currency(x) extract the parts
	// explicitly when a plain number is wanted.
	for name, b := range moneyBuiltins() {
		predeclared[name] = b
	}

	// Compile (or reuse the cached compiled program for) the .star file, then
	// initialize it fresh for this call — Init() must run per-invocation to
	// get a private globals dict, even though the compiled *Program itself
	// is shared/cached across calls.
	prog, err := globalProgramCache.getProgram(scriptPath, func(name string) bool {
		_, ok := predeclared[name]
		return ok
	})
	if err != nil {
		return &ScriptResult{
			OK:      false,
			Error:   fmt.Sprintf("script compile error: %v", err),
			Elapsed: time.Since(start),
		}, nil
	}
	globals, err := prog.Init(thread, predeclared)
	if err != nil {
		return &ScriptResult{
			OK:      false,
			Error:   fmt.Sprintf("script runtime error: %v", err),
			Elapsed: time.Since(start),
		}, nil
	}

	// Find the execute function
	executeFn, ok := globals["execute"]
	if !ok {
		return &ScriptResult{
			OK:      false,
			Error:   "script does not define execute(resource, params, ctx)",
			Elapsed: time.Since(start),
		}, nil
	}

	// Build params as a Starlark dict
	paramsDict := starlark.NewDict(len(params))
	for k, v := range params {
		sv, err := toStarlark(v)
		if err != nil {
			return nil, fmt.Errorf("convert param %q: %w", k, err)
		}
		if err := paramsDict.SetKey(starlark.String(k), sv); err != nil {
			return nil, fmt.Errorf("set param %q: %w", k, err)
		}
	}

	// Call execute(resource, params, ctx)
	result, err := starlark.Call(thread, executeFn, starlark.Tuple{resource, paramsDict, ctxObj}, nil)
	if err != nil {
		// A fail() in the script is an expected, authored outcome — report the
		// author's message as the script's error, not as a runtime fault.
		var fe *FailError
		if errors.As(err, &fe) {
			return &ScriptResult{
				OK:         false,
				Error:      fe.Message,
				LogEntries: ctxObj.Log.Entries(),
				Elapsed:    time.Since(start),
			}, nil
		}
		return &ScriptResult{
			OK:      false,
			Error:   fmt.Sprintf("script runtime error: %v", err),
			Elapsed: time.Since(start),
		}, nil
	}

	// Parse the result
	sr := &ScriptResult{
		OK:         true,
		LogEntries: ctxObj.Log.Entries(),
		Elapsed:    time.Since(start),
	}

	// Check if result is an ok_result / fail_result struct
	if s, ok := result.(*starlarkstruct.Struct); ok {
		if okVal, err := s.Attr("ok"); err == nil {
			if okVal == starlark.False {
				sr.OK = false
				if msgVal, err := s.Attr("message"); err == nil {
					sr.Error = fmt.Sprint(msgVal)
				}
				return sr, nil
			}
		}
		if dataVal, err := s.Attr("data"); err == nil && dataVal != nil && dataVal != starlark.None {
			sr.Data = fromStarlarkValue(dataVal)
		}
	}

	// If the script simply returned None, treat as ok with no data
	return sr, nil
}

// fromStarlarkValue converts a starlark.Value to a Go map[string]any for ok(data).
func fromStarlarkValue(v starlark.Value) map[string]any {
	switch x := v.(type) {
	case *starlark.Dict:
		result := make(map[string]any, x.Len())
		for _, item := range x.Items() {
			key, _ := starlark.AsString(item[0])
			result[key] = fromStarlark(item[1])
		}
		return result
	case starlark.NoneType:
		return nil
	default:
		return map[string]any{"value": fromStarlark(v)}
	}
}

// ─── Action Executor (ScriptExecutor) ───

// ScriptExecutor implements action.Executor for impl types script and script_ref.
type ScriptExecutor struct {
	// ScriptPathResolver resolves impl refs (e.g. "billing/order_checkout") to
	// absolute file paths (e.g. "/spec/modules/billing/scripts/order_checkout.star").
	ScriptPathResolver func(ref string) (string, error)

	// SaveHandler is the save function for resource operations. version is the
	// caller's current known record version, threaded through for optimistic
	// concurrency (CAS) — see db.UpdateParams.Version. ctx carries the
	// request-scoped TxScope (if any) so this write joins the same
	// transaction as everything else in the current action execution — see
	// renderers/jsonb-persist/txscope.go.
	SaveHandler func(ctx context.Context, workspaceID, module, entity, id string, version int, data map[string]any) error

	// CallHandler is the cross-resource call function. callerResources is
	// the calling action's declared uses.resources (todo 2.6.4) — the
	// resource layer checks cross-module calls against it. targetID is the
	// record the target action runs against, or "" for a collection-level
	// action (resource.call("module.entity.<id>", ...)).
	CallHandler func(ctx context.Context, workspaceID, fromModule, targetModule, targetEntity, targetID, action string, params map[string]any, callerResources []string) (any, error)

	// LoadHandler loads another entity by ID, returning its data, version, and
	// the resolved record ID (a natural-key request resolves to a different,
	// UUID identity).
	// callerResources is the calling action's declared uses.resources.
	LoadHandler func(ctx context.Context, workspaceID, fromModule, module, entity, id string, callerResources []string) (map[string]any, int, string, error)

	// FindHandler finds another entity by field values (#31), returning its data,
	// version, and the resolved record ID, or (nil, 0, "", nil) when no row
	// matches. callerResources is the calling action's declared uses.resources.
	FindHandler func(ctx context.Context, workspaceID, fromModule, module, entity string, match map[string]any, callerResources []string) (map[string]any, int, string, error)

	// UpsertHandler writes a row of a `characteristic: summary` projection
	// (item 4.1), matching on `match` and merging `data`. It is the ONE
	// supported write path for summary entities. The wiring layer enforces that
	// the running script is the entity's `maintained_by`; this handler only
	// forwards the call. Returns the row id and whether a new row was created.
	UpsertHandler func(ctx context.Context, workspaceID, fromModule, module, entity string, match, data map[string]any, callerResources []string) (string, bool, error)

	// MaintainerRef is the script ref (e.g. "cafe-stock/stock_level_apply") of
	// the script currently executing, when known. The wiring layer compares it
	// against a summary entity's `maintained_by` to authorize resource.upsert
	// (item 4.1). Empty when the caller is not a named script.
	MaintainerRef string

	// CreateHandler creates a new record of another entity, returning its ID.
	// callerResources is the calling action's declared uses.resources.
	CreateHandler func(ctx context.Context, workspaceID, fromModule, module, entity string, data map[string]any, callerResources []string) (string, error)

	// NextKeyHandler generates natural keys, scoped to the entity that owns
	// the field (natural key counters are per module/entity/field).
	NextKeyHandler func(ctx context.Context, workspaceID, module, entity, fieldName, scope string) (string, error)

	// UnitConvertHandler converts a value between units of a field's declared
	// unit dimension (S12, item 4.6). It resolves the entity/field's `unit:`
	// declaration and applies its factors.
	UnitConvertHandler func(ctx context.Context, workspaceID, fromModule, entity, field string, value float64, from, to string) (float64, error)

	// DatastoreResolver resolves a ctx primitive ("db", "cache", "lock", ...)
	// and datastore name ("default" or a named datastore) to a live
	// connection. The third argument is the owning module of the executing
	// script — the resolver enforces module-scoped datastore binding
	// (todo 2.9.4, platform/06-datastore.md §1.1). It is wired into the
	// CtxAPI before script execution so ctx.db()/ctx.cache()/... resolve to
	// real backends instead of failing with "datastore resolver not
	// configured" (todo 2.9.1). A nil resolver keeps the current behavior
	// (every ctx.* primitive errors).
	DatastoreResolver func(primitiveType, name, module string) (interface{}, error)

	// DatastoreResolverNamed resolves named logical primitives (plan fase
	// C): ctx.db.named("analytics") → the service registered under
	// "db/analytics" in the owning App's App Registry. Nil = named access
	// falls back to probing DatastoreResolver for a ResolveNamed method.
	DatastoreResolverNamed interface {
		ResolveNamed(primitiveType, alias, module string) (interface{}, error)
	}

	// StrictPrimitives enables strict enforcement of ctx.* primitive access
	// against the caller action's uses.primitives (todo 2.6.4). Off by
	// default (dev mode); enabled in ProdMode/StrictMode.
	StrictPrimitives bool

	// SecretsStore holds `secret: true` Config key values, keyed by name.
	// It backs ctx.secrets().get(key) (todo 6.8.1). A nil store means no
	// secrets are available.
	SecretsStore map[string]string

	// SecretsAudit is called on every successful ctx.secrets().get(key)
	// (todo 6.8.4). May be nil.
	SecretsAudit func(key string)

	// ConfigStore holds resolved non-secret Config key values, keyed by name
	// (todo 7.2.1/7.2.2). It backs ctx.config.get(key). A nil store means no
	// config values are available (ctx.config.get returns the default).
	ConfigStore map[string]any
}

// SetDatastoreResolver sets the resolver used to wire ctx.* primitives to
// live datastore connections. See DatastoreResolver.
func (e *ScriptExecutor) SetDatastoreResolver(resolver func(primitiveType, name, module string) (interface{}, error)) {
	e.DatastoreResolver = resolver
}

// SetDatastoreResolverNamed wires the named-logical-primitive resolver (plan
// fase C). See DatastoreResolverNamed.
func (e *ScriptExecutor) SetDatastoreResolverNamed(nr interface {
	ResolveNamed(primitiveType, alias, module string) (interface{}, error)
}) {
	e.DatastoreResolverNamed = nr
}

// SetStrictPrimitives toggles strict ctx.* primitive enforcement (todo 2.6.4).
func (e *ScriptExecutor) SetStrictPrimitives(strict bool) {
	e.StrictPrimitives = strict
}

// SetSecretsStore wires the secret Config values backing ctx.secrets (todo 6.8.1).
func (e *ScriptExecutor) SetSecretsStore(store map[string]string) {
	e.SecretsStore = store
}

// SetSecretsAudit wires the audit hook called on each secret read (todo 6.8.4).
func (e *ScriptExecutor) SetSecretsAudit(audit func(key string)) {
	e.SecretsAudit = audit
}

// SetConfigStore wires the resolved non-secret Config values backing
// ctx.config (todo 7.2.2).
func (e *ScriptExecutor) SetConfigStore(store map[string]any) {
	e.ConfigStore = store
}

// NewScriptExecutor creates a ScriptExecutor with the given resolution function.
func NewScriptExecutor(resolver func(ref string) (string, error)) *ScriptExecutor {
	return &ScriptExecutor{
		ScriptPathResolver: resolver,
	}
}

// Execute runs a Starlark script for an action. resourceVersion is the
// current known version of the record (for CAS on resource.save()). ctx
// carries the request-scoped TxScope (if any, set by the caller) through
// to every handler — so a script's resource.save()/create()/load()/call()
// calls all join the same transaction as the rest of the action's
// execution instead of each opening its own. jobReporter, when non-nil,
// wires ctx.job.progress for a tracked async job (todo 7.13).
func (e *ScriptExecutor) Execute(ctx context.Context, scriptPath string, module, entity, id string, resourceData map[string]any, params map[string]any, workspaceID, userID string, resourceVersion int, uses *spec.UsesDecl, jobReporter JobReporter) (*ScriptResult, error) {
	// Build resource API
	res := NewResourceAPI(module, entity, id, resourceVersion, resourceData)
	callerResources := declaredUsesResources(uses)
	if e.SaveHandler != nil {
		res.SetSaveFunc(func(m, ent, rid string, v int, data map[string]any) error {
			return e.SaveHandler(ctx, workspaceID, m, ent, rid, v, data)
		})
	}
	if e.CallHandler != nil {
		res.SetCallFunc(func(targetModule, targetEntity, targetID, actionName string, p map[string]any) (any, error) {
			// Infer target module from the calling context if not specified
			if targetModule == "" {
				targetModule = module
			}
			return e.CallHandler(ctx, workspaceID, module, targetModule, targetEntity, targetID, actionName, p, callerResources)
		})
	}
	if e.LoadHandler != nil {
		res.SetLoadFunc(func(m, ent, eid string) (map[string]any, int, string, error) {
			return e.LoadHandler(ctx, workspaceID, module, m, ent, eid, callerResources)
		})
	}
	if e.FindHandler != nil {
		res.SetFindFunc(func(m, ent string, match map[string]any) (map[string]any, int, string, error) {
			return e.FindHandler(ctx, workspaceID, module, m, ent, match, callerResources)
		})
	}
	if e.UpsertHandler != nil {
		res.SetUpsertFunc(func(m, ent string, match, data map[string]any) (string, bool, error) {
			return e.UpsertHandler(ctx, workspaceID, module, m, ent, match, data, callerResources)
		})
	}
	if e.CreateHandler != nil {
		res.SetCreateFunc(func(m, ent string, data map[string]any) (string, error) {
			return e.CreateHandler(ctx, workspaceID, module, m, ent, data, callerResources)
		})
	}

	// Build ctx
	ctxObj := NewCtxAPI(workspaceID, "", userID, "", nil)
	// Module-scoped ctx.* primitives (todo 2.9.4): the executing script's
	// owning module determines which datastore ctx.db() resolves to.
	ctxObj.SetModule(module)
	ctxObj.SetUses(uses)
	ctxObj.SetStrictPrimitives(e.StrictPrimitives)
	// ctx.request_id (todo 8.2.3): propagate the correlation ID from the
	// HTTP boundary into the script for log/trace correlation.
	ctxObj.RequestID = observability.RequestIDFromContext(ctx)
	if e.DatastoreResolver != nil {
		ctxObj.SetDatastoreResolver(e.DatastoreResolver)
	}
	if e.DatastoreResolverNamed != nil {
		ctxObj.SetDatastoreResolverNamed(e.DatastoreResolverNamed)
	}
	if e.NextKeyHandler != nil {
		ctxObj.NextKey = func(fieldName, scope string) (string, error) {
			return e.NextKeyHandler(ctx, workspaceID, module, entity, fieldName, scope)
		}
	}
	if e.UnitConvertHandler != nil {
		ctxObj.UnitConvert = func(targetEntity, field string, value float64, from, to string) (float64, error) {
			return e.UnitConvertHandler(ctx, workspaceID, module, targetEntity, field, value, from, to)
		}
	}
	// ctx.secrets (todo 6.8): only keys declared in uses.secrets are readable.
	if e.SecretsStore != nil {
		ctxObj.Secrets = NewSecretsAPI(e.SecretsStore, declaredUsesSecrets(uses), e.SecretsAudit)
	}
	// ctx.config (todo 7.2.2): non-secret Config keys. When no store is
	// wired, ctx.config.get returns the caller's default (or None).
	if e.ConfigStore != nil {
		ctxObj.Config = NewConfigAPI(e.ConfigStore)
	}
	// ctx.job (todo 7.13): progress reporting for tracked async jobs. When no
	// reporter is wired (not a tracked job), ctx.job.progress is a no-op.
	if jobReporter != nil {
		ctxObj.Job = NewJobAPI(jobReporter)
	}
	ctxObj.Now = now

	return ExecuteScript(ctx, scriptPath, res, params, ctxObj)
}
