// Package service provides the Service registry — the runtime bridge between
// kind: Service manifests and action dispatch (01-core-basic.md §1.1).
//
// A Service is a stateless, pure computation resource: no persisted state, no
// characteristic/doc_status/lifecycle guard. It exposes a set of actions that
// are dispatched through the same action dispatcher as entity custom actions,
// so impl types (native/script/script_ref/compiled/sidecar) and permission
// enforcement behave uniformly.
package service

import (
	"sort"
	"sync"

	"github.com/primadi/formspec/pkg/spec"
)

// Registry maps {module}.{name} → ServiceSpec for the runtime.
type Registry struct {
	mu       sync.RWMutex
	services map[string]*spec.ServiceSpec // key = "module/name"
}

// NewRegistry creates an empty Service registry.
func NewRegistry() *Registry {
	return &Registry{services: make(map[string]*spec.ServiceSpec)}
}

// Add registers a Service manifest by module and name. Later registrations
// with the same key overwrite earlier ones (user override wins).
func (r *Registry) Add(module, name string, svc *spec.ServiceSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[module+"/"+name] = svc
}

// Get returns the ServiceSpec for {module}.{name}, or false if absent.
func (r *Registry) Get(module, name string) (*spec.ServiceSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	svc, ok := r.services[module+"/"+name]
	return svc, ok
}

// GetAction returns the named action on a Service, or false if the service or
// action is absent.
func (r *Registry) GetAction(module, name, actionName string) (*spec.Action, bool) {
	svc, ok := r.Get(module, name)
	if !ok {
		return nil, false
	}
	for i := range svc.Actions {
		if svc.Actions[i].Name == actionName {
			return &svc.Actions[i], true
		}
	}
	return nil, false
}

// ServiceInfo is a lightweight summary of a registered Service.
type ServiceInfo struct {
	Name    string   `json:"name"`
	Module  string   `json:"module"`
	Actions []string `json:"actions"`
}

// List returns a sorted summary of all registered Services.
func (r *Registry) List() []ServiceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ServiceInfo, 0, len(r.services))
	for key, svc := range r.services {
		module, name := splitKey(key)
		actions := make([]string, 0, len(svc.Actions))
		for _, a := range svc.Actions {
			actions = append(actions, a.Name)
		}
		out = append(out, ServiceInfo{Module: module, Name: name, Actions: actions})
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
