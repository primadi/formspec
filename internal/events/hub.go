// Package events holds the low-level, dependency-free types shared between
// event delivery (internal/action), the outbox worker (internal/db), and
// the websocket transport (internal/api) — without creating an import cycle
// between them. internal/events depends on nothing else in this repo except
// pkg/spec (a leaf package already imported everywhere); internal/db,
// internal/action, and internal/api all depend on internal/events.
package events

// EventMessage is the wire payload pushed to a websocket subscriber and
// written to the durable event log.
type EventMessage struct {
	Event    string         `json:"event"`      // e.g. "completed"
	Resource string         `json:"resource"`   // "module/entity", e.g. "clinic/visit"
	Payload  map[string]any `json:"payload"`
	Emitted  string         `json:"emitted_at"` // RFC3339Nano
}

// Hub is the connection-manager contract a websocket transport implements
// and that delivery code depends on without importing internal/api.
type Hub interface {
	// Broadcast pushes msg to every connection registered under tenantID.
	// Zero connections for tenantID MUST be a cheap no-op, never an error.
	Broadcast(tenantID string, msg EventMessage)
}

// NoopHub is used where no hub is configured (e.g. tests that don't care
// about push delivery, or a build with websocket support disabled).
type NoopHub struct{}

// Broadcast is a no-op.
func (NoopHub) Broadcast(string, EventMessage) {}

var _ Hub = NoopHub{}
