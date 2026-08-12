package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/events"
)

type wsConn struct {
	id        string
	workspace string
	identity  *auth.Identity // nil if no auth validator is configured (see Broadcast)
	conn      *websocket.Conn
	send      chan events.EventMessage

	// mu guards the subscription state below. all marks a "*" subscription
	// (every resource in the workspace); subs maps a subscribed resource
	// ("module/entity") to the set of event names the connection wants, with
	// the allEvents sentinel meaning every event on that resource. Delivery
	// is subscription-based (Spec Resolution API §5): a connection that never
	// subscribes receives nothing — filtered further by workspace and by the
	// per-message permission check in Broadcast (2.6.6).
	mu   sync.Mutex
	all  bool
	subs map[string]map[string]struct{}
}

// allEvents is the sentinel event name meaning "every event on this resource".
const allEvents = "*"

// wants reports whether the connection should receive an event. Delivery is
// subscription-based: no subscription → nothing; a "*" subscription → every
// resource in the workspace; otherwise only subscribed resources (and, when
// the subscription is event-scoped, matching event names).
func (c *wsConn) wants(resource, event string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.all {
		return true
	}
	evs, ok := c.subs[resource]
	if !ok {
		return false
	}
	if _, all := evs[allEvents]; all {
		return true
	}
	_, ok = evs[event]
	return ok
}

// subscribe registers interest in a resource. resource "*" subscribes to the
// whole workspace. An empty event subscribes to all events on the resource; a
// non-empty event narrows the subscription to that event.
func (c *wsConn) subscribe(resource, event string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if resource == "*" {
		c.all = true
		return
	}
	if c.subs == nil {
		c.subs = make(map[string]map[string]struct{})
	}
	evs, ok := c.subs[resource]
	if !ok {
		evs = make(map[string]struct{})
		c.subs[resource] = evs
	}
	if event == "" {
		evs[allEvents] = struct{}{}
	} else {
		evs[event] = struct{}{}
	}
}

// unsubscribe removes interest in a resource. resource "*" clears the
// whole-workspace subscription. An empty event drops the resource entirely;
// a non-empty event removes only that event, dropping the resource entry when
// nothing remains.
func (c *wsConn) unsubscribe(resource, event string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if resource == "*" {
		c.all = false
		return
	}
	evs, ok := c.subs[resource]
	if !ok {
		return
	}
	if event == "" {
		delete(c.subs, resource)
		return
	}
	delete(evs, event)
	if len(evs) == 0 {
		delete(c.subs, resource)
	}
}

// WSHub is a workspace-scoped websocket connection manager implementing
// events.Hub. Broadcast is a hot, read-heavy path (called once per
// delivered event, potentially many times/sec), so a mutex-protected map is
// used rather than an actor/channel design — each connection additionally
// has its own buffered send channel and dedicated writer goroutine, so a
// slow/blocked individual socket can't stall the hub or other connections.
// Only target: {scope: workspace} is supported (the only documented/used
// scope anywhere in the repo) — a later scope: user would be an additive
// second index, not a redesign.
type WSHub struct {
	mu          sync.RWMutex
	byWorkspace map[string]map[string]*wsConn
	registry    *entity.Registry // resolves EventMessage.Resource → plural for permission checks; nil disables filtering
}

// NewWSHub creates an empty hub. registry is used by Broadcast (2.6.6) to
// resolve an event's "module/entity" Resource to the {module}.{plural}.view
// permission a connection's identity is checked against; pass nil to skip
// per-message permission filtering entirely (e.g. in tests with no registry).
func NewWSHub(registry *entity.Registry) *WSHub {
	return &WSHub{byWorkspace: make(map[string]map[string]*wsConn), registry: registry}
}

func (h *WSHub) register(c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.byWorkspace[c.workspace]
	if !ok {
		conns = make(map[string]*wsConn)
		h.byWorkspace[c.workspace] = conns
	}
	conns[c.id] = c
}

func (h *WSHub) unregister(c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.byWorkspace[c.workspace]
	if !ok {
		return
	}
	delete(conns, c.id)
	if len(conns) == 0 {
		delete(h.byWorkspace, c.workspace)
	}
}

// Broadcast implements events.Hub. Zero connections for workspaceID is a
// cheap no-op — reading a nil map entry is safe in Go.
//
// Per-message permission filter (2.6.6): a connection only receives msg if
// its identity has {module}.{plural}.view for msg.Resource. A connection
// with no identity (no auth validator configured — matches the relaxed/dev
// fallback used everywhere else in this package) or a Resource/registry
// that can't be resolved to a permission is delivered to unfiltered, so
// this only engages once real auth is actually wired up.
func (h *WSHub) Broadcast(workspaceID string, msg events.EventMessage) {
	perm, havePerm := h.viewPermissionFor(msg.Resource)

	h.mu.RLock()
	conns := h.byWorkspace[workspaceID]
	targets := make([]*wsConn, 0, len(conns))
	for _, c := range conns {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		if havePerm && c.identity != nil && !c.identity.HasPermission(perm) {
			continue
		}
		// Subscription filter: only deliver events the connection asked for.
		if !c.wants(msg.Resource, msg.Event) {
			continue
		}
		select {
		case c.send <- msg:
		default:
			// Slow consumer: drop rather than block the broadcaster.
			// Durable redelivery for a durable event is the outbox's job,
			// not the hub's.
		}
	}
}

