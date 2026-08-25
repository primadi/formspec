package subscription

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// DynamicSubscription is a runtime-created subscription (todo 7.3.4): data,
// not manifest, living in formspec.core.subscription. Created/updated/deleted
// via the admin panel (UI surface), stored as an entity record, and merged
// into the subscription registry by the DynamicRefresher.
type DynamicSubscription struct {
	Name string
	Spec *spec.SubscriptionSpec
}

// RecordToSubscription converts a formspec.core.subscription entity record's
// data map into a DynamicSubscription. Returns ok=false for inactive or
// malformed records (no name, no events, no handler).
func RecordToSubscription(data map[string]any) (DynamicSubscription, bool) {
	name, _ := data["name"].(string)
	if name == "" {
		return DynamicSubscription{}, false
	}
	if active, ok := data["active"].(bool); ok && !active {
		return DynamicSubscription{}, false
	}
	events := toStringSlice(data["events"])
	if len(events) == 0 {
		return DynamicSubscription{}, false
	}
	handlerType := toString(data["handler_type"])
	handlerRef := toString(data["handler_ref"])
	if handlerType == "" && handlerRef == "" {
		return DynamicSubscription{}, false
	}

	sub := &spec.SubscriptionSpec{
		Events:    events,
		Handler:   spec.ImplDecl{Type: spec.ImplType(handlerType), Ref: handlerRef},
		Durable:   toString(data["durability"]),
		Store:     toString(data["store"]),
		Position:  toString(data["position"]),
		Filter:    toString(data["filter"]),
		Transform: toString(data["transform"]),
		MaxRetry:  toInt(data["max_retry"]),
		Retention: toString(data["retention"]),
	}
	return DynamicSubscription{Name: name, Spec: sub}, true
}

// MergeDynamic replaces all dynamic subscriptions (keyed "formspec.core/{name}")
// with the given set. Manifest subscriptions are untouched. Used at boot, on
// spec reload, and by the periodic DynamicRefresher (todo 7.3.4).
func (r *Registry) MergeDynamic(subs []DynamicSubscription) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove previous dynamic entries (keyed under the reserved formspec.core
	// namespace) so a refresh never leaves stale event mappings behind.
	for key, sub := range r.subscriptions {
		if strings.HasPrefix(key, CoreModule+"/") {
			for _, ev := range sub.Events {
				r.byEvent[ev] = removeSub(r.byEvent[ev], sub)
			}
			delete(r.subscriptions, key)
		}
	}

	for _, ds := range subs {
		key := CoreModule + "/" + ds.Name
		r.subscriptions[key] = ds.Spec
		for _, ev := range ds.Spec.Events {
			r.byEvent[ev] = append(r.byEvent[ev], ds.Spec)
		}
	}
}

// DynamicSource loads the current dynamic subscriptions for a workspace. Wired
// from resource/formspec.go to read the formspec.core.subscription entity
// store — kept as a callback so internal/subscription stays decoupled from the
// entity registry / DB.
type DynamicSource func(ctx context.Context, workspaceID string) ([]DynamicSubscription, error)

// DynamicRefresher periodically reloads dynamic subscriptions from a
// DynamicSource and merges them into the registry (todo 7.3.4), so admin-panel
// CRUD changes take effect without a spec reload or restart. Same lifecycle
// pattern as the outbox / escalation / streaming workers.
type DynamicRefresher struct {
	reg       *Registry
	source    DynamicSource
	workspace string
	interval  time.Duration

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	running bool
}

// DynamicRefresherOption configures the refresher.
type DynamicRefresherOption func(*DynamicRefresher)

// WithDynamicRefreshInterval sets the poll interval (default 5s).
func WithDynamicRefreshInterval(d time.Duration) DynamicRefresherOption {
	return func(w *DynamicRefresher) { w.interval = d }
}

// NewDynamicRefresher creates a dynamic-subscription refresher.
func NewDynamicRefresher(reg *Registry, source DynamicSource, workspace string, opts ...DynamicRefresherOption) *DynamicRefresher {
	w := &DynamicRefresher{
		reg:       reg,
		source:    source,
		workspace: workspace,
		interval:  5 * time.Second,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Start begins the background refresh loop.
func (w *DynamicRefresher) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.running = true
	w.wg.Add(1)
	go w.runLoop()
	log.Printf("[subscription-dynamic] started (poll=%v)", w.interval)
}

// Stop signals the refresher to shut down and waits for completion.
func (w *DynamicRefresher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	log.Printf("[subscription-dynamic] stopped")
}

// IsRunning reports whether the refresher is running.
func (w *DynamicRefresher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// Refresh loads dynamic subscriptions once and merges them into the registry.
// Exposed so boot/reload can refresh synchronously without waiting for the
// first poll.
func (w *DynamicRefresher) Refresh(ctx context.Context) error {
	if w.source == nil {
		return nil
	}
	subs, err := w.source(ctx, w.workspace)
	if err != nil {
		return err
	}
	w.reg.MergeDynamic(subs)
	return nil
}

// runLoop polls the dynamic source on the configured interval.
func (w *DynamicRefresher) runLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if err := w.Refresh(w.ctx); err != nil {
				log.Printf("[subscription-dynamic] refresh: %v", err)
			}
		}
	}
}

// ─── record value coercion helpers ───

// toString coerces a JSONB record value to a string.
func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return ""
	}
}

// toInt coerces a JSONB record value to an int.
func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

// toStringSlice coerces a JSONB array value to []string.
func toStringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}