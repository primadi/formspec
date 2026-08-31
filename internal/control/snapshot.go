package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/primadi/formspec/internal/artifact"
	"github.com/primadi/formspec/pkg/spec"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore"
)

// SnapshotHandler handles GET /v1/snapshot — builds and returns the
// desired-state snapshot to Resource Planes using ETag-based conditional pull.
type SnapshotHandler struct {
	store  artifact.Store
	signer *artifact.Signer
}

// NewSnapshotHandler creates a new snapshot handler.
func NewSnapshotHandler(store artifact.Store, signer *artifact.Signer) *SnapshotHandler {
	return &SnapshotHandler{store: store, signer: signer}
}

// HandleSnapshot responds to GET /v1/snapshot.
// Supports ETag-based conditional pull via If-None-Match header.
// Returns 304 Not Modified if the snapshot version hasn't changed.
func (h *SnapshotHandler) HandleSnapshot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspace")
	if workspaceID == "" {
		workspaceID = "default"
	}

	// Get current snapshot version
	currentVersion, err := h.store.CurrentSnapshotVersion(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("get snapshot version: %v", err),
		})
		return
	}

	versionStr := strconv.Itoa(currentVersion)

	// ETag check: if client's version matches, return 304
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch == versionStr || ifNoneMatch == `"`+versionStr+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Build snapshot
	snapshot, err := h.buildSnapshot(ctx, workspaceID, currentVersion)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("build snapshot: %v", err),
		})
		return
	}

	// Sign the snapshot by computing its hash (simplified for v0.2)
	snapData, _ := json.Marshal(snapshot)
	snapHash := sha256.Sum256(snapData)
	snapshot.Signature = hex.EncodeToString(snapHash[:])

	// Set ETag header
	w.Header().Set("ETag", versionStr)
	w.Header().Set("Cache-Control", "no-cache")
	writeJSON(w, http.StatusOK, snapshot)

	log.Printf("[control] Snapshot v%d served for workspace %q (%d deployments)",
		currentVersion, workspaceID, len(snapshot.Deployments))
}

func (h *SnapshotHandler) buildSnapshot(ctx context.Context, workspaceID string, version int) (*artifact.Snapshot, error) {
	// Get all deployments for this workspace
	deployments, err := h.store.ListDeployments(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	if deployments == nil {
		deployments = []*artifact.Deployment{}
	}

	snapshot := &artifact.Snapshot{
		Version:     version,
		IssuedAt:    time.Now().UTC(),
		Environment: "dev", // TODO: read from environment config
		Deployments: make([]artifact.Deployment, len(deployments)),
	}

	for i, d := range deployments {
		snapshot.Deployments[i] = *d
	}

	// Workspace Binding (plan fase B2): evaluate every registered
	// kind: Datastore service's access.filter against this workspace and
	// include only the matches in the snapshot — services that don't match
	// are invisible to the workspace (platform/06-datastore.md §4).
	datastores, err := h.store.ListDatastores(ctx)
	if err != nil {
		return nil, fmt.Errorf("list datastores: %w", err)
	}
	ws := datastore.WorkspaceInfo{ID: workspaceID, Environment: snapshot.Environment}
	for _, ds := range datastores {
		dsSpec, err := decodeDatastoreSpec(ds.Spec)
		if err != nil {
			return nil, fmt.Errorf("datastore %q: %w", ds.Name, err)
		}
		// Service-level environment/labels (registration metadata) must
		// match the snapshot environment first; then the spec's own
		// access.filter decides workspace visibility.
		if ds.Environment != "" && ds.Environment != snapshot.Environment {
			continue
		}
		var filter *spec.DatastoreAccessFilter
		if dsSpec.Access != nil {
			filter = dsSpec.Access.Filter
		}
		if !datastore.FilterMatch(filter, ws) {
			continue
		}
		binding := artifact.DatastoreBinding{
			Name: ds.Name,
			Spec: ds.Spec,
		}
		if dsSpec.Access != nil && dsSpec.Access.Permission != nil {
			permJSON, err := json.Marshal(dsSpec.Access.Permission)
			if err != nil {
				return nil, fmt.Errorf("datastore %q permission: %w", ds.Name, err)
			}
			binding.Permission = permJSON
		}
		snapshot.Datastores = append(snapshot.Datastores, binding)
	}

	return snapshot, nil
}

// decodeDatastoreSpec parses a registered DatastoreSpec from its stored JSON.
func decodeDatastoreSpec(raw json.RawMessage) (*spec.DatastoreSpec, error) {
	var ds spec.DatastoreSpec
	if err := json.Unmarshal(raw, &ds); err != nil {
		return nil, fmt.Errorf("decode spec: %w", err)
	}
	return &ds, nil
}

// HashForETag computes a content-based ETag for snapshot comparison.
func HashForETag(v any) string {
	data, _ := json.Marshal(v)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
