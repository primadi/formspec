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
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/events"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
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

	// Subscribe to everything (via the wire) — delivery is subscription-based.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"op":"subscribe","resource":"*"}`)); err != nil {
		t.Fatalf("write subscribe frame: %v", err)
	}
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
	hub := NewWSHub(nil)

	connA := &wsConn{id: "a", workspace: "tenant-a", send: make(chan events.EventMessage, 4)}
	connB := &wsConn{id: "b", workspace: "tenant-b", send: make(chan events.EventMessage, 4)}
	connA.subscribe("*", "")
	connB.subscribe("*", "")
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
	hub := NewWSHub(nil)
	// Must not panic or block when no connection is registered for the workspace.
	hub.Broadcast("no-such-workspace", events.EventMessage{Event: "completed"})
}

func TestWSHub_RegisterUnregister(t *testing.T) {
	hub := NewWSHub(nil)
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

func TestWSHub_HasListeners(t *testing.T) {
	hub := NewWSHub(nil)
	if hub.HasListeners("t1") {
		t.Errorf("HasListeners should be false with no registered connection")
	}

	c := &wsConn{id: "x", workspace: "t1", send: make(chan events.EventMessage, 1)}
	hub.register(c)
	if !hub.HasListeners("t1") {
		t.Errorf("HasListeners should be true after registering a connection")
	}
	if hub.HasListeners("t2") {
		t.Errorf("HasListeners for a different workspace should be false")
	}

	hub.unregister(c)
	if hub.HasListeners("t1") {
		t.Errorf("HasListeners should be false after the last connection unregisters")
	}
}

// assertReceived drains c.send and reports whether a message arrived within
// the timeout, without consuming anything else (used by the subscription
// tests below).
func assertReceived(t *testing.T, c *wsConn, want bool, label string) {
	t.Helper()
	select {
	case <-c.send:
		if !want {
			t.Fatalf("%s: connection unexpectedly received a broadcast", label)
		}
	case <-time.After(150 * time.Millisecond):
		if want {
			t.Fatalf("%s: connection did not receive the broadcast", label)
		}
	}
}

func TestWSHub_NoSubscription_ReceivesNothing(t *testing.T) {
	hub := NewWSHub(nil)
	c := &wsConn{id: "a", workspace: "t1", send: make(chan events.EventMessage, 4)}
	hub.register(c)

	// Delivery is subscription-based: never subscribed → no event.
	hub.Broadcast("t1", events.EventMessage{Event: "created", Resource: "clinic/visit"})
	assertReceived(t, c, false, "no-subscription")
}

func TestWSHub_SubscribeFiltersResource(t *testing.T) {
	hub := NewWSHub(nil)
	c := &wsConn{id: "a", workspace: "t1", send: make(chan events.EventMessage, 4)}
	hub.register(c)
	c.subscribe("clinic/visit", "")

	hub.Broadcast("t1", events.EventMessage{Event: "created", Resource: "clinic/visit"})
	assertReceived(t, c, true, "subscribed resource")

	hub.Broadcast("t1", events.EventMessage{Event: "created", Resource: "pharmacy/prescription"})
	assertReceived(t, c, false, "unsubscribed resource")
}

func TestWSHub_SubscribeStar_ReceivesAll(t *testing.T) {
	hub := NewWSHub(nil)
	c := &wsConn{id: "a", workspace: "t1", send: make(chan events.EventMessage, 4)}
	hub.register(c)
	c.subscribe("*", "")

	hub.Broadcast("t1", events.EventMessage{Event: "created", Resource: "clinic/visit"})
	assertReceived(t, c, true, "star: clinic/visit")
	hub.Broadcast("t1", events.EventMessage{Event: "created", Resource: "pharmacy/prescription"})
	assertReceived(t, c, true, "star: pharmacy/prescription")
}

func TestWSHub_SubscribeEventFilter(t *testing.T) {
	hub := NewWSHub(nil)
	c := &wsConn{id: "a", workspace: "t1", send: make(chan events.EventMessage, 4)}
	hub.register(c)
	c.subscribe("clinic/visit", "created")

	hub.Broadcast("t1", events.EventMessage{Event: "created", Resource: "clinic/visit"})
	assertReceived(t, c, true, "subscribed event")

	hub.Broadcast("t1", events.EventMessage{Event: "updated", Resource: "clinic/visit"})
	assertReceived(t, c, false, "unsubscribed event on subscribed resource")
}

func TestWSHub_Unsubscribe(t *testing.T) {
	hub := NewWSHub(nil)
	c := &wsConn{id: "a", workspace: "t1", send: make(chan events.EventMessage, 4)}
	hub.register(c)
	c.subscribe("clinic/visit", "")
	c.unsubscribe("clinic/visit", "")

	hub.Broadcast("t1", events.EventMessage{Event: "created", Resource: "clinic/visit"})
	assertReceived(t, c, false, "after unsubscribe")
}

func TestWSHub_UnsubscribeSpecificEventKeepsResource(t *testing.T) {
	hub := NewWSHub(nil)
	c := &wsConn{id: "a", workspace: "t1", send: make(chan events.EventMessage, 4)}
	hub.register(c)
	c.subscribe("clinic/visit", "created")
	c.subscribe("clinic/visit", "updated")
	c.unsubscribe("clinic/visit", "created")

	hub.Broadcast("t1", events.EventMessage{Event: "created", Resource: "clinic/visit"})
	assertReceived(t, c, false, "removed event")
	hub.Broadcast("t1", events.EventMessage{Event: "updated", Resource: "clinic/visit"})
	assertReceived(t, c, true, "kept event")
}

// TestHandleWS_SubscribeFrameFiltersEndToEnd dials a real websocket client and
// drives the subscription protocol over the wire: after subscribing to
// clinic/visit, only that resource's events are pushed. Note: coder/websocket
// closes the connection when a read's context expires, so the "expect no
// message" read must be the test's final operation.
func TestHandleWS_SubscribeFrameFiltersEndToEnd(t *testing.T) {
	srv, rb := newTestRouterServer(t)
	conn := dialWS(t, srv, "acme")
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"op":"subscribe","resource":"clinic/visit"}`)); err != nil {
		t.Fatalf("write subscribe frame: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	rb.Hub().Broadcast("demo", events.EventMessage{Event: "created", Resource: "clinic/visit"})
	msg, ok := readOneMessage(t, conn, 2*time.Second)
	if !ok {
		t.Fatal("did not receive the subscribed resource's event")
	}
	if msg.Resource != "clinic/visit" || msg.Event != "created" {
		t.Errorf("got %+v, want created clinic/visit", msg)
	}

	// Unsubscribed resource must NOT arrive — expect the read to time out.
	rb.Hub().Broadcast("demo", events.EventMessage{Event: "created", Resource: "pharmacy/prescription"})
	ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel2()
	if _, data, err := conn.Read(ctx2); err == nil {
		t.Fatalf("received event for unsubscribed resource: %s", data)
	}
}

// TestHandleWS_UnsubscribeFrameEndToEnd verifies unsubscribe over the wire:
// after unsubscribing, the previously subscribed resource is no longer pushed.
func TestHandleWS_UnsubscribeFrameEndToEnd(t *testing.T) {
	srv, rb := newTestRouterServer(t)
	conn := dialWS(t, srv, "acme")
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"op":"subscribe","resource":"clinic/visit"}`)); err != nil {
		t.Fatalf("write subscribe frame: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	rb.Hub().Broadcast("demo", events.EventMessage{Event: "created", Resource: "clinic/visit"})
	msg, ok := readOneMessage(t, conn, 2*time.Second)
	if !ok {
		t.Fatal("did not receive the subscribed resource's event")
	}
	if msg.Resource != "clinic/visit" {
		t.Errorf("got %+v, want clinic/visit", msg)
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()
	if err := conn.Write(ctx3, websocket.MessageText, []byte(`{"op":"unsubscribe","resource":"clinic/visit"}`)); err != nil {
		t.Fatalf("write unsubscribe frame: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// After unsubscribe, expect NO message — the read times out (final op).
	rb.Hub().Broadcast("demo", events.EventMessage{Event: "updated", Resource: "clinic/visit"})
	ctx4, cancel4 := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel4()
	if _, data, err := conn.Read(ctx4); err == nil {
		t.Fatalf("received event after unsubscribe: %s", data)
	}
}
