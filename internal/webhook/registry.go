// Package webhook provides the Webhook registry — the runtime bridge between
// kind: Webhook manifests and inbound endpoint handling (02-core-extended.md §4).
//
// A Webhook declares a verified inbound endpoint. spec.for references a single
// Service action that handles the payload; the framework verifies the request
// (signature or token auth) BEFORE the handler runs.
package webhook

import (
	"sort"
	"sync"

	"github.com/primadi/formspec/pkg/spec"
)

// Registry maps {module}.{name} → WebhookSpec for the runtime.
type Registry struct {
	mu       sync.RWMutex
	webhooks map[string]*spec.WebhookSpec // key = "module/name"
}

// NewRegistry creates an empty Webhook registry.
func NewRegistry() *Registry {
	return &Registry{webhooks: make(map[string]*spec.WebhookSpec)}
}

// Add registers a Webhook manifest by module and name. Later registrations
// with the same key overwrite earlier ones (user override wins).
func (r *Registry) Add(module, name string, wh *spec.WebhookSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webhooks[module+"/"+name] = wh
}

// Get returns the WebhookSpec for {module}.{name}, or false if absent.
func (r *Registry) Get(module, name string) (*spec.WebhookSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	wh, ok := r.webhooks[module+"/"+name]
	return wh, ok
}

// WebhookInfo is a lightweight summary of a registered Webhook.
type WebhookInfo struct {
	Name   string `json:"name"`
	Module string `json:"module"`
	Path   string `json:"path"`
	Method string `json:"method"`
	For    string `json:"for"`
}

// List returns a sorted summary of all registered Webhooks.
func (r *Registry) List() []WebhookInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]WebhookInfo, 0, len(r.webhooks))
	for key, wh := range r.webhooks {
		module, name := splitKey(key)
		out = append(out, WebhookInfo{
			Module: module, Name: name,
			Path: wh.Path, Method: wh.Method, For: wh.For,
		})
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
