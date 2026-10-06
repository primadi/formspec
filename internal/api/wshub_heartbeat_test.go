package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fastHeartbeat shortens the hub's heartbeat knobs so a test can observe
// several cycles without sleeping for the production default (25s).
func fastHeartbeat(rb *RouterBuilder, interval, timeout time.Duration) {
	rb.Hub().hbInterval = interval
	rb.Hub().hbTimeout = timeout
}

// TestHandleWS_HeartbeatFrameReachesClient verifies the connection's single
// heartbeat loop emits an application-level `hb` frame on the wire — the only
// liveness signal a JS client can actually observe (protocol pings are
// invisible to JavaScript).
func TestHandleWS_HeartbeatFrameReachesClient(t *testing.T) {
	srv, rb := newTestRouterServer(t)
	fastHeartbeat(rb, 60*time.Millisecond, 200*time.Millisecond)

	conn := dialWS(t, srv, "acme")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read heartbeat: %v", err)
		}
		var f hbFrame
		if json.Unmarshal(data, &f) != nil {
			t.Fatalf("heartbeat frame is not JSON: %s", data)
		}
		if f.Op == "hb" {
			if f.Ts == 0 {
				t.Error("heartbeat frame has no timestamp")
			}
			return
		}
	}
}

// TestHandleWS_HeartbeatKeepsAckedConnectionAlive is the negative control for
// the timeout: a client that answers hb_ack must NOT be closed.
func TestHandleWS_HeartbeatKeepsAckedConnectionAlive(t *testing.T) {
	srv, rb := newTestRouterServer(t)
	fastHeartbeat(rb, 50*time.Millisecond, 150*time.Millisecond)

	conn := dialWS(t, srv, "acme")

	// Keep acknowledging for longer than interval+timeout (a stalled peer would
	// already be closed by now).
	deadline := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		_, data, err := conn.Read(ctx)
		cancel()
		if err != nil {
			t.Fatalf("acked connection was closed early: %v", err)
		}
		var f hbFrame
		if json.Unmarshal(data, &f) == nil && f.Op == "hb" {
			wctx, wcancel := context.WithTimeout(context.Background(), time.Second)
			err := conn.Write(wctx, websocket.MessageText, []byte(`{"op":"hb_ack"}`))
			wcancel()
			if err != nil {
				t.Fatalf("write hb_ack: %v", err)
			}
		}
	}
}

// TestHandleWS_HeartbeatClosesStalledConnection verifies the server stops
// waiting on a half-open peer: with no inbound frame, the connection is closed
// and unregistered, so the hub's connection count drops back to zero.
func TestHandleWS_HeartbeatClosesStalledConnection(t *testing.T) {
	srv, rb := newTestRouterServer(t)
	fastHeartbeat(rb, 40*time.Millisecond, 40*time.Millisecond)

	conn := dialWS(t, srv, "acme")

	// Never write anything: just drain frames until the server closes.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			break // closed, as intended
		}
	}

	// The deferred unregister must have run — no stale connection left behind.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !rb.Hub().HasListeners("acme") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("stalled connection was closed but never unregistered")
}
