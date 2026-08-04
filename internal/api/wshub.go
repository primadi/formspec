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
	"github.com/primadi/forma/internal/auth"
	"github.com/primadi/forma/internal/entity"
	"github.com/primadi/forma/internal/events"
)

type wsConn struct {
	id        string
	workspace string
	identity  *auth.Identity // nil if no auth validator is configured (see Broadcast)
	conn      *websocket.Conn
	send      chan events.EventMessage
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
		select {
		case c.send <- msg:
		default:
			// Slow consumer: drop rather than block the broadcaster.
			// Durable redelivery for a durable event is the outbox's job,
			// not the hub's.
		}
	}
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
// registers it in the hub, scoped to the caller's workspace. Push-only for
// v1 — no inbound application protocol is defined; the read loop exists
// solely to detect client disconnects and service ping/close control
// frames (required by the underlying library to keep the connection
// alive).
func (b *RouterBuilder) HandleWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// workspaceFromContext already carries AuthMiddleware's dev/prod fallback
		// (identity workspace > explicit workspace > "demo"), matching every
		// other handler in this package — no separate identity gate here.
		// The identity itself (nil if no auth validator is configured) is
		// captured for Broadcast's per-message permission filter (2.6.6).
		workspaceID := workspaceFromContext(r.Context())
		identity := IdentityFromContext(r.Context())

		c, err := websocket.Accept(w, r, nil)
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

func readPump(ctx context.Context, c *wsConn) {
	for {
		if _, _, err := c.conn.Read(ctx); err != nil {
			return
		}
	}
}
