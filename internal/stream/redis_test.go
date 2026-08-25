package stream

import (
	"context"
	"os"
	"testing"
)

// redisAddr returns the Redis/Valkey address to test against, or "" to skip.
// The dev container exposes a Redis-compatible Valkey service at "valkey:6379"
// (compose.yaml); override with FORMSPEC_REDIS_ADDR.
func redisAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("FORMSPEC_REDIS_ADDR")
	if addr == "" {
		addr = "valkey:6379"
	}
	r, err := NewRedis(addr)
	if err != nil {
		t.Skipf("redis not reachable at %s: %v", addr, err)
	}
	_ = r.Close()
	return addr
}

func TestRedis_AppendReadAck(t *testing.T) {
	addr := redisAddr(t)
	r, err := NewRedis(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()

	const streamName = "test.demo.products.on_create"
	const group = "test/audit"
	// Clean slate.
	_ = r.client.Del(ctx, streamName).Err()

	id1, err := r.Append(ctx, streamName, map[string]any{"name": "a"})
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := r.Append(ctx, streamName, map[string]any{"name": "b"})

	entries, err := r.Read(ctx, streamName, group, "c", "earliest", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("Read: want 2, got %d", len(entries))
	}
	if entries[0].ID != id1 || entries[1].ID != id2 {
		t.Errorf("Read order: want %s,%s got %s,%s", id1, id2, entries[0].ID, entries[1].ID)
	}
	if entries[0].Attempts != 1 {
		t.Errorf("Attempts: want 1, got %d", entries[0].Attempts)
	}

	// Ack both — next read returns nothing new.
	if err := r.Ack(ctx, streamName, group, id1); err != nil {
		t.Fatal(err)
	}
	if err := r.Ack(ctx, streamName, group, id2); err != nil {
		t.Fatal(err)
	}
	entries, err = r.Read(ctx, streamName, group, "c", "earliest", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("Read after ack: want 0, got %d", len(entries))
	}
}

func TestRedis_AtLeastOnceRetry(t *testing.T) {
	addr := redisAddr(t)
	r, err := NewRedis(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()

	const streamName = "test.demo.products.on_create"
	const group = "test/retry"
	_ = r.client.Del(ctx, streamName).Err()

	id, _ := r.Append(ctx, streamName, map[string]any{"v": 1})

	// First read — attempts=1.
	entries, _ := r.Read(ctx, streamName, group, "c", "earliest", 10)
	if len(entries) != 1 || entries[0].Attempts != 1 {
		t.Fatalf("first read: want 1 entry attempts=1, got %d attempts=%d", len(entries), entries[0].Attempts)
	}
	// Do NOT ack — next read must redeliver with attempts=2 (at-least-once).
	entries, _ = r.Read(ctx, streamName, group, "c", "earliest", 10)
	if len(entries) != 1 || entries[0].ID != id || entries[0].Attempts != 2 {
		t.Fatalf("redelivery: want id=%s attempts=2, got id=%s attempts=%d", id, entries[0].ID, entries[0].Attempts)
	}
	if err := r.Ack(ctx, streamName, group, id); err != nil {
		t.Fatal(err)
	}
	entries, _ = r.Read(ctx, streamName, group, "c", "earliest", 10)
	if len(entries) != 0 {
		t.Fatalf("after ack: want 0 entries, got %d", len(entries))
	}
}

func TestRedis_Trim(t *testing.T) {
	addr := redisAddr(t)
	r, err := NewRedis(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()

	const streamName = "test.demo.products.on_create"
	_ = r.client.Del(ctx, streamName).Err()

	for i := 0; i < 5; i++ {
		_, _ = r.Append(ctx, streamName, map[string]any{"i": i})
	}
	if err := r.Trim(ctx, streamName, "3"); err != nil {
		t.Fatal(err)
	}
	entries, _ := r.Read(ctx, streamName, "test/trim", "c", "earliest", 10)
	if len(entries) != 3 {
		t.Fatalf("trim by count: want 3 entries, got %d", len(entries))
	}
}