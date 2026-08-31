package rediskv

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestQueue_FIFO proves the Redis-backed queue matches memory.Queue
// semantics: FIFO order and (nil, nil) — not an error — on empty dequeue.
func TestQueue_FIFO(t *testing.T) {
	addr := redisAddr()
	if addr == "" {
		t.Skip("no redis test target (set FORMSPEC_TEST_REDIS)")
	}
	// Unique namespace per run — a previous run's queue items must never
	// leak into this run.
	q, err := NewQueue(addr, fmt.Sprintf("test-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer q.Close()
	ctx := context.Background()

	// Empty dequeue → (nil, nil), no error.
	v, err := q.Dequeue(ctx, "jobs")
	if err != nil || v != nil {
		t.Fatalf("dequeue empty: want (nil,nil), got (%v,%v)", v, err)
	}

	// Enqueue two payloads, dequeue in FIFO order.
	if err := q.Enqueue(ctx, "jobs", "first"); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	if err := q.Enqueue(ctx, "jobs", map[string]any{"n": 2}); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	v, err = q.Dequeue(ctx, "jobs")
	if err != nil || v != "first" {
		t.Fatalf("dequeue 1: want (first,nil), got (%v,%v)", v, err)
	}
	v, err = q.Dequeue(ctx, "jobs")
	if err != nil {
		t.Fatalf("dequeue 2: %v", err)
	}
	m, ok := v.(map[string]any)
	if !ok || m["n"] != float64(2) {
		t.Fatalf("dequeue 2: want {n:2}, got %v", v)
	}

	// Drained again.
	v, err = q.Dequeue(ctx, "jobs")
	if err != nil || v != nil {
		t.Fatalf("dequeue drained: want (nil,nil), got (%v,%v)", v, err)
	}
}
