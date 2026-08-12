// Package resource implements the Resource Plane side of the Plane Protocol:
// snapshot fetching, artifact deployment, evidence submission, and local
// deployment state management.
package resource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/primadi/formspec/internal/artifact"
)

// LocalManifestManager manages the local deployment manifest file.
// This file tracks what artifacts are currently deployed and their sha256,
// enabling hash-based deployment optimization.
type LocalManifestManager struct {
	mu       sync.RWMutex
	filePath string
	manifest *artifact.LocalDeploymentManifest
}

// NewLocalManifestManager creates or loads a local deployment manifest.
func NewLocalManifestManager(dataDir string) (*LocalManifestManager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	path := filepath.Join(dataDir, "deployment_manifest.json")
	mgr := &LocalManifestManager{
		filePath: path,
		manifest: &artifact.LocalDeploymentManifest{
			ControlVersion: 0,
			Artifacts:      make(map[string]artifact.LocalArtifactState),
		},
	}

	// Load existing manifest if present
	data, err := os.ReadFile(path)
	if err == nil {
		var loaded artifact.LocalDeploymentManifest
		if err := json.Unmarshal(data, &loaded); err == nil {
			if loaded.Artifacts == nil {
				loaded.Artifacts = make(map[string]artifact.LocalArtifactState)
			}
			mgr.manifest = &loaded
		}
	}

	return mgr, nil
}

// GetControlVersion returns the last known control snapshot version.
func (m *LocalManifestManager) GetControlVersion() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.manifest.ControlVersion
}

// SetControlVersion updates the last known control snapshot version.
func (m *LocalManifestManager) SetControlVersion(version int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.manifest.ControlVersion = version
	return m.save()
}

// GetArtifactHash returns the sha256 of a deployed artifact by key ("app/name").
// Returns empty string if not found.
func (m *LocalManifestManager) GetArtifactHash(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.manifest.Artifacts[key]; ok {
		return s.SHA256
	}
	return ""
}

// GetArtifactState returns the full state of a deployed artifact.
func (m *LocalManifestManager) GetArtifactState(key string) (artifact.LocalArtifactState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.manifest.Artifacts[key]
	return s, ok
}

// SetArtifactState updates the state of a deployed artifact.
func (m *LocalManifestManager) SetArtifactState(key string, state artifact.LocalArtifactState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.manifest.Artifacts[key] = state
	return m.save()
}

// RemoveArtifact removes a deployed artifact from the manifest.
func (m *LocalManifestManager) RemoveArtifact(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.manifest.Artifacts, key)
	return m.save()
}

// Snapshot returns a copy of the current manifest.
func (m *LocalManifestManager) Snapshot() artifact.LocalDeploymentManifest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.manifest
}

func (m *LocalManifestManager) save() error {
	data, err := json.MarshalIndent(m.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(m.filePath, data, 0644)
}
