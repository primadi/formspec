// Package events holds the low-level, dependency-free types shared between
// event delivery (internal/action), the outbox worker (renderers/jsonb-persist), and
// the websocket transport (internal/api) — without creating an import cycle
// between them. internal/events depends on nothing else in this repo except
// pkg/spec (a leaf package already imported everywhere); renderers/jsonb-persist,
// internal/action, and internal/api all depend on internal/events.
package events

// EventMessage is the wire payload pushed to a websocket subscriber and
// written to the durable event log.
type EventMessage struct {
	Event    string         `json:"event"`    // e.g. "completed"
	Resource string         `json:"resource"` // "module/entity", e.g. "clinic/visit"
	Payload  map[string]any `json:"payload"`
	Emitted  string         `json:"emitted_at"` // RFC3339Nano
}

// Hub is the connection-manager contract a websocket transport implements
// and that delivery code depends on without importing internal/api.
type Hub interface {
	// Broadcast pushes msg to every connection registered under workspaceID.
	// Zero connections for workspaceID MUST be a cheap no-op, never an error.
	Broadcast(workspaceID string, msg EventMessage)

	// HasListeners reports whether at least one websocket connection is
	// registered for workspaceID. Callers use it to skip realtime publish
	// work (immediate push, outbox insurance) entirely when nobody is
	// listening — see action.NotifyMutation and action.DeliverEvents.
	HasListeners(workspaceID string) bool
}

// NoopHub is used where no hub is configured (e.g. tests that don't care
// about push delivery, or a build with websocket support disabled).
type NoopHub struct{}

// Broadcast is a no-op.
func (NoopHub) Broadcast(string, EventMessage) {}

// HasListeners reports no listeners — NoopHub never delivers.
func (NoopHub) HasListeners(string) bool { return false }

var _ Hub = NoopHub{}
