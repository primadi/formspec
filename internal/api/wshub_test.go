package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/primadi/forma/internal/entity"
	"github.com/primadi/forma/internal/events"
	db "github.com/primadi/forma/renderers/jsonbpersist"
)

func newTestRouterServer(t *testing.T) (*httptest.Server, *RouterBuilder) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.OpenSQLite(filepath.Join(dir, "wshub_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	rb := NewRouterBuilder(reg)
	rb.BuildRoutes()
	srv := httptest.NewServer(rb.BuildHTTP())
	t.Cleanup(srv.Close)
	return srv, rb
}

func dialWS(t *testing.T, srv *httptest.Server, workspace string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/" + workspace + "/_ui/_ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func readOneMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) (events.EventMessage, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		return events.EventMessage{}, false
	}
	var msg events.EventMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return msg, true
}

// TestHandleWS_BroadcastEndToEnd dials a real websocket client against the
// actual router (same code path production uses: HandleWS → register →
// RouterBuilder.Hub().Broadcast → writePump → the wire), proving a message
// broadcast through the hub reaches a connected client.
func TestHandleWS_BroadcastEndToEnd(t *testing.T) {
	srv, rb := newTestRouterServer(t)
	conn := dialWS(t, srv, "acme")

	// Give the server a moment to register the connection before broadcasting.
	time.Sleep(50 * time.Millisecond)

	// No auth validator is configured in this test, so AuthMiddleware's
	// dev fallback applies and every connection is registered under workspace
	// "demo" regardless of the URL workspace segment (see workspaceFromContext).
	rb.Hub().Broadcast("demo", events.EventMessage{Event: "completed", Resource: "clinic/visit"})

	msg, ok := readOneMessage(t, conn, 2*time.Second)
	if !ok {
		t.Fatal("client did not receive the broadcast")
	}
	if msg.Event != "completed" || msg.Resource != "clinic/visit" {
		t.Errorf("got %+v, want Event=completed Resource=clinic/visit", msg)
	}
}

func TestWSHub_Broadcast_WorkspaceIsolation(t *testing.T) {
	hub := NewWSHub()

	connA := &wsConn{id: "a", workspace: "tenant-a", send: make(chan events.EventMessage, 4)}
	connB := &wsConn{id: "b", workspace: "tenant-b", send: make(chan events.EventMessage, 4)}
	hub.register(connA)
	hub.register(connB)

	hub.Broadcast("tenant-a", events.EventMessage{Event: "completed"})

	select {
	case <-connA.send:
		// expected
	case <-time.After(200 * time.Millisecond):
		t.Fatal("tenant-a connection did not receive its own tenant's broadcast")
	}
	select {
	case msg := <-connB.send:
		t.Fatalf("tenant-b connection unexpectedly received tenant-a's broadcast: %+v", msg)
	case <-time.After(100 * time.Millisecond):
		// expected: no message for the other workspace
	}
}

func TestWSHub_Broadcast_ZeroConnectionsIsNoop(t *testing.T) {
	hub := NewWSHub()
	// Must not panic or block when no connection is registered for the workspace.
	hub.Broadcast("no-such-workspace", events.EventMessage{Event: "completed"})
}

func TestWSHub_RegisterUnregister(t *testing.T) {
	hub := NewWSHub()
	c := &wsConn{id: "x", workspace: "t1", send: make(chan events.EventMessage, 1)}
	hub.register(c)
	if len(hub.byWorkspace["t1"]) != 1 {
		t.Fatalf("expected 1 registered connection, got %d", len(hub.byWorkspace["t1"]))
	}
	hub.unregister(c)
	if _, ok := hub.byWorkspace["t1"]; ok {
		t.Errorf("expected workspace map to be cleaned up after last connection unregisters")
	}
}
