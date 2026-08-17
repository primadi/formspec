// Package action — ScriptRef Executor
//
// This file implements the Executor interface for impl types script and script_ref.
// It delegates to the starlark.ScriptExecutor for actual Starlark execution.
package action

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	starlark "github.com/primadi/formspec/internal/starlark"
	"github.com/primadi/formspec/pkg/spec"
)

// ScriptExecutor implements action.Executor for script and script_ref impl types.
// It resolves script refs to .star file paths and executes them via starlark.
type ScriptExecutor struct {
	// basePath is the root of the spec directory (e.g. "./verticals/billing/spec").
	basePath string
	// engine is the underlying Starlark execution engine.
	engine *starlark.ScriptExecutor
}

// NewScriptExecutor creates a ScriptExecutor for the given spec base path.
func NewScriptExecutor(basePath string) *ScriptExecutor {
	engine := starlark.NewScriptExecutor(nil) // resolver set below
	engine.ScriptPathResolver = resolveScriptPath(basePath)
	return &ScriptExecutor{
		basePath: basePath,
		engine:   engine,
	}
}

// getSpecDir extracts the entity spec directory from ExecuteParams.
// It prefers SpecDir (set by the handler from the entity YAML path),
// falling back to basePath when SpecDir is empty (legacy callers).
func (e *ScriptExecutor) getSpecDir(params ExecuteParams) string {
	if params.SpecDir != "" {
		return params.SpecDir
	}
	return e.basePath
}

// SetSaveHandler sets the save callback used by resource.save() in scripts.
func (e *ScriptExecutor) SetSaveHandler(fn func(ctx context.Context, workspaceID, module, entity, id string, version int, data map[string]any) error) {
	e.engine.SaveHandler = fn
}

// SetCallHandler sets the cross-resource call callback. callerResources is
// the calling action's declared uses.resources (nil-safe) — the framework
// checks cross-module calls against it (todo 2.6.4).
func (e *ScriptExecutor) SetCallHandler(fn func(ctx context.Context, workspaceID, fromModule, targetModule, targetEntity, action string, params map[string]any, callerResources []string) (any, error)) {
	e.engine.CallHandler = fn
}

// SetLoadHandler sets the entity load callback. callerResources is the
// calling action's declared uses.resources (todo 2.6.4).
func (e *ScriptExecutor) SetLoadHandler(fn func(ctx context.Context, workspaceID, fromModule, module, entity, id string, callerResources []string) (map[string]any, int, error)) {
	e.engine.LoadHandler = fn
}

// SetCreateHandler sets the entity create callback used by resource.create()
// in scripts. callerResources is the calling action's declared
// uses.resources (todo 2.6.4).
func (e *ScriptExecutor) SetCreateHandler(fn func(ctx context.Context, workspaceID, fromModule, module, entity string, data map[string]any, callerResources []string) (string, error)) {
	e.engine.CreateHandler = fn
}

// SetNextKeyHandler sets the natural key generation callback.
func (e *ScriptExecutor) SetNextKeyHandler(fn func(ctx context.Context, workspaceID, module, entity, fieldName string) (string, error)) {
	e.engine.NextKeyHandler = fn
}

