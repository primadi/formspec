package action

import (
	"context"
	"sort"

	"github.com/primadi/formspec/pkg/spec"
)

const defaultHookPriority = 10

// SelectHooks returns hooks matching timing and actionName ("*" matches any
// action), sorted ascending by effective priority (0 → default 10, per Core
// Extended §8's tier table); ties keep manifest declaration order (stable
// sort).
func SelectHooks(hooks []spec.HookDecl, timing spec.HookTiming, actionName string) []spec.HookDecl {
	var matched []spec.HookDecl
	for _, h := range hooks {
		if h.On != timing {
			continue
		}
		if h.Action != actionName && h.Action != "*" {
			continue
		}
		matched = append(matched, h)
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return effectivePriority(matched[i]) < effectivePriority(matched[j])
	})
	return matched
}

func effectivePriority(h spec.HookDecl) int {
	if h.Priority == 0 {
		return defaultHookPriority
	}
	return h.Priority
}

// SelectEventHooks returns hooks matching timing and eventName ("*" matches
// any event), sorted ascending by effective priority — the event-side
// counterpart of SelectHooks for before_deliver/after_deliver (todo 7.8.5).
func SelectEventHooks(hooks []spec.HookDecl, timing spec.HookTiming, eventName string) []spec.HookDecl {
	var matched []spec.HookDecl
	for _, h := range hooks {
		if h.On != timing {
			continue
		}
		if h.Event != eventName && h.Event != "*" {
			continue
		}
		matched = append(matched, h)
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return effectivePriority(matched[i]) < effectivePriority(matched[j])
	})
	return matched
}

// RunBeforePhase runs, in priority order, every hook matching (on: before,
// action: actionName|"*"), then — if actionSpec has its own Impl (a reserved
// action's own script override, e.g. create/update) — runs that impl last,
// as the final step of the before-phase. Each step goes through the same
// Dispatcher.Dispatch used for custom actions, so any impl type works
// uniformly (script_ref, native, ...). params.Resource is a map (reference
// type), so resource.set() mutations from any step are already visible to
// the caller without extra plumbing. The first failure aborts immediately —
// no later before-steps run, and whatever base guard called this (e.g.
// store.Insert/Update) must not run either.
func RunBeforePhase(ctx context.Context, disp *Dispatcher, hooks []spec.HookDecl, actionSpec *spec.Action, actionName string, params *ExecuteParams) error {
	for _, h := range SelectHooks(hooks, spec.HookOnBefore, actionName) {
		// A hook runs as part of the enclosing action, so it inherits the
		// action's uses declaration — cross-module resource access from a
		// hook is gated by the action's uses.resources (todo 2.6.4).
		hookAction := spec.Action{Name: actionName, Impl: h.Impl}
		if actionSpec != nil {
			hookAction.Uses = actionSpec.Uses
		}
		if _, err := disp.Dispatch(ctx, hookAction, *params); err != nil {
			runOnErrorPhase(ctx, disp, hooks, actionName, params, err)
			return err
		}
	}
	if actionSpec != nil && actionSpec.Impl != nil {
		if _, err := disp.Dispatch(ctx, *actionSpec, *params); err != nil {
			runOnErrorPhase(ctx, disp, hooks, actionName, params, err)
			return err
		}
	}
	return nil
}

// RunAfterPhase runs (on: after, action: actionName|"*") hooks synchronously,
// after the base guard has already committed. Errors drive on_error hooks
// and are logged, but never fail the HTTP response — the record already
// exists; there is nothing left to roll back (mirrors Core Basic §12's
// reasoning that on_* events are "logically impossible to be sync and
// cancel something already committed").
func RunAfterPhase(ctx context.Context, disp *Dispatcher, hooks []spec.HookDecl, actionSpec *spec.Action, actionName string, persisted ExecuteParams) {
	for _, h := range SelectHooks(hooks, spec.HookOnAfter, actionName) {
		hookAction := spec.Action{Name: actionName, Impl: h.Impl}
		if actionSpec != nil {
			hookAction.Uses = actionSpec.Uses
		}
		if _, err := disp.Dispatch(ctx, hookAction, persisted); err != nil {
			logger(persisted).Error("hook.after.failed", map[string]any{"action": actionName, "error": err.Error()})
			runOnErrorPhase(ctx, disp, hooks, actionName, &persisted, err)
		}
	}
}

// runOnErrorPhase runs (on: on_error, action: actionName|"*") hooks, passing
// the failing error via the well-known params key "_hook_error" so the
// script can branch on it. Its own errors are logged only, best-effort — an
// on_error handler failing must never mask the original failure.
func runOnErrorPhase(ctx context.Context, disp *Dispatcher, hooks []spec.HookDecl, actionName string, params *ExecuteParams, cause error) {
	matches := SelectHooks(hooks, spec.HookOnError, actionName)
	if len(matches) == 0 {
		return
	}
	errParams := *params
	errParams.Params = withHookError(params.Params, cause)
	for _, h := range matches {
		if _, err := disp.Dispatch(ctx, spec.Action{Name: actionName, Impl: h.Impl}, errParams); err != nil {
			logger(errParams).Error("hook.on_error.failed", map[string]any{"action": actionName, "error": err.Error()})
		}
	}
}

func withHookError(params map[string]any, cause error) map[string]any {
	merged := make(map[string]any, len(params)+1)
	for k, v := range params {
		merged[k] = v
	}
	merged["_hook_error"] = cause.Error()
	return merged
}

func logger(params ExecuteParams) RuntimeLogger {
	if params.RuntimeCtx != nil && params.RuntimeCtx.Logger != nil {
		return params.RuntimeCtx.Logger
	}
	return DefaultLogger
}
