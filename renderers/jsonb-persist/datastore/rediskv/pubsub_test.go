package rediskv

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestPubSub_PublishSubscribe proves the Redis-backed pub/sub delivers a
// JSON-encoded payload to a subscriber registered before the publish.
func TestPubSub_PublishSubscribe(t *testing.T) {
	addr := redisAddr()
	if addr == "" {
		t.Skip("no redis test target (set FORMSPEC_TEST_REDIS)")
	}
	p, err := NewPubSub(addr, fmt.Sprintf("test-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer p.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan any, 1)
	if err := p.Subscribe(ctx, "events", func(v any) { received <- v }); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	// Give the subscription goroutine time to register with the server.
	time.Sleep(150 * time.Millisecond)

	if err := p.Publish(ctx, "events", map[string]any{"kind": "created"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case v := <-received:
		m, ok := v.(map[string]any)
		if !ok || m["kind"] != "created" {
			t.Fatalf("received payload: want {kind:created}, got %v", v)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("no delivery within 3s")
	}
}
