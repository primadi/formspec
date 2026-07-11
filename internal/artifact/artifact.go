// Package artifact provides types and operations for Forma artifact
// management — the signed envelope format used by the Control Plane to
// store and distribute YAML manifests, scripts, and assets to Resource Planes.
//
// An Artifact is the unit of deployment: a collection of YAML manifests
// (and optional scripts/assets) bundled into a signed envelope. Every
// artifact has a sha256 content hash and a monotonic version number.
package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ArtifactID is a unique identifier for an artifact.
type ArtifactID string

// FileManifest describes a single file within an artifact envelope.
type FileManifest struct {
	Path    string `json:"path"`    // relative path, e.g. "billing/invoice.yaml"
	SHA256  string `json:"sha256"`  // hex-encoded sha256 of content
	Content []byte `json:"content"` // raw file content (base64-encoded in JSON)
}

// ArtifactEnvelope is the signed bundle distributed to Resource Planes.
type ArtifactEnvelope struct {
	ArtifactID   ArtifactID     `json:"artifact_id"`
	App          string         `json:"app"`     // app name from kind: App
	Version      int            `json:"version"` // monotonic version per app
	SHA256       string         `json:"sha256"`  // aggregate sha256 of all files
	Files        []FileManifest `json:"files"`
	Signature    string         `json:"signature"`      // hex-encoded ed25519 signature
	SigningKeyID string         `json:"signing_key_id"` // identity of the signing key
	PrevVersion  int            `json:"prev_version,omitempty"`
	PrevSHA256   string         `json:"prev_sha256,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// Artifact is the stored representation in the Control DB.
type Artifact struct {
	ID        ArtifactID        `json:"id"`
	App       string            `json:"app"`
	Version   int               `json:"version"`
	SHA256    string            `json:"sha256"`
	Envelope  *ArtifactEnvelope `json:"envelope"`
	Status    ArtifactStatus    `json:"status"` // active | superseded | revoked
	CreatedAt time.Time         `json:"created_at"`
}

// ArtifactStatus represents the lifecycle status of an artifact.
type ArtifactStatus string

const (
	ArtifactStatusActive     ArtifactStatus = "active"
	ArtifactStatusSuperseded ArtifactStatus = "superseded"
	ArtifactStatusRevoked    ArtifactStatus = "revoked"
)

// Deployment represents the desired deployment state of an artifact
// to a specific workspace.
type Deployment struct {
	ID          ArtifactID `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	App         string     `json:"app"`
	ArtifactID  ArtifactID `json:"artifact_id"`
	Version     int        `json:"version"`
	SHA256      string     `json:"sha256"`
	Status      string     `json:"status"` // pending | deployed | rolled_back
	CreatedAt   time.Time  `json:"created_at"`
}

// EvidenceType enumerates the types of evidence a Resource Plane can submit.
type EvidenceType string

const (
	EvidenceDeployStatus EvidenceType = "deploy_status"
	EvidenceMetering     EvidenceType = "metering"
	EvidenceAuditAnchor  EvidenceType = "audit_anchor"
	EvidenceViolation    EvidenceType = "violation"
	EvidenceHealth       EvidenceType = "health"
)

// DeployPhase enumerates the phases of artifact deployment.
type DeployPhase string

const (
	DeployPhaseUpToDate   DeployPhase = "up_to_date"
	DeployPhaseFetched    DeployPhase = "fetched"
	DeployPhaseVerified   DeployPhase = "verified"
	DeployPhaseLoaded     DeployPhase = "loaded"
	DeployPhaseFailed     DeployPhase = "failed"
	DeployPhaseRolledBack DeployPhase = "rolled_back"
)

// DeployStatusPayload is the payload for deploy_status evidence.
type DeployStatusPayload struct {
	ArtifactID ArtifactID  `json:"artifact_id"`
	App        string      `json:"app"`
	Version    int         `json:"version"`
	SHA256     string      `json:"sha256"`
	Phase      DeployPhase `json:"phase"`
	Error      string      `json:"error,omitempty"`
}

// EvidenceRecord is a single evidence record submitted by a Resource Plane.
type EvidenceRecord struct {
	Type       EvidenceType    `json:"type"`
	InstanceID string          `json:"instance_id"`
	Sequence   int64           `json:"sequence"`
	Payload    json.RawMessage `json:"payload"`
	Signature  string          `json:"signature"`
	ReceivedAt time.Time       `json:"received_at,omitempty"`
}

// Snapshot is the desired-state bundle sent from Control to Resource Plane.
type Snapshot struct {
	Version     int             `json:"version"`
	IssuedAt    time.Time       `json:"issued_at"`
	Environment string          `json:"environment"`
	Signature   string          `json:"signature"`
	Deployments []Deployment    `json:"deployments"`
	Policy      json.RawMessage `json:"policy,omitempty"`
	Trust       json.RawMessage `json:"trust,omitempty"`
	Grants      json.RawMessage `json:"grants,omitempty"`
	Licenses    json.RawMessage `json:"licenses,omitempty"`
	Revocations json.RawMessage `json:"revocations,omitempty"`
	Memberships json.RawMessage `json:"memberships,omitempty"`
}

// LocalDeploymentManifest is the local state file maintained by the Resource Plane.
type LocalDeploymentManifest struct {
	ControlVersion int                           `json:"control_version"`
	Artifacts      map[string]LocalArtifactState `json:"artifacts"` // key = "app/name"
}

// LocalArtifactState tracks the state of a single deployed artifact.
type LocalArtifactState struct {
	ArtifactID ArtifactID `json:"artifact_id"`
	App        string     `json:"app"`
	Version    int        `json:"version"`
	SHA256     string     `json:"sha256"`
	LoadedAt   time.Time  `json:"loaded_at"`
	Status     string     `json:"status"` // active | rolled_back
}

// ComputeSHA256 computes the aggregate sha256 of a set of file contents.
// Files are sorted by path for deterministic output.
func ComputeSHA256(files []FileManifest) string {
	sorted := make([]FileManifest, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	h := sha256.New()
	for _, f := range sorted {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write(f.Content)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// FileSHA256 computes the sha256 of a single byte slice.
func FileSHA256(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// ValidateEnvelopeIntegrity verifies that all file hashes and the aggregate
// hash in the envelope are correct.
func ValidateEnvelopeIntegrity(e *ArtifactEnvelope) error {
	if e == nil {
		return fmt.Errorf("envelope is nil")
	}

	// Verify each file's individual sha256
	for _, f := range e.Files {
		expected := strings.ToLower(f.SHA256)
		actual := strings.ToLower(FileSHA256(f.Content))
		if expected != actual {
			return fmt.Errorf("file %s: sha256 mismatch (expected %s, got %s)",
				f.Path, expected, actual)
		}
	}

	// Verify aggregate sha256
	expected := strings.ToLower(e.SHA256)
	actual := strings.ToLower(ComputeSHA256(e.Files))
	if expected != actual {
		return fmt.Errorf("envelope: aggregate sha256 mismatch (expected %s, got %s)",
			expected, actual)
	}

	return nil
}