// HasListeners reports whether at least one connection is registered for the
// workspace. Cheap read-only check — callers gate realtime publish on it so an
// event with no live listener does zero work (see action.NotifyMutation and
// action.DeliverEvents).
func (h *WSHub) HasListeners(workspaceID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.byWorkspace[workspaceID]) > 0
}

// viewPermissionFor resolves an EventMessage.Resource ("module/entity") to
// the {module}.{plural}.view permission string used everywhere else
// (internal/entity/registry.go, internal/api/generator.go). Returns
// ok=false when the hub has no registry or the resource doesn't parse/
// resolve — callers must treat that as "can't filter", not "deny".
func (h *WSHub) viewPermissionFor(resource string) (perm string, ok bool) {
	if h.registry == nil {
		return "", false
	}
	module, name, found := strings.Cut(resource, "/")
	if !found {
		return "", false
	}
	info, found := h.registry.GetEntity(module, name)
	if !found {
		return "", false
	}
	plural := info.EntitySpec.Plural
	if plural == "" {
		plural = name + "s"
	}
	return module + "." + plural + ".view", true
}

var _ events.Hub = (*WSHub)(nil)

var wsConnSeq uint64

func nextConnID() string {
	return fmt.Sprintf("ws-%d", atomic.AddUint64(&wsConnSeq, 1))
}

// HandleWS upgrades an authenticated request to a websocket connection and
// registers it in the hub, scoped to the caller's workspace. The wire is
// push-only for event data; the only inbound application frames a client
// sends are subscribe/unsubscribe directives (Spec Resolution API §5) that
// tell the hub which resources (and optional events) this connection wants.
// The read loop both serves ping/close control frames and applies those
// subscription directives; Broadcast then only pushes matching events.
func (b *RouterBuilder) HandleWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// workspaceFromContext already carries AuthMiddleware's dev/prod fallback
		// (identity workspace > explicit workspace > "demo"), matching every
		// other handler in this package — no separate identity gate here.
		// The identity itself (nil if no auth validator is configured) is
		// captured for Broadcast's per-message permission filter (2.6.6).
		workspaceID := workspaceFromContext(r.Context())
		identity := IdentityFromContext(r.Context())

		// InsecureSkipVerify disables coder/websocket's Origin-vs-Host check.
		// It is required when the SPA is reached through a dev reverse proxy
		// (Vite --dev-ui: Origin stays the browser's :5173 while Host becomes
		// the backend's), and in general for any proxy where Origin != Host.
		// Authorization is still enforced per-connection by AuthMiddleware
		// (?token= for the handshake) and per-message by the workspace-scoped
		// permission filter in Broadcast (2.6.6) — this channel is push-only.
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()

		conn := &wsConn{id: nextConnID(), workspace: workspaceID, identity: identity, conn: c, send: make(chan events.EventMessage, 32)}
		b.hub.register(conn)
		defer b.hub.unregister(conn)

		ctx := r.Context()
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			writePump(ctx, conn, stop)
		}()

		readPump(ctx, conn) // blocks until the client disconnects or errors
		close(stop)
		wg.Wait()
	}
}

func writePump(ctx context.Context, c *wsConn, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
				return
			}
		}
	}
}

// clientFrame is an inbound subscription-control message. The connection is
// push-only for event data; the only application frames a client sends are
// subscribe/unsubscribe directives.
type clientFrame struct {
	Op       string `json:"op"`       // "subscribe" | "unsubscribe"
	Resource string `json:"resource"` // "module/entity" or "*"
	Event    string `json:"event"`    // optional — empty = all events on the resource
}

// readPump reads until the client disconnects or errors. Inbound text frames
// are parsed as subscription-control messages and applied to the connection;
// control frames (ping/pong/close) are served internally by the library.
// Malformed or unknown frames are ignored — never fatal.
func readPump(ctx context.Context, c *wsConn) {
	for {
		typ, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			continue
		}
		var f clientFrame
		if json.Unmarshal(data, &f) != nil {
			continue
		}
		switch f.Op {
		case "subscribe":
			if f.Resource != "" {
				c.subscribe(f.Resource, f.Event)
			}
		case "unsubscribe":
			if f.Resource != "" {
				c.unsubscribe(f.Resource, f.Event)
			}
		default:
			// unknown op — ignore
		}
	}
}
