// ─── Realtime Types ───
//
// Wire contract for the websocket push channel (spec Resolution API §5,
// `internal/events/hub.go`). The server is push-only: it broadcasts
// `EventMessage` to every connection registered under a workspace (per-message
// permission-filtered); the client filters by `resource` locally.

export interface RealtimeMessage {
  /** Declared entity event name, e.g. "completed". */
  event: string
  /** "module/entity", e.g. "clinic/visit". */
  resource: string
  /** Event fields (declared in the entity's events[].payload). */
  payload: Record<string, unknown>
  /** RFC3339Nano timestamp of emission. */
  emitted_at: string
}
