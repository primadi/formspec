package memory

import (
	"context"
	"testing"
	"time"
)

func TestKV_SetGetDelete(t *testing.T) {
	ctx := context.Background()
	kv := NewKV()

	if _, err := kv.Get(ctx, "missing"); err != nil {
		t.Fatalf("get missing: %v", err)
	}

	if err := kv.Set(ctx, "k", "v", 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := kv.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "v" {
		t.Fatalf("got %v, want v", got)
	}

	if err := kv.Delete(ctx, "k"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, _ = kv.Get(ctx, "k")
	if got != nil {
		t.Fatalf("got %v after delete, want nil", got)
	}
}

func TestKV_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	kv := NewKV()
	base := time.Now()
	kv.now = func() time.Time { return base }

	if err := kv.Set(ctx, "k", "v", time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, _ := kv.Get(ctx, "k")
	if got != "v" {
		t.Fatalf("got %v before expiry, want v", got)
	}

	// Advance past TTL.
	kv.now = func() time.Time { return base.Add(2 * time.Second) }
	got, _ = kv.Get(ctx, "k")
	if got != nil {
		t.Fatalf("got %v after expiry, want nil", got)
	}
}

func TestLock_AcquireRelease(t *testing.T) {
	ctx := context.Background()
	l := NewLock()

	ok, err := l.Acquire(ctx, "key", 0)
	if err != nil || !ok {
		t.Fatalf("first acquire: ok=%v err=%v", ok, err)
	}
	// Second acquire while held → false.
	ok, _ = l.Acquire(ctx, "key", 0)
	if ok {
		t.Fatalf("second acquire should fail while held")
	}
	if err := l.Release(ctx, "key"); err != nil {
		t.Fatalf("release: %v", err)
	}
	ok, _ = l.Acquire(ctx, "key", 0)
	if !ok {
		t.Fatalf("acquire after release should succeed")
	}
}

func TestLock_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	l := NewLock()
	base := time.Now()
	l.now = func() time.Time { return base }

	ok, _ := l.Acquire(ctx, "key", time.Second)
	if !ok {
		t.Fatalf("acquire failed")
	}
	// Still held.
	ok, _ = l.Acquire(ctx, "key", time.Second)
	if ok {
		t.Fatalf("acquire should fail while held")
	}
	// Advance past TTL → re-acquirable.
	l.now = func() time.Time { return base.Add(2 * time.Second) }
	ok, _ = l.Acquire(ctx, "key", time.Second)
	if !ok {
		t.Fatalf("acquire after TTL expiry should succeed")
	}
}

func TestQueue_EnqueueDequeue(t *testing.T) {
	ctx := context.Background()
	q := NewQueue()

	// Empty queue → nil.
	got, err := q.Dequeue(ctx, "jobs")
	if err != nil || got != nil {
		t.Fatalf("dequeue empty: got=%v err=%v", got, err)
	}

	if err := q.Enqueue(ctx, "jobs", "a"); err != nil {
		t.Fatalf("enqueue a: %v", err)
	}
	if err := q.Enqueue(ctx, "jobs", "b"); err != nil {
		t.Fatalf("enqueue b: %v", err)
	}
	got, _ = q.Dequeue(ctx, "jobs")
	if got != "a" {
		t.Fatalf("first dequeue got %v, want a", got)
	}
	got, _ = q.Dequeue(ctx, "jobs")
	if got != "b" {
		t.Fatalf("second dequeue got %v, want b", got)
	}
}

func TestPubSub_PublishSubscribe(t *testing.T) {
	ctx := context.Background()
	ps := NewPubSub()

	received := make(chan any, 1)
	if err := ps.Subscribe(ctx, "chan", func(p any) { received <- p }); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := ps.Publish(ctx, "chan", "hello"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case got := <-received:
		if got != "hello" {
			t.Fatalf("got %v, want hello", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("subscriber not invoked")
	}
}

func TestStorage_UploadDownload(t *testing.T) {
	ctx := context.Background()
	s, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	if err := s.Upload(ctx, "a/b.txt", []byte("data")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	got, err := s.Download(ctx, "a/b.txt")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(got) != "data" {
		t.Fatalf("got %q, want data", got)
	}
}

func TestStorage_TraversalNeutralized(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	s, err := NewStorage(root)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	// Traversal attempts are neutralized: the file lands under root, never
	// outside it.
	if err := s.Upload(ctx, "../../etc/passwd", []byte("x")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	got, err := s.Download(ctx, "../../etc/passwd")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(got) != "x" {
		t.Fatalf("got %q, want x", got)
	}
}
