// Cross-manifest Workflow validation (S9, kafe ledger 1.7).
//
// A kind: Workflow attaches approval to a state-machine transition. Whether the
// trigger actually resolves is only answerable with both manifests in view, so
// this check lives beside validateIntegrators in the CLI's cross-manifest layer
// rather than in the per-manifest loader.
//
// Two failure modes it exists to catch — both previously validated green:
//
//  1. A transition reference that matches nothing (typo, renamed action, wrong
//     entity). The workflow never intercepts anything: approval silently does
//     not happen.
//  2. A single-origin state pair describing a MULTI-origin transition. A
//     transition like `void-order` reachable from `[paid, in_kitchen, ready,
//     served]` is matched by `from: paid, to: cancelled` for one origin only, so
//     voiding a prepared order skipped approval entirely. The fix is to name the
//     transition, which covers every origin at once.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// transitionIndex maps "{module}.{entity}" to that entity's transitions.
type transitionIndex map[string]map[string]transitionInfo

// transitionInfo describes one state-machine transition by its `via` name.
type transitionInfo struct {
	// froms are the origin states; more than one means the transition is
	// reachable from several states.
	froms []string
	// to is the target state.
	to string
}

// validateWorkflows resolves every Workflow trigger against the entities' state
// machines. Returns a map of manifest source → error message.
func validateWorkflows(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}

	index := buildTransitionIndex(manifests)

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindWorkflow || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		wf, err := manifest.RawSpecToWorkflowSpec(specMap)
		if err != nil {
			rejects[m.Source] = "invalid workflow spec: " + err.Error()
			continue
		}
		if err := spec.ValidateWorkflowSpec(wf); err != nil {
			rejects[m.Source] = err.Error()
			continue
		}
		if wf.On == nil || wf.On.Transition == nil {
			continue
		}

		transitions, ok := index[wf.Entity]
		if !ok {
			rejects[m.Source] = fmt.Sprintf(
				"workflow targets %q, which has no state machine in this spec tree — the approval would never trigger", wf.Entity)
			continue
		}

		ref := wf.On.Transition
		if ref.ByName() {
			if msg := workflowNameError(wf.Entity, transitions, ref.Name); msg != "" {
				rejects[m.Source] = msg
			}
			continue
		}

		// State-pair form: verify the pair exists, then verify it is the WHOLE
		// transition rather than one origin of several.
		var matched *transitionInfo
		var matchedName string
		names := sortedNames(transitions)
		for _, name := range names {
			info := transitions[name]
			if info.to != ref.To || !containsString(info.froms, ref.From) {
				continue
			}
			copied := info
			matched = &copied
			matchedName = name
			break
		}
		if matched == nil {
			rejects[m.Source] = fmt.Sprintf(
				"workflow intercepts %s %s->%s, which is not a transition of %s — the approval would never trigger (known transitions: %s)",
				wf.Entity, ref.From, ref.To, wf.Entity, describeTransitions(transitions, names))
			continue
		}
		if len(matched.froms) > 1 {
			rejects[m.Source] = fmt.Sprintf(
				"workflow intercepts %s %s->%s, but transition %q is reachable from %s — this pair covers only %q, so approval would be skipped when the transition starts from any other state. Name the transition instead:\n         on:\n           transition: { name: %s }",
				wf.Entity, ref.From, ref.To, matchedName, strings.Join(matched.froms, ", "), ref.From, matchedName)
		}
	}

	return rejects
}

// workflowNameError validates the name form, returning "" when it resolves.
func workflowNameError(entity string, transitions map[string]transitionInfo, name string) string {
	// A fully-qualified reference is accepted only when it agrees with the
	// entity the workflow declared — otherwise the workflow would silently
	// attach to a different entity than it appears to.
	short := name
	if i := strings.LastIndex(name, "."); i >= 0 {
		qualified := name[:i]
		if qualified != entity {
			return fmt.Sprintf(
				"workflow on.transition.name %q names entity %q but spec.entity is %q — pick one form", name, qualified, entity)
		}
		short = name[i+1:]
	}

	if _, ok := transitions[short]; ok {
		return ""
	}
	return fmt.Sprintf(
		"workflow names transition %q, which does not exist on %s — the approval would never trigger (known transitions: %s)",
		short, entity, describeTransitions(transitions, sortedNames(transitions)))
}

// buildTransitionIndex collects every entity state machine's transitions,
// keyed by "{module}.{entity}" — the same qualifier Workflow.spec.entity uses.
//
// A transition is keyed by its `via` name. Transitions without a `via` are
// skipped: they cannot be referenced by name, and the state-pair form resolves
// them through the (from,to) scan above.
func buildTransitionIndex(manifests []manifest.RawManifest) transitionIndex {
	index := transitionIndex{}

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		entitySpec, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || entitySpec.StateMachine == nil {
			continue
		}

		key := m.Metadata.Module + "." + m.Metadata.Name
		transitions := map[string]transitionInfo{}
		for _, t := range entitySpec.StateMachine.Transitions {
			// The canonical key is `via`; the struct exposes it as Action.
			if t.Action == "" {
				continue
			}
			transitions[t.Action] = transitionInfo{froms: []string(t.From), to: t.To}
		}
		if len(transitions) > 0 {
			index[key] = transitions
		}
	}

	return index
}

// describeTransitions renders the known transitions for an error message.
func describeTransitions(transitions map[string]transitionInfo, names []string) string {
	if len(names) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s (%s->%s)", name, strings.Join(transitions[name].froms, "|"), transitions[name].to))
	}
	return strings.Join(parts, ", ")
}

// sortedNames returns the transition names in deterministic order.
func sortedNames(transitions map[string]transitionInfo) []string {
	names := make([]string, 0, len(transitions))
	for name := range transitions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// containsString reports whether list contains want.
func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