// Execute runs the script for the given action. ctx is threaded through to
// the engine (and from there to every resource.*/ctx.* handler) so a
// request-scoped TxScope, if one is active, is honored by every mutation
// the script performs — see internal/starlark's Execute doc comment.
func (e *ScriptExecutor) Execute(ctx context.Context, action spec.Action, params ExecuteParams) (*ExecuteResult, error) {
	if action.Impl == nil || action.Impl.Ref == "" {
		return nil, fmt.Errorf("script action %s has no impl.ref", action.Name)
	}

	// Resolve the script ref — prefer entity's spec directory, fall back to basePath
	scriptPath, err := resolveScript(e.basePath, e.getSpecDir(params), action.Impl.Ref)
	if err != nil {
		return nil, fmt.Errorf("resolve script ref %q: %w", action.Impl.Ref, err)
	}

	// Execute the script
	result, err := e.engine.Execute(
		ctx,
		scriptPath,
		params.Module,
		params.Entity,
		params.ResourceID,
		params.Resource,
		params.Params,
		params.WorkspaceID,
		params.UserID,
		params.ResourceVersion,
		declaredUsesResources(action),
	)
	if err != nil {
		return nil, fmt.Errorf("script execution error: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("script failed: %s", result.Error)
	}

	return &ExecuteResult{Data: result.Data}, nil
}

// declaredUsesResources returns the caller action's declared uses.resources
// as a []string, or nil when the action declares no uses block (todo 2.6.4).
// This is threaded through the script execution chain so cross-module
// resource access can be checked against it at runtime.
func declaredUsesResources(action spec.Action) []string {
	if action.Uses == nil {
		return nil
	}
	return action.Uses.Resources
}

// resolveScript resolves a script ref to an absolute file path.
//
// Resolution order (first match wins):
//
//  1. From entity's spec directory (specDir) — used when the entity YAML
//     file's directory is known. No folder structure is assumed; ref is just
//     a filename (e.g. "cancel") resolved relative to the entity's location:
//     a. {specDir}/scripts/{ref}.star
//     b. {specDir}/{ref}.star (direct, allows refs with relative paths)
//
//  2. Fallback from spec root (basePath) — used when specDir is empty
//     (hook scripts, legacy callers). Tries module-scoped patterns:
//     a. {basePath}/modules/{module}/scripts/{name}.star  (module-scoped)
//     b. {basePath}/modules/{ref}.star                     (flat ref-as-path)
//     c. {basePath}/{ref}.star                              (direct path)
//     d. {basePath}/scripts/{name}.star                     (top-level scripts)
func resolveScript(basePath, specDir, ref string) (string, error) {
	parts := parseRef(ref)
	tried := make([]string, 0, 6)

	// ── 1. Resolve relative to entity's spec directory ──
	if specDir != "" {
		candidate := filepath.Join(specDir, "scripts", ref+".star")
		tried = append(tried, candidate)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		candidate = filepath.Join(specDir, ref+".star")
		tried = append(tried, candidate)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// ── 2. Fallback: module-scoped (for backward compatibility) ──
	if len(parts) >= 2 {
		module := parts[0]
		scriptName := strings.Join(parts[1:], "/")
		candidate := filepath.Join(basePath, "modules", module, "scripts", scriptName+".star")
		tried = append(tried, candidate)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		candidate = filepath.Join(basePath, "modules", ref+".star")
		tried = append(tried, candidate)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// ── 3. Fallback: direct path from spec root ──
	candidate := filepath.Join(basePath, ref+".star")
	tried = append(tried, candidate)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	// ── 4. Fallback: flat scripts/ directory ──
	if len(parts) >= 1 {
		name := parts[len(parts)-1]
		candidate = filepath.Join(basePath, "scripts", name+".star")
		tried = append(tried, candidate)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("script not found for ref %q (tried: %s)", ref, strings.Join(tried, ", "))
}

// resolveScriptPath returns a function that resolves script refs to file paths.
// Kept for backward compatibility with starlark.ScriptPathResolver and
// non-entity script resolution (e.g. hook scripts without specDir context).
// Uses the same module-scoped patterns as the original resolver:
//   - "{basePath}/modules/{module}/scripts/{name}.star"  (module-scoped)
//   - "{basePath}/modules/{ref}.star"                     (flat ref-as-path)
//   - "{basePath}/scripts/{name}.star"                    (top-level scripts)
func resolveScriptPath(basePath string) func(ref string) (string, error) {
	return func(ref string) (string, error) {
		parts := parseRef(ref)
		tried := make([]string, 0, 4)

		if len(parts) >= 2 {
			module := parts[0]
			scriptName := strings.Join(parts[1:], "/")
			candidate := filepath.Join(basePath, "modules", module, "scripts", scriptName+".star")
			tried = append(tried, candidate)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}

		candidate := filepath.Join(basePath, "modules", ref+".star")
		tried = append(tried, candidate)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		if len(parts) >= 2 {
			// Nested module layout fallback (multi-segment ref)
			if len(parts) >= 3 {
				dir := strings.Join(parts[:len(parts)-1], "/")
				name := parts[len(parts)-1]
				candidate = filepath.Join(basePath, "modules", dir, "scripts", name+".star")
				tried = append(tried, candidate)
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}

			scriptName := parts[len(parts)-1]
			candidate = filepath.Join(basePath, "scripts", scriptName+".star")
			tried = append(tried, candidate)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}

		return "", fmt.Errorf("script not found for ref %q (tried: %s)", ref, strings.Join(tried, ", "))
	}
}

// parseRef splits a ref like "billing/order_checkout" into parts.
func parseRef(ref string) []string {
	parts := make([]string, 0)
	current := ""
	for _, c := range ref {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
