package minio

// Extended Storage capabilities (plan: storage-links-plan.md Fase 2):
// Stater (object size), Deleter (object removal), Linker (presigned URL),
// and ChunkUploader (S3 multipart upload) — the mirror of
// internal/starlark's capability interfaces and the sidecar contract.

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
)

// chunkSession tracks an in-flight multipart upload. MinIO's multipart
// upload id is server-side; we keep the object path and the completed parts
// so CompleteChunkUpload can submit them in order.
type chunkSession struct {
	path  string
	parts []minio.CompletePart
}

// multipartSessions is process-local state for chunked uploads. The sidecar
// callback runs in the same process (cmd/formspec/dev.go), so an in-memory
// map is sufficient; an orphaned session expires server-side via MinIO's
// lifecycle (IncompleteUploadAbortDays) and the sweeper can abort it.
type multipartSessions struct {
	mu       sync.Mutex
	sessions map[string]*chunkSession
}

var sessions = &multipartSessions{sessions: make(map[string]*chunkSession)}

func (m *multipartSessions) put(id string, s *chunkSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = s
}

func (m *multipartSessions) get(id string) (*chunkSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *multipartSessions) remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

// Stat returns the object's size in bytes (todo 7.17.7). Used by the
// download-size limit (413 without loading the object).
func (s *Storage) Stat(ctx context.Context, path string) (int64, error) {
	info, err := s.client.StatObject(ctx, s.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		return 0, fmt.Errorf("minio stat %s: %w", path, err)
	}
	return info.Size, nil
}

// Delete removes the object at path (todo 7.17.6 — delete-after-download
// and TTL sweep).
func (s *Storage) Delete(ctx context.Context, path string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, path, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("minio delete %s: %w", path, err)
	}
	return nil
}

// Link returns a presigned GET URL valid for ttl (todo 7.17.4). This is the
// `visibility: signed` path — permission is checked at link-generation time,
// then MinIO serves the bytes directly.
func (s *Storage) Link(ctx context.Context, path string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = DefaultLinkTTL
	}
	// S3 caps presigned URLs at 7 days.
	if ttl > 7*24*time.Hour {
		ttl = 7 * 24 * time.Hour
	}
	u, err := s.client.PresignedGetObject(ctx, s.bucket, path, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("minio presign %s: %w", path, err)
	}
	return u.String(), nil
}

// DefaultLinkTTL is used when a link request carries no explicit TTL.
const DefaultLinkTTL = 15 * time.Minute

// InitChunkUpload starts an S3 multipart upload and returns its id
// (todo 7.17.5).
func (s *Storage) InitChunkUpload(ctx context.Context, path string) (string, error) {
	uploadID, err := s.core.NewMultipartUpload(ctx, s.bucket, path, minio.PutObjectOptions{
		ContentType: contentTypeFor(path),
	})
	if err != nil {
		return "", fmt.Errorf("minio init multipart %s: %w", path, err)
	}
	id := uploadID // S3's upload id is already globally unique
	sessions.put(id, &chunkSession{path: path})
	return id, nil
}

// PutChunk uploads one part of an in-flight multipart upload (todo 7.17.5).
func (s *Storage) PutChunk(ctx context.Context, uploadID string, partNo int, data []byte) error {
	sess, ok := sessions.get(uploadID)
	if !ok {
		return fmt.Errorf("minio put_chunk: unknown upload id %q", uploadID)
	}
	// Part numbers are 1-based; PutObjectPart lives on minio.Core.
	partNumber := partNo + 1
	obj, err := s.core.PutObjectPart(ctx, s.bucket, sess.path, uploadID,
		partNumber, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
	if err != nil {
		return fmt.Errorf("minio put part %d %s: %w", partNumber, sess.path, err)
	}
	sess.parts = append(sess.parts, minio.CompletePart{
		PartNumber: partNumber,
		ETag:       obj.ETag,
	})
	return nil
}

// CompleteChunkUpload assembles the parts into the final object and returns
// its path (todo 7.17.5).
func (s *Storage) CompleteChunkUpload(ctx context.Context, uploadID string) (string, error) {
	sess, ok := sessions.get(uploadID)
	if !ok {
		return "", fmt.Errorf("minio complete_upload: unknown upload id %q", uploadID)
	}
	if len(sess.parts) == 0 {
		return "", fmt.Errorf("minio complete_upload %s: no parts uploaded", sess.path)
	}
	sort.Slice(sess.parts, func(i, j int) bool {
		return sess.parts[i].PartNumber < sess.parts[j].PartNumber
	})
	_, err := s.core.CompleteMultipartUpload(ctx, s.bucket, sess.path, uploadID,
		sess.parts, minio.PutObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("minio complete multipart %s: %w", sess.path, err)
	}
	sessions.remove(uploadID)
	return sess.path, nil
}
