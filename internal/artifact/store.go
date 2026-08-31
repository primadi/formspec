package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Store is the persistence layer for artifacts, deployments, and evidence.
// In production, this is backed by the Control Plane database (PostgreSQL).
// In development, a SQLite or in-memory implementation is used.
type Store interface {
	// Artifact CRUD
	CreateArtifact(ctx context.Context, a *Artifact) error
	GetArtifact(ctx context.Context, id ArtifactID) (*Artifact, error)
	ListArtifacts(ctx context.Context, app string) ([]*Artifact, error)
	GetLatestArtifact(ctx context.Context, app string) (*Artifact, error)

	// Deployment state
	UpsertDeployment(ctx context.Context, d *Deployment) error
	ListDeployments(ctx context.Context, workspaceID string) ([]*Deployment, error)
	GetDeployment(ctx context.Context, workspaceID, app string) (*Deployment, error)

	// Datastore registry (plan fase B2 — Workspace Binding): registered
	// kind: Datastore services, evaluated per-workspace at snapshot time
	// via access.filter.
	UpsertDatastore(ctx context.Context, ds *DatastoreRegistration) error
	ListDatastores(ctx context.Context) ([]*DatastoreRegistration, error)

	// Evidence (append-only)
	AppendEvidence(ctx context.Context, e *EvidenceRecord) error
	ListEvidence(ctx context.Context, instanceID string, since time.Time) ([]*EvidenceRecord, error)

	// Snapshot version management
	CurrentSnapshotVersion(ctx context.Context) (int, error)
	IncrementSnapshotVersion(ctx context.Context) (int, error)

	Close() error
}

// MemStore is an in-memory implementation of Store, suitable for dev and testing.
type MemStore struct {
	mu             sync.RWMutex
	artifacts      map[ArtifactID]*Artifact
	deployments    map[string]*Deployment // key = "workspace/app"
	datastores     map[string]*DatastoreRegistration
	evidence       []*EvidenceRecord
	snapVersion    int
	nextArtifactID int
}

// NewMemStore creates a new in-memory artifact store.
func NewMemStore() *MemStore {
	return &MemStore{
		artifacts:   make(map[ArtifactID]*Artifact),
		deployments: make(map[string]*Deployment),
		datastores:  make(map[string]*DatastoreRegistration),
		evidence:    make([]*EvidenceRecord, 0),
		snapVersion: 1,
	}
}

// DatastoreRegistration is one registered kind: Datastore service at the
// Control Plane (plan fase B2): the spec plus the workspace metadata used
// to evaluate access.filter at snapshot time.
type DatastoreRegistration struct {
	// Name is the logical service name (metadata.name).
	Name string `json:"name"`
	// Spec is the DatastoreSpec as JSON (driver, connection, serves, access).
	Spec json.RawMessage `json:"spec"`
	// Environment is the environment this service belongs to (matched
	// against the snapshot's environment).
	Environment string `json:"environment,omitempty"`
	// Labels are service labels matched against workspace labels.
	Labels map[string]string `json:"labels,omitempty"`
}

func deploymentKey(workspaceID, app string) string {
	return workspaceID + "/" + app
}

func (m *MemStore) CreateArtifact(ctx context.Context, a *Artifact) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextArtifactID++
	a.ID = ArtifactID(fmt.Sprintf("art-%d", m.nextArtifactID))
	a.CreatedAt = time.Now().UTC()
	if a.Status == "" {
		a.Status = ArtifactStatusActive
	}
	m.artifacts[a.ID] = a
	return nil
}

func (m *MemStore) GetArtifact(ctx context.Context, id ArtifactID) (*Artifact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, ok := m.artifacts[id]
	if !ok {
		return nil, fmt.Errorf("artifact %s: not found", id)
	}
	return a, nil
}

func (m *MemStore) ListArtifacts(ctx context.Context, app string) ([]*Artifact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Artifact
	for _, a := range m.artifacts {
		if a.App == app {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *MemStore) GetLatestArtifact(ctx context.Context, app string) (*Artifact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *Artifact
	for _, a := range m.artifacts {
		if a.App == app && a.Status == ArtifactStatusActive {
			if latest == nil || a.Version > latest.Version {
				latest = a
			}
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("no active artifact for app %s", app)
	}
	return latest, nil
}

func (m *MemStore) UpsertDeployment(ctx context.Context, d *Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := deploymentKey(d.WorkspaceID, d.App)
	existing, ok := m.deployments[key]
	if ok {
		d.ID = existing.ID
		d.CreatedAt = existing.CreatedAt
	} else {
		d.CreatedAt = time.Now().UTC()
	}
	m.deployments[key] = d
	return nil
}

func (m *MemStore) ListDeployments(ctx context.Context, workspaceID string) ([]*Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Deployment
	for _, d := range m.deployments {
		if d.WorkspaceID == workspaceID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *MemStore) GetDeployment(ctx context.Context, workspaceID, app string) (*Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.deployments[deploymentKey(workspaceID, app)]
	if !ok {
		return nil, fmt.Errorf("deployment %s/%s: not found", workspaceID, app)
	}
	return d, nil
}

// UpsertDatastore registers (or replaces) a kind: Datastore service in the
// Control Plane registry (plan fase B2).
func (m *MemStore) UpsertDatastore(ctx context.Context, ds *DatastoreRegistration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.datastores[ds.Name] = ds
	return nil
}

// ListDatastores returns all registered datastore services.
func (m *MemStore) ListDatastores(ctx context.Context) ([]*DatastoreRegistration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DatastoreRegistration
	for _, ds := range m.datastores {
		result = append(result, ds)
	}
	return result, nil
}

func (m *MemStore) AppendEvidence(ctx context.Context, e *EvidenceRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	e.ReceivedAt = time.Now().UTC()
	m.evidence = append(m.evidence, e)
	return nil
}

func (m *MemStore) ListEvidence(ctx context.Context, instanceID string, since time.Time) ([]*EvidenceRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*EvidenceRecord
	for _, e := range m.evidence {
		if e.InstanceID == instanceID && e.ReceivedAt.After(since) {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *MemStore) CurrentSnapshotVersion(ctx context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapVersion, nil
}

func (m *MemStore) IncrementSnapshotVersion(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapVersion++
	return m.snapVersion, nil
}

func (m *MemStore) Close() error { return nil }

// Ensure MemStore implements Store.
var _ Store = (*MemStore)(nil)

// ---- Errors ----

// ErrNotFound is returned when an artifact or deployment is not found.
var ErrNotFound = fmt.Errorf("not found")

// ErrAlreadyExists is returned when attempting to create a duplicate artifact.
var ErrAlreadyExists = fmt.Errorf("already exists")

// ---- Helpers ----

// MarshalPayload marshals a deploy status payload into json.RawMessage.
func MarshalPayload(p *DeployStatusPayload) (json.RawMessage, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal deploy status: %w", err)
	}
	return json.RawMessage(data), nil
}
