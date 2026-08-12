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

// ArtifactClient fetches and verifies artifacts from the Control Plane.
type ArtifactClient struct {
	controlURL string
	httpClient *http.Client
	signer     *artifact.Signer // for verification (nil = skip in dev)
	devMode    bool
}

// NewArtifactClient creates a new artifact client.
func NewArtifactClient(controlURL string, signer *artifact.Signer, devMode bool) *ArtifactClient {
	return &ArtifactClient{
		controlURL: controlURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		signer:  signer,
		devMode: devMode,
	}
}

// FetchEnvelope downloads a signed artifact envelope from the Control Plane.
func (c *ArtifactClient) FetchEnvelope(artifactID artifact.ArtifactID) (*artifact.ArtifactEnvelope, error) {
	url := fmt.Sprintf("%s/v1/artifacts/%s", c.controlURL, string(artifactID))

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch artifact %s: %w", artifactID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch artifact %s: HTTP %d: %s", artifactID, resp.StatusCode, string(body))
	}

	var envelope artifact.ArtifactEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode artifact envelope: %w", err)
	}

	return &envelope, nil
}

// VerifyAndExtract validates the artifact envelope and extracts the file manifests.
// In dev mode, it skips signature verification but still validates hashes.
func (c *ArtifactClient) VerifyAndExtract(envelope *artifact.ArtifactEnvelope) ([]artifact.FileManifest, error) {
	// 1. Verify envelope integrity (file hashes + aggregate hash)
	if err := artifact.ValidateEnvelopeIntegrity(envelope); err != nil {
		return nil, fmt.Errorf("integrity check failed: %w", err)
	}

	log.Printf("[resource] Envelope integrity verified: %s v%d (sha256=%s)",
		envelope.App, envelope.Version, envelope.SHA256[:12])

	// 2. Verify signature (skip in dev mode with self-signed)
	if !c.devMode && c.signer != nil {
		// In production, verify against trusted public key
		// For now, we use the signer's public key
		if err := artifact.Verify(envelope, c.signer.PublicKey()); err != nil {
			return nil, fmt.Errorf("signature verification failed: %w", err)
		}
		log.Printf("[resource] Signature verified: key=%s", envelope.SigningKeyID)
	}

	return envelope.Files, nil
}

// ExtractYAMLFiles filters file manifests to only YAML files.
func ExtractYAMLFiles(files []artifact.FileManifest) []artifact.FileManifest {
	var yamls []artifact.FileManifest
	for _, f := range files {
		if len(f.Path) > 5 && (f.Path[len(f.Path)-5:] == ".yaml" || f.Path[len(f.Path)-4:] == ".yml") {
			yamls = append(yamls, f)
		}
	}
	return yamls
}

// ExtractScriptFiles filters to Starlark script files.
func ExtractScriptFiles(files []artifact.FileManifest) []artifact.FileManifest {
	var scripts []artifact.FileManifest
	for _, f := range files {
		if len(f.Path) > 5 && f.Path[len(f.Path)-5:] == ".star" {
			scripts = append(scripts, f)
		}
	}
	return scripts
}
