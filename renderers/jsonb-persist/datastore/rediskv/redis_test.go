package rediskv

import (
	"context"
	"os"
	"testing"
	"time"
)

// redisAddr returns the Redis/Valkey address to test against, or "" to skip
// (same pattern as internal/stream/redis_test.go — the dev container exposes
// a Redis-compatible Valkey service at "valkey:6379").
func redisAddr() string {
	if v := os.Getenv("FORMSPEC_TEST_REDIS"); v != "" {
		return v
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "valkey:6379" // dev container service
	}
	return "" // skip — no local redis assumed
}

func TestKV_SetGetDelete(t *testing.T) {
	addr := redisAddr()
	if addr == "" {
		t.Skip("no redis test target (set FORMSPEC_TEST_REDIS)")
	}
	kv, err := New(addr, "test")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer kv.Close()
	ctx := context.Background()

	// Miss → (nil, nil), no error.
	v, err := kv.Get(ctx, "missing")
	if err != nil || v != nil {
		t.Fatalf("get missing: want (nil,nil), got (%v,%v)", v, err)
	}

	// Set + Get roundtrip (JSON-encoded value).
	if err := kv.Set(ctx, "k1", map[string]any{"a": 1}, time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, err = kv.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	m, ok := v.(map[string]any)
	if !ok || m["a"] != float64(1) {
		t.Fatalf("get: want {a:1}, got %v", v)
	}

	// TTL expiry.
	if err := kv.Set(ctx, "k2", "x", 30*time.Millisecond); err != nil {
		t.Fatalf("set ttl: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	v, err = kv.Get(ctx, "k2")
	if err != nil || v != nil {
		t.Fatalf("get after ttl: want (nil,nil), got (%v,%v)", v, err)
	}

	// Delete.
	if err := kv.Set(ctx, "k3", 1, 0); err != nil {
		t.Fatalf("set k3: %v", err)
	}
	if err := kv.Delete(ctx, "k3"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if v, _ := kv.Get(ctx, "k3"); v != nil {
		t.Fatalf("get after delete: want nil, got %v", v)
	}
}

// TestKV_BroadcastInvalidate_TwoInstances: instance A deletes+broadcasts;
// instance B's copy of the key is gone (Fase 14 v2 — multi-instance
// invalidation via Redis pub/sub).
func TestKV_BroadcastInvalidate_TwoInstances(t *testing.T) {
	addr := redisAddr()
	if addr == "" {
		t.Skip("no redis test target (set FORMSPEC_TEST_REDIS)")
	}
	a, err := New(addr, "test")
	if err != nil {
		t.Fatalf("connect A: %v", err)
	}
	defer a.Close()
	b, err := New(addr, "test")
	if err != nil {
		t.Fatalf("connect B: %v", err)
	}
	defer b.Close()
	ctx := context.Background()

	// Both instances hold the same key.
	for _, kv := range []*KV{a, b} {
		if err := kv.Set(ctx, "shared", "v", time.Minute); err != nil {
			t.Fatalf("set: %v", err)
		}
	}

	// A mutates → broadcast. B must lose its copy.
	if err := a.BroadcastInvalidate(ctx, "shared"); err != nil {
		t.Fatalf("broadcast: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		v, err := b.Get(ctx, "shared")
		if err != nil {
			t.Fatalf("get on B: %v", err)
		}
		if v == nil {
			return // invalidated on B — success
		}
		if time.Now().After(deadline) {
			t.Fatalf("B still holds key after broadcast: %v", v)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
