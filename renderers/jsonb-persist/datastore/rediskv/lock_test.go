package rediskv

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestLock_AcquireRelease proves the Redis-backed lock matches memory.Lock
// semantics: exclusive acquire, non-holder release is a no-op, holder
// release frees the lock, and TTL expiry makes it re-acquirable.
func TestLock_AcquireRelease(t *testing.T) {
	addr := redisAddr()
	if addr == "" {
		t.Skip("no redis test target (set FORMSPEC_TEST_REDIS)")
	}
	// Unique namespace per run — a previous run's keys (e.g. a 1-minute
	// lock TTL) must never leak into this run.
	ns := fmt.Sprintf("test-%d", time.Now().UnixNano())
	l1, err := NewLock(addr, ns)
	if err != nil {
		t.Fatalf("connect l1: %v", err)
	}
	defer l1.Close()
	l2, err := NewLock(addr, ns)
	if err != nil {
		t.Fatalf("connect l2: %v", err)
	}
	defer l2.Close()
	ctx := context.Background()

	// First acquirer wins.
	ok, err := l1.Acquire(ctx, "job", time.Minute)
	if err != nil || !ok {
		t.Fatalf("first acquire: want (true,nil), got (%v,%v)", ok, err)
	}

	// Second acquirer (different token) is rejected.
	ok, err = l2.Acquire(ctx, "job", time.Minute)
	if err != nil || ok {
		t.Fatalf("second acquire: want (false,nil), got (%v,%v)", ok, err)
	}

	// Non-holder release is a no-op — lock stays held.
	if err := l2.Release(ctx, "job"); err != nil {
		t.Fatalf("non-holder release: %v", err)
	}
	if ok, _ := l2.Acquire(ctx, "job", time.Minute); ok {
		t.Fatalf("lock was freed by a non-holder release")
	}

	// Holder release frees the lock.
	if err := l1.Release(ctx, "job"); err != nil {
		t.Fatalf("holder release: %v", err)
	}
	ok, err = l2.Acquire(ctx, "job", time.Minute)
	if err != nil || !ok {
		t.Fatalf("re-acquire after holder release: want (true,nil), got (%v,%v)", ok, err)
	}

	// TTL expiry: a lock with a short TTL becomes re-acquirable.
	l2.Release(ctx, "job")
	if ok, _ := l1.Acquire(ctx, "ttl", 30*time.Millisecond); !ok {
		t.Fatalf("ttl acquire: want true")
	}
	time.Sleep(80 * time.Millisecond)
	if ok, _ := l2.Acquire(ctx, "ttl", time.Minute); !ok {
		t.Fatalf("acquire after ttl expiry: want true")
	}
}
