package control

import (
	"net/http"
)

// PollHandler handles POST /v1/poll — a dev-only endpoint that a local
// Resource Plane can call to trigger an immediate snapshot pull.
//
// This is strictly a dev-mode optimization. In production, Resource Planes
// rely on their own pull cadence (5-minute intervals). The endpoint exists
// only because in dev, both processes run on the same machine and we can
// reduce latency from 10s to ~100ms without violating the pull-based model.
type PollHandler struct {
	// In a real implementation, this would notify a connected Resource Plane.
	// For v0.2, it simply returns the current snapshot version.
	snapshotHandler *SnapshotHandler
}

// NewPollHandler creates a new dev-mode poll handler.
func NewPollHandler(snapshotHandler *SnapshotHandler) *PollHandler {
	return &PollHandler{snapshotHandler: snapshotHandler}
}

// HandlePoll responds to POST /v1/poll by returning the current snapshot version.
// The Resource Plane, upon receiving this response, should immediately call
// GET /v1/snapshot with the returned version.
type PollResponse struct {
	Version       int    `json:"version"`
	ShouldRefresh bool   `json:"should_refresh"`
	Message       string `json:"message,omitempty"`
}

func (h *PollHandler) HandlePoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	currentVersion, err := h.snapshotHandler.store.CurrentSnapshotVersion(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, PollResponse{
		Version:       currentVersion,
		ShouldRefresh: true,
		Message:       "dev-mode poll trigger — call GET /v1/snapshot to refresh",
	})
}
