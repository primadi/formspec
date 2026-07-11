// Package control implements the Control Plane HTTP/gRPC API handlers
// for artifact registration, snapshot serving, evidence collection, and
// dev-mode poll triggers.
package control

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/primadi/forma/internal/artifact"
	"github.com/primadi/forma/internal/manifest"
)

// RegisterHandler handles POST /v1/artifacts — receives YAML manifests,
// validates, hashes, signs, and stores them as artifacts.
type RegisterHandler struct {
	store  artifact.Store
	signer *artifact.Signer
}

// NewRegisterHandler creates a new artifact registration handler.
func NewRegisterHandler(store artifact.Store, signer *artifact.Signer) *RegisterHandler {
	return &RegisterHandler{store: store, signer: signer}
}

// RegisterRequest is the API request for artifact registration.
type RegisterRequest struct {
	App   string              `json:"app"`
	Files []RegisterFileEntry `json:"files"`
}

// RegisterFileEntry is a single file in the registration request.
type RegisterFileEntry struct {
	Path    string `json:"path"`
	Content []byte `json:"content"` // raw content
}

// RegisterResponse is the API response after successful registration.
type RegisterResponse struct {
	ArtifactID artifact.ArtifactID `json:"artifact_id"`
	App        string              `json:"app"`
	Version    int                 `json:"version"`
	SHA256     string              `json:"sha256"`
}

// HandleRegister processes a YAML artifact registration request.
// It accepts JSON payload with files content.
func (h *RegisterHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	req, err := parseRegisterRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	// Validate files
	if len(req.Files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "no files provided",
		})
		return
	}

	// Build file manifests with individual sha256
	fileManifests := make([]artifact.FileManifest, 0, len(req.Files))
	for _, f := range req.Files {
		fm := artifact.FileManifest{
			Path:    f.Path,
			SHA256:  artifact.FileSHA256(f.Content),
			Content: f.Content,
		}
		fileManifests = append(fileManifests, fm)
	}

	// Validate YAML files using the manifest loader
	loader := manifest.NewLoader("")
	for _, fm := range fileManifests {
		ext := strings.ToLower(filepath.Ext(fm.Path))
		if ext == ".yaml" || ext == ".yml" {
			raws, errs := loader.ParseBytes(fm.Content, fm.Path)
			if len(errs) > 0 {
				errMsg := ""
				for _, e := range errs {
					errMsg += e.Error() + "; "
				}
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("yaml validation failed for %s: %s", fm.Path, errMsg),
				})
				return
			}
			// Run per-manifest validation
			for _, raw := range raws {
				if err := loader.Validate(raw); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{
						"error": fmt.Sprintf("validation failed for %s: %v", raw.Source, err),
					})
					return
				}
			}
		}
	}

	// Compute aggregate sha256
	aggregateSHA256 := artifact.ComputeSHA256(fileManifests)

	// Get current version for this app
	latestArtifact, err := h.store.GetLatestArtifact(ctx, req.App)
	nextVersion := 1
	prevVersion := 0
	prevSHA256 := ""
	if err == nil && latestArtifact != nil {
		nextVersion = latestArtifact.Version + 1
		prevVersion = latestArtifact.Version
		prevSHA256 = latestArtifact.SHA256
	}

	now := time.Now().UTC()

	// Build envelope
	envelope := &artifact.ArtifactEnvelope{
		App:         req.App,
		Version:     nextVersion,
		SHA256:      aggregateSHA256,
		Files:       fileManifests,
		PrevVersion: prevVersion,
		PrevSHA256:  prevSHA256,
		CreatedAt:   now,
	}

	// Sign the envelope
	if err := h.signer.Sign(envelope); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("signing failed: %v", err),
		})
		return
	}

	// Create artifact record
	art := &artifact.Artifact{
		App:       req.App,
		Version:   nextVersion,
		SHA256:    aggregateSHA256,
		Envelope:  envelope,
		Status:    artifact.ArtifactStatusActive,
		CreatedAt: now,
	}

	if err := h.store.CreateArtifact(ctx, art); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("store artifact: %v", err),
		})
		return
	}

	// Update envelope with the assigned ID
	envelope.ArtifactID = art.ID

	// Mark previous artifact as superseded
	if prevVersion > 0 && latestArtifact != nil {
		latestArtifact.Status = artifact.ArtifactStatusSuperseded
	}

	// Bump snapshot version (triggers Resource Plane pull on next poll)
	_, _ = h.store.IncrementSnapshotVersion(ctx)

	log.Printf("[control] Registered artifact %s v%d (sha256=%s) for app %q",
		art.ID, nextVersion, aggregateSHA256[:12], req.App)

	writeJSON(w, http.StatusCreated, RegisterResponse{
		ArtifactID: art.ID,
		App:        req.App,
		Version:    nextVersion,
		SHA256:     aggregateSHA256,
	})
}

func parseRegisterRequest(r *http.Request) (*RegisterRequest, error) {
	ct := r.Header.Get("Content-Type")

	if strings.HasPrefix(ct, "application/json") {
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, fmt.Errorf("decode json: %w", err)
		}
		return &req, nil
	}

	if strings.HasPrefix(ct, "multipart/form-data") {
		return parseMultipartRequest(r)
	}

	return nil, fmt.Errorf("unsupported content type: %s (use application/json or multipart/form-data)", ct)
}

func parseMultipartRequest(r *http.Request) (*RegisterRequest, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, fmt.Errorf("parse multipart: %w", err)
	}

	var req RegisterRequest
	req.App = r.FormValue("app")

	for name, headers := range r.MultipartForm.File {
		for _, header := range headers {
			file, err := header.Open()
			if err != nil {
				return nil, fmt.Errorf("open file %s: %w", name, err)
			}

			content, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				return nil, fmt.Errorf("read file %s: %w", name, err)
			}

			req.Files = append(req.Files, RegisterFileEntry{
				Path:    name,
				Content: content,
			})
		}
	}

	return &req, nil
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
