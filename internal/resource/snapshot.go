package resource

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/primadi/formspec/internal/artifact"
)

// SnapshotFetcher pulls desired-state snapshots from the Control Plane
// using ETag-based conditional HTTP requests.
type SnapshotFetcher struct {
	controlURL    string
	workspaceID   string
	httpClient    *http.Client
	localManifest *LocalManifestManager
}

// NewSnapshotFetcher creates a new snapshot fetcher.
func NewSnapshotFetcher(controlURL, workspaceID string, localManifest *LocalManifestManager) *SnapshotFetcher {
	return &SnapshotFetcher{
		controlURL:  controlURL,
		workspaceID: workspaceID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		localManifest: localManifest,
	}
}

// FetchResult holds the result of a snapshot fetch operation.
type FetchResult struct {
	Snapshot   *artifact.Snapshot
	Changed    bool // true if snapshot content changed
	NewVersion int  // new snapshot version
}

// Fetch pulls the latest snapshot from the Control Plane.
// Uses If-None-Match header for conditional requests.
// Returns (nil, false) if snapshot hasn't changed (304).
func (f *SnapshotFetcher) Fetch() (*FetchResult, error) {
	url := fmt.Sprintf("%s/v1/snapshot?workspace=%s", f.controlURL, f.workspaceID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set ETag for conditional pull
	currentVersion := f.localManifest.GetControlVersion()
	if currentVersion > 0 {
		req.Header.Set("If-None-Match", fmt.Sprintf("%d", currentVersion))
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	// 304 = no changes
	if resp.StatusCode == http.StatusNotModified {
		return &FetchResult{
			Snapshot:   nil,
			Changed:    false,
			NewVersion: currentVersion,
		}, nil
	}

	// Non-200 = error
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("snapshot fetch: HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse snapshot
	var snapshot artifact.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}

	log.Printf("[resource] Snapshot v%d received (%d deployments)",
		snapshot.Version, len(snapshot.Deployments))

	// Update local version
	if err := f.localManifest.SetControlVersion(snapshot.Version); err != nil {
		log.Printf("[resource] Warning: failed to update local version: %v", err)
	}

	return &FetchResult{
		Snapshot:   &snapshot,
		Changed:    true,
		NewVersion: snapshot.Version,
	}, nil
}
