package memory

import (
	"bytes"
	"context"
	"testing"
)

// TestStorageStatDelete verifies the extended capabilities (todo 7.17.4/7.17.6):
// Stat reports the object size and Delete removes it (missing = no-op).
func TestStorageStatDelete(t *testing.T) {
	ctx := context.Background()
	s, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	if err := s.Upload(ctx, "dir/a.txt", []byte("hello world")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	size, err := s.Stat(ctx, "dir/a.txt")
	if err != nil || size != 11 {
		t.Fatalf("stat: size=%d err=%v (want 11, nil)", size, err)
	}
	if err := s.Delete(ctx, "dir/a.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Delete of a missing object is a no-op (idempotent sweeps).
	if err := s.Delete(ctx, "dir/a.txt"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
	if _, err := s.Stat(ctx, "dir/a.txt"); err == nil {
		t.Fatalf("stat deleted: expected error")
	}
}

// TestStorageChunkUpload verifies the chunked upload contract (todo 7.17.5):
// init → parts → complete assembles the parts in order and cleans the
// session; an empty or unknown session fails.
func TestStorageChunkUpload(t *testing.T) {
	ctx := context.Background()
	s, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	id, err := s.InitChunkUpload(ctx, "dir/big.bin")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	for i, part := range [][]byte{[]byte("AAA"), []byte("BBB"), []byte("CCC")} {
		if err := s.PutChunk(ctx, id, i, part); err != nil {
			t.Fatalf("put chunk %d: %v", i, err)
		}
	}
	path, err := s.CompleteChunkUpload(ctx, id)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if path != "dir/big.bin" {
		t.Fatalf("complete: path=%q", path)
	}
	got, err := s.Download(ctx, path)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if !bytes.Equal(got, []byte("AAABBBCCC")) {
		t.Fatalf("download: got %q, want %q", got, "AAABBBCCC")
	}

	// Session is consumed — a second complete fails.
	if _, err := s.CompleteChunkUpload(ctx, id); err == nil {
		t.Fatalf("complete twice: expected error")
	}
	// Unknown upload id fails.
	if err := s.PutChunk(ctx, "nonexistent", 0, []byte("x")); err == nil {
		t.Fatalf("put chunk unknown id: expected error")
	}
}
