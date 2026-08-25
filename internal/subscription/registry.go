// Package subscription provides the Subscription registry — the runtime
// bridge between kind: Subscription manifests and event dispatch
// (02-core-extended.md §3, 01-core-basic.md §7).
//
// A Subscription makes one module react to another resource's events without
// modifying the publisher. Tier 1 (Core, outbox) matches an emitted event
// against subscriptions and calls each matching subscription's handler — a
// Service action referenced by `handler.ref` ({module}.{service}).
package subscription

import (
	"sort"
	"sync"

	"github.com/primadi/formspec/pkg/spec"
)

// Registry maps event name → subscriptions that subscribe to it.
//
// Event names are fully-qualified resource events, e.g. "billing.invoice.on_submit"
// (the SubscriptionSpec.Events entries). The registry indexes by that exact
// string so a publisher emission can look up matching subscriptions directly.
type Registry struct {
	mu            sync.RWMutex
	byEvent       map[string][]*spec.SubscriptionSpec
	subscriptions map[string]*spec.SubscriptionSpec // key = "module/name"
}

// NewRegistry creates an empty Subscription registry.
func NewRegistry() *Registry {
	return &Registry{
		byEvent:       make(map[string][]*spec.SubscriptionSpec),
		subscriptions: make(map[string]*spec.SubscriptionSpec),
	}
}

// Add registers a Subscription manifest by module and name. Later
// registrations with the same key overwrite earlier ones (user override
// wins). The subscription is indexed under each of its declared events.
func (r *Registry) Add(module, name string, sub *spec.SubscriptionSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := module + "/" + name

	// Remove any prior index entries for this key so a re-registration
	// (hot reload) doesn't leave stale event mappings behind.
	if old, ok := r.subscriptions[key]; ok {
		for _, ev := range old.Events {
			r.byEvent[ev] = removeSub(r.byEvent[ev], old)
		}
	}

	r.subscriptions[key] = sub
	for _, ev := range sub.Events {
		r.byEvent[ev] = append(r.byEvent[ev], sub)
	}
}

// removeSub returns list without the given subscription pointer.
func removeSub(list []*spec.SubscriptionSpec, target *spec.SubscriptionSpec) []*spec.SubscriptionSpec {
	out := list[:0]
	for _, s := range list {
		if s != target {
			out = append(out, s)
		}
	}
	return out
}

// Get returns the SubscriptionSpec for {module}.{name}, or false if absent.
func (r *Registry) Get(module, name string) (*spec.SubscriptionSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sub, ok := r.subscriptions[module+"/"+name]
	return sub, ok
}

// ForEvent returns all subscriptions that subscribe to the given event name.
// The returned slice is a copy; callers must not mutate it.
func (r *Registry) ForEvent(eventName string) []*spec.SubscriptionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byEvent[eventName]
	out := make([]*spec.SubscriptionSpec, len(list))
	copy(out, list)
	return out
}

// DurableSub is a subscription with durability: durable (Tier 2 streaming),
// paired with its module/name key so the StreamingWorker can name its
// consumer group.
type DurableSub struct {
	Module string
	Name   string
	Spec   *spec.SubscriptionSpec
}

// Durable returns all subscriptions with durability: durable (Tier 2
// streaming, todo 7.3.2). Used by the StreamingWorker to know which
// subscriptions to consume from streams.
func (r *Registry) Durable() []DurableSub {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []DurableSub
	for key, sub := range r.subscriptions {
		if sub.Durable == "durable" {
			module, name := splitKey(key)
			out = append(out, DurableSub{Module: module, Name: name, Spec: sub})
		}
	}
	return out
}

// SubscriptionInfo is a lightweight summary of a registered Subscription.
type SubscriptionInfo struct {
	Name   string   `json:"name"`
	Module string   `json:"module"`
	Events []string `json:"events"`
	For    string   `json:"for"`
}

// List returns a sorted summary of all registered Subscriptions.
func (r *Registry) List() []SubscriptionInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]SubscriptionInfo, 0, len(r.subscriptions))
	for key, sub := range r.subscriptions {
		module, name := splitKey(key)
		out = append(out, SubscriptionInfo{
			Module: module, Name: name,
			Events: sub.Events,
			For:    sub.Handler.Ref,
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
