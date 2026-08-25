// Package workflow provides the Workflow registry and approval engine — the
// runtime bridge between kind: Workflow manifests and state-machine
// transition interception (02-core-extended.md §2).
//
// A Workflow attaches role-based approval to a state-machine transition
// WITHOUT modifying the Entity. The intercepted transition only executes
// after every applicable step reaches its quorum; approval is a signed
// statement recorded in the audit trail.
package workflow

import (
	"sort"
	"sync"

	"github.com/primadi/formspec/pkg/spec"
)

// Registry maps {module}.{name} → WorkflowSpec for the runtime, and indexes
// workflows by the transition they intercept ({entity}.{from}.{to}).
type Registry struct {
	mu           sync.RWMutex
	workflows    map[string]*spec.WorkflowSpec // key = "module/name"
	byTransition map[string][]*spec.WorkflowSpec
}

// NewRegistry creates an empty Workflow registry.
func NewRegistry() *Registry {
	return &Registry{
		workflows:    make(map[string]*spec.WorkflowSpec),
		byTransition: make(map[string][]*spec.WorkflowSpec),
	}
}

// Add registers a Workflow manifest by module and name. Later registrations
// with the same key overwrite earlier ones (user override wins). The
// workflow is indexed under the transition it intercepts.
func (r *Registry) Add(module, name string, wf *spec.WorkflowSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := module + "/" + name

	// Remove any prior index entries for this key so a re-registration
	// (hot reload) doesn't leave stale transition mappings behind.
	if old, ok := r.workflows[key]; ok {
		if t := transitionKey(old); t != "" {
			r.byTransition[t] = removeWorkflow(r.byTransition[t], old)
		}
	}

	r.workflows[key] = wf
	if t := transitionKey(wf); t != "" {
		r.byTransition[t] = append(r.byTransition[t], wf)
	}
}

// transitionKey builds the index key "{entity}.{from}.{to}" for a workflow,
// or "" when the workflow has no transition trigger.
func transitionKey(wf *spec.WorkflowSpec) string {
	if wf == nil || wf.On == nil || wf.On.Transition == nil {
		return ""
	}
	return wf.Entity + "." + wf.On.Transition.From + "." + wf.On.Transition.To
}

// removeWorkflow returns list without the given workflow pointer.
func removeWorkflow(list []*spec.WorkflowSpec, target *spec.WorkflowSpec) []*spec.WorkflowSpec {
	out := list[:0]
	for _, w := range list {
		if w != target {
			out = append(out, w)
		}
	}
	return out
}

// Get returns the WorkflowSpec for {module}.{name}, or false if absent.
func (r *Registry) Get(module, name string) (*spec.WorkflowSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	wf, ok := r.workflows[module+"/"+name]
	return wf, ok
}

// ForTransition returns all workflows that intercept the given transition.
// entity is "module.entity" (e.g. "gl.journal-entry"); from/to are the state
// names. The returned slice is a copy; callers must not mutate it.
func (r *Registry) ForTransition(entity, from, to string) []*spec.WorkflowSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byTransition[entity+"."+from+"."+to]
	out := make([]*spec.WorkflowSpec, len(list))
	copy(out, list)
	return out
}

// WorkflowInfo is a lightweight summary of a registered Workflow.
type WorkflowInfo struct {
	Name   string `json:"name"`
	Module string `json:"module"`
	Entity string `json:"entity"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// List returns a sorted summary of all registered Workflows.
func (r *Registry) List() []WorkflowInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]WorkflowInfo, 0, len(r.workflows))
	for key, wf := range r.workflows {
		module, name := splitKey(key)
		info := WorkflowInfo{Module: module, Name: name, Entity: wf.Entity}
		if wf.On != nil && wf.On.Transition != nil {
			info.From = wf.On.Transition.From
			info.To = wf.On.Transition.To
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func splitKey(key string) (module, name string) {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return "", key
}
