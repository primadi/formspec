package control

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/primadi/formspec/internal/artifact"
)

// EvidenceHandler handles POST /v1/evidence — receives signed evidence
// records from Resource Planes (deploy status, health, etc.).
type EvidenceHandler struct {
	store artifact.Store
}

// NewEvidenceHandler creates a new evidence handler.
func NewEvidenceHandler(store artifact.Store) *EvidenceHandler {
	return &EvidenceHandler{store: store}
}

// EvidenceBatch is a batch of evidence records submitted by a Resource Plane.
type EvidenceBatch struct {
	Records []EvidenceSubmitEntry `json:"records"`
}

// EvidenceSubmitEntry is a single evidence record in a batch submission.
type EvidenceSubmitEntry struct {
	Type       artifact.EvidenceType `json:"type"`
	InstanceID string                `json:"instance_id"`
	Sequence   int64                 `json:"sequence"`
	Payload    json.RawMessage       `json:"payload"`
	Signature  string                `json:"signature"`
}

// HandleEvidence receives and stores evidence from Resource Planes.
func (h *EvidenceHandler) HandleEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var batch EvidenceBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid evidence batch: %v", err),
		})
		return
	}

	if len(batch.Records) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "empty evidence batch",
		})
		return
	}

	accepted := 0
	for _, entry := range batch.Records {
		record := &artifact.EvidenceRecord{
			Type:       entry.Type,
			InstanceID: entry.InstanceID,
			Sequence:   entry.Sequence,
			Payload:    entry.Payload,
			Signature:  entry.Signature,
			ReceivedAt: time.Now().UTC(),
		}

		if err := h.store.AppendEvidence(ctx, record); err != nil {
			log.Printf("[control] Evidence append error (type=%s, instance=%s, seq=%d): %v",
				entry.Type, entry.InstanceID, entry.Sequence, err)
			continue
		}
		accepted++

		// Log deploy_status changes
		if entry.Type == artifact.EvidenceDeployStatus {
			var status artifact.DeployStatusPayload
			if err := json.Unmarshal(entry.Payload, &status); err == nil {
				log.Printf("[control] Deploy status: %s %s v%d phase=%s",
					status.App, status.ArtifactID, status.Version, status.Phase)
			}
		}
	}

	log.Printf("[control] Evidence accepted: %d/%d records", accepted, len(batch.Records))

	writeJSON(w, http.StatusOK, map[string]any{
		"accepted": accepted,
		"total":    len(batch.Records),
	})
}
