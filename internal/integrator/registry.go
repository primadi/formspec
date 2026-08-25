// Package integrator provides the Integrator registry and dispatch bridge —
// the runtime bridge between kind: Integrator manifests and cross-module
// event → action bridging (02-core-extended.md §5).
//
// An Integrator bridges two entities/modules that do not know each other
// directly: `listen.resource` + `listen.event` triggers `call.resource` +
// `call.action`. The listen/call resources are resolved through the registry —
// the Integrator never imports the other module's definitions directly.
package integrator

import (
	"sort"
	"sync"

	"github.com/primadi/formspec/pkg/spec"
)

// Registry maps {module}.{name} → IntegratorSpec for the runtime, and indexes
// integrators by the event they listen to ({resource}.{event}).
type Registry struct {
	mu           sync.RWMutex
	integrators  map[string]*spec.IntegratorSpec // key = "module/name"
	byEvent      map[string][]*spec.IntegratorSpec
}

// NewRegistry creates an empty Integrator registry.
func NewRegistry() *Registry {
	return &Registry{
		integrators: make(map[string]*spec.IntegratorSpec),
		byEvent:     make(map[string][]*spec.IntegratorSpec),
	}
}

// Add registers an Integrator manifest by module and name. Later registrations
// with the same key overwrite earlier ones (user override wins). The
// integrator is indexed under the event it listens to.
func (r *Registry) Add(module, name string, it *spec.IntegratorSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := module + "/" + name

	// Remove any prior index entries for this key so a re-registration
	// (hot reload) doesn't leave stale event mappings behind.
	if old, ok := r.integrators[key]; ok {
		if ev := eventKey(old); ev != "" {
			r.byEvent[ev] = removeIntegrator(r.byEvent[ev], old)
		}
	}

	r.integrators[key] = it
	if ev := eventKey(it); ev != "" {
		r.byEvent[ev] = append(r.byEvent[ev], it)
	}
}

// eventKey builds the index key "{resource}.{event}" for an integrator, or ""
// when the integrator has no listen declaration.
func eventKey(it *spec.IntegratorSpec) string {
	if it == nil || it.Listen == nil {
		return ""
	}
	return it.Listen.Resource + "." + it.Listen.Event
}

// removeIntegrator returns list without the given integrator pointer.
func removeIntegrator(list []*spec.IntegratorSpec, target *spec.IntegratorSpec) []*spec.IntegratorSpec {
	out := list[:0]
	for _, it := range list {
		if it != target {
			out = append(out, it)
		}
	}
	return out
}

// Get returns the IntegratorSpec for {module}.{name}, or false if absent.
func (r *Registry) Get(module, name string) (*spec.IntegratorSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	it, ok := r.integrators[module+"/"+name]
	return it, ok
}

// ForEvent returns all integrators that listen to the given event. eventName
// is the fully-qualified resource event (e.g. "billing.invoice.on_submit").
// The returned slice is a copy; callers must not mutate it.
func (r *Registry) ForEvent(eventName string) []*spec.IntegratorSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byEvent[eventName]
	out := make([]*spec.IntegratorSpec, len(list))
	copy(out, list)
	return out
}

// IntegratorInfo is a lightweight summary of a registered Integrator.
type IntegratorInfo struct {
	Name     string `json:"name"`
	Module   string `json:"module"`
	Listen   string `json:"listen"`
	Call     string `json:"call"`
	Compensate string `json:"compensate,omitempty"`
}

// List returns a sorted summary of all registered Integrators.
func (r *Registry) List() []IntegratorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]IntegratorInfo, 0, len(r.integrators))
	for key, it := range r.integrators {
		module, name := splitKey(key)
		info := IntegratorInfo{Module: module, Name: name}
		if it.Listen != nil {
			info.Listen = it.Listen.Resource + "." + it.Listen.Event
		}
		if it.Call != nil {
			info.Call = it.Call.Resource + "." + it.Call.Action
		}
		info.Compensate = it.Compensate
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