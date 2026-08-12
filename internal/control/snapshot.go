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

	return snapshot, nil
}

// HashForETag computes a content-based ETag for snapshot comparison.
func HashForETag(v any) string {
	data, _ := json.Marshal(v)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
