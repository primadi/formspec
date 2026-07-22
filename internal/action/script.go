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

	starlark "github.com/primadi/forma/internal/starlark"
	"github.com/primadi/forma/pkg/spec"
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

// SetSaveHandler sets the save callback used by resource.save() in scripts.
func (e *ScriptExecutor) SetSaveHandler(fn func(ctx context.Context, workspaceID, module, entity, id string, version int, data map[string]any) error) {
	e.engine.SaveHandler = fn
}

// SetCallHandler sets the cross-resource call callback.
func (e *ScriptExecutor) SetCallHandler(fn func(ctx context.Context, workspaceID, fromModule, targetModule, targetEntity, action string, params map[string]any) (any, error)) {
	e.engine.CallHandler = fn
}

// SetLoadHandler sets the entity load callback.
func (e *ScriptExecutor) SetLoadHandler(fn func(ctx context.Context, workspaceID, module, entity, id string) (map[string]any, int, error)) {
	e.engine.LoadHandler = fn
}

// SetCreateHandler sets the entity create callback used by resource.create() in scripts.
func (e *ScriptExecutor) SetCreateHandler(fn func(ctx context.Context, workspaceID, module, entity string, data map[string]any) (string, error)) {
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

	// Resolve the script ref to a file path
	scriptPath, err := resolveScriptPath(e.basePath)(action.Impl.Ref)
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
	)
	if err != nil {
		return nil, fmt.Errorf("script execution error: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("script failed: %s", result.Error)
	}

	return &ExecuteResult{Data: result.Data}, nil
}

// resolveScriptPath returns a function that resolves script refs to file paths.
//
// Resolution rules (first match wins):
//   - "module/script_name" → "{basePath}/modules/{module}/scripts/{script_name}.star"
//     (the layout used by every module-scoped example: Clinic, Inventory,
//     General-Ledger, Order-to-Cash — a per-module "scripts/" subdirectory)
//   - "module/script_name" → "{basePath}/modules/{ref}.star"
//     (flat ref-as-path, kept for backward compatibility)
//   - "{basePath}/scripts/{script_name}.star" (top-level scripts, used by
//     examples with a flat spec layout, e.g. Customer, Midtrans-Payment-Gateway)
func resolveScriptPath(basePath string) func(ref string) (string, error) {
	return func(ref string) (string, error) {
		parts := parseRef(ref)
		tried := make([]string, 0, 3)

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
