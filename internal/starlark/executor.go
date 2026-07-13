// Package starlark — Script Executor
//
// This file provides ExecuteScript, which loads a Starlark .star file, binds
// the resource and ctx objects, and executes the `def execute(resource, params, ctx)`
// function. The result is either ok() or fail(msg).
package starlark

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

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
//   - scriptPath: absolute path to the .star file
//   - resource: the ResourceAPI for the current entity record
//   - params: input parameters from the action request
//   - ctx: the CtxAPI providing workspace, user, log, etc.
//
// The script MUST define: def execute(resource, params, ctx)
// It signals success via: return ok(data)  or  return ok()
// It signals failure via: return fail("message")  or  return fail({"field": "error"})
func ExecuteScript(scriptPath string, resource *ResourceAPI, params map[string]any, ctxObj *CtxAPI) (*ScriptResult, error) {
	start := time.Now()

	// Validate the script file exists
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("script not found: %s: %w", scriptPath, err)
	}

	// Load and execute the Starlark module
	thread := &starlark.Thread{
		Name:  filepath.Base(scriptPath),
		Print: func(_ *starlark.Thread, msg string) {},
		Load:  nil, // no imports in sandbox
	}

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
			return starlarkstruct.FromStringDict(starlark.String("fail_result"), starlark.StringDict{
				"ok":      starlark.False,
				"message": starlark.String(msg),
			}), nil
		}),
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
		paramsDict.SetKey(starlark.String(k), sv)
	}

	// Call execute(resource, params, ctx)
	result, err := starlark.Call(thread, executeFn, starlark.Tuple{resource, paramsDict, ctxObj}, nil)
	if err != nil {
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
	// concurrency (CAS) — see db.UpdateParams.Version.
	SaveHandler func(workspaceID, module, entity, id string, version int, data map[string]any) error

	// CallHandler is the cross-resource call function.
	CallHandler func(workspaceID, fromModule, targetModule, targetEntity, action string, params map[string]any) (any, error)

	// LoadHandler loads another entity by ID, returning its data and version.
	LoadHandler func(workspaceID, module, entity, id string) (map[string]any, int, error)

	// CreateHandler creates a new record of another entity, returning its ID.
	CreateHandler func(workspaceID, module, entity string, data map[string]any) (string, error)

	// NextKeyHandler generates natural keys, scoped to the entity that owns
	// the field (natural key counters are per module/entity/field).
	NextKeyHandler func(workspaceID, module, entity, fieldName string) (string, error)
}

// NewScriptExecutor creates a ScriptExecutor with the given resolution function.
func NewScriptExecutor(resolver func(ref string) (string, error)) *ScriptExecutor {
	return &ScriptExecutor{
		ScriptPathResolver: resolver,
	}
}

// Execute runs a Starlark script for an action. resourceVersion is the
// current known version of the record (for CAS on resource.save()).
func (e *ScriptExecutor) Execute(scriptPath string, module, entity, id string, resourceData map[string]any, params map[string]any, workspaceID, userID string, resourceVersion int) (*ScriptResult, error) {
	// Build resource API
	res := NewResourceAPI(module, entity, id, resourceVersion, resourceData)
	if e.SaveHandler != nil {
		res.SetSaveFunc(func(m, ent, rid string, v int, data map[string]any) error {
			return e.SaveHandler(workspaceID, m, ent, rid, v, data)
		})
	}
	if e.CallHandler != nil {
		res.SetCallFunc(func(targetModule, targetEntity, actionName string, p map[string]any) (any, error) {
			// Infer target module from the calling context if not specified
			if targetModule == "" {
				targetModule = module
			}
			return e.CallHandler(workspaceID, module, targetModule, targetEntity, actionName, p)
		})
	}
	if e.LoadHandler != nil {
		res.SetLoadFunc(func(m, ent, eid string) (map[string]any, int, error) {
			return e.LoadHandler(workspaceID, m, ent, eid)
		})
	}
	if e.CreateHandler != nil {
		res.SetCreateFunc(func(m, ent string, data map[string]any) (string, error) {
			return e.CreateHandler(workspaceID, m, ent, data)
		})
	}

	// Build ctx
	ctxObj := NewCtxAPI(workspaceID, "", userID, "", nil)
	if e.NextKeyHandler != nil {
		ctxObj.NextKey = func(fieldName string) (string, error) {
			return e.NextKeyHandler(workspaceID, module, entity, fieldName)
		}
	}
	ctxObj.Now = now

	return ExecuteScript(scriptPath, res, params, ctxObj)
}
