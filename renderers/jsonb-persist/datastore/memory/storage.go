package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Storage is a filesystem-backed object store used for ctx.storage() in dev
// mode (docs/spec/platform/06-datastore.md §5: storage → filesystem lokal).
// Paths are resolved under a root directory and sanitized to prevent
// directory traversal.
type Storage struct {
	root string
}

// NewStorage creates a filesystem storage rooted at root (created if absent).
func NewStorage(root string) (*Storage, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Storage{root: root}, nil
}

// resolve safely joins a storage path under root, rejecting traversal.
func (s *Storage) resolve(path string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimPrefix(path, "/"))
	full := filepath.Join(s.root, clean)
	rel, err := filepath.Rel(s.root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", os.ErrPermission
	}
	return full, nil
}

// Upload writes data to path (creating parent directories).
func (s *Storage) Upload(_ context.Context, path string, data []byte) error {
	full, err := s.resolve(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

// Download reads data from path.
func (s *Storage) Download(_ context.Context, path string) ([]byte, error) {
	full, err := s.resolve(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}
