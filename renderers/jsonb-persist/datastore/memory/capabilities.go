package memory

// Extended Storage capabilities (plan: storage-links-plan.md Fase 2):
// Stater (object size), Deleter (object removal), and ChunkUploader
// (parts-dir + concat) for the dev-mode filesystem backend — the mirror of
// internal/starlark's capability interfaces and the sidecar contract.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// chunkDir is the root-relative directory holding in-flight chunk sessions.
const chunkDir = ".chunks"

// newUploadID returns a random 128-bit hex id for a chunk session.
func newUploadID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is effectively fatal; fall back to time-based.
		return fmt.Sprintf("u%d", os.Getpid())
	}
	return hex.EncodeToString(b)
}

// Stat returns the object's size in bytes (todo 7.17.7).
func (s *Storage) Stat(_ context.Context, path string) (int64, error) {
	full, err := s.resolve(path)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", path, err)
	}
	return info.Size(), nil
}

// Delete removes the object at path (todo 7.17.6 — delete-after-download
// and TTL sweep). A missing object is a no-op so sweeps stay idempotent.
func (s *Storage) Delete(_ context.Context, path string) error {
	full, err := s.resolve(path)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	return nil
}

// InitChunkUpload creates a chunk session directory and returns its id
// (todo 7.17.5).
func (s *Storage) InitChunkUpload(_ context.Context, path string) (string, error) {
	if _, err := s.resolve(path); err != nil {
		return "", err // validate the target path up front
	}
	id := newUploadID()
	dir, err := s.resolve(filepath.Join(chunkDir, id))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("init chunk upload: %w", err)
	}
	// Record the target object path so complete can assemble even though
	// the caller only carries the upload id.
	if err := os.WriteFile(filepath.Join(dir, "target"), []byte(path), 0o644); err != nil {
		return "", fmt.Errorf("init chunk upload: %w", err)
	}
	return id, nil
}

// PutChunk writes one part file into the session directory (todo 7.17.5).
func (s *Storage) PutChunk(_ context.Context, uploadID string, partNo int, data []byte) error {
	if uploadID == "" || partNo < 0 || strings.Contains(uploadID, "/") {
		return fmt.Errorf("put_chunk: invalid upload id %q or part %d", uploadID, partNo)
	}
	dir, err := s.resolve(filepath.Join(chunkDir, uploadID))
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("put_chunk: unknown upload id %q", uploadID)
	}
	partPath := filepath.Join(dir, fmt.Sprintf("part-%06d", partNo))
	if err := os.WriteFile(partPath, data, 0o644); err != nil {
		return fmt.Errorf("put_chunk part %d: %w", partNo, err)
	}
	return nil
}

// CompleteChunkUpload concatenates the parts in order into the final object
// and removes the session directory (todo 7.17.5).
func (s *Storage) CompleteChunkUpload(_ context.Context, uploadID string) (string, error) {
	if uploadID == "" || strings.Contains(uploadID, "/") {
		return "", fmt.Errorf("complete_upload: invalid upload id %q", uploadID)
	}
	dir, err := s.resolve(filepath.Join(chunkDir, uploadID))
	if err != nil {
		return "", err
	}
	targetRaw, err := os.ReadFile(filepath.Join(dir, "target"))
	if err != nil {
		return "", fmt.Errorf("complete_upload: unknown upload id %q", uploadID)
	}
	target := string(targetRaw)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("complete_upload: read parts %q: %w", uploadID, err)
	}
	var parts []string
	for _, e := range entries {
		if name := e.Name(); strings.HasPrefix(name, "part-") {
			parts = append(parts, filepath.Join(dir, name))
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("complete_upload %q: no parts uploaded", uploadID)
	}
	sort.Strings(parts)

	full, err := s.resolve(target)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	out, err := os.Create(full)
	if err != nil {
		return "", fmt.Errorf("complete_upload: create %s: %w", target, err)
	}
	defer out.Close()
	for _, p := range parts {
		data, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("complete_upload: read part: %w", err)
		}
		if _, err := out.Write(data); err != nil {
			return "", fmt.Errorf("complete_upload: write %s: %w", target, err)
		}
	}
	// Session cleanup is best-effort — the object is already assembled.
	_ = os.RemoveAll(dir)
	return target, nil
}
