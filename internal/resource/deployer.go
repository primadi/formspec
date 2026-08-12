package resource

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/primadi/formspec/internal/artifact"
	"github.com/primadi/formspec/internal/manifest"
)

// Deployer is the convergence engine that implements GitOps-style deployment.
// It polls the Control Plane for snapshot changes, compares sha256 hashes
// against the local deployment manifest, and deploys only changed artifacts.
type Deployer struct {
	snapshotFetcher *SnapshotFetcher
	artifactClient  *ArtifactClient
	localManifest   *LocalManifestManager
	evidenceSender  *EvidenceSender
	manifestLoader  *manifest.Loader
	pollInterval    time.Duration
	devMode         bool

	// OnDeploy is called when an artifact is loaded. The callback receives
	// the loaded YAML file manifests and should register them with the
	// entity registry and sync schemas.
	OnDeploy func(ctx context.Context, yamlFiles []artifact.FileManifest) error
}

// NewDeployer creates a new deployment convergence engine.
func NewDeployer(
	snapshotFetcher *SnapshotFetcher,
	artifactClient *ArtifactClient,
	localManifest *LocalManifestManager,
	evidenceSender *EvidenceSender,
	pollInterval time.Duration,
	devMode bool,
) *Deployer {
	return &Deployer{
		snapshotFetcher: snapshotFetcher,
		artifactClient:  artifactClient,
		localManifest:   localManifest,
		evidenceSender:  evidenceSender,
		manifestLoader:  manifest.NewLoader(""),
		pollInterval:    pollInterval,
		devMode:         devMode,
	}
}

// RunOnce performs a single deployment convergence cycle.
// Returns true if any changes were deployed.
func (d *Deployer) RunOnce(ctx context.Context) (bool, error) {
	// Step 1: Fetch snapshot
	result, err := d.snapshotFetcher.Fetch()
	if err != nil {
		return false, fmt.Errorf("fetch snapshot: %w", err)
	}

	// No changes
	if !result.Changed {
		return false, nil
	}

	snapshot := result.Snapshot
	if snapshot == nil {
		return false, nil
	}

	log.Printf("[resource] Convergence cycle: v%d with %d deployments",
		snapshot.Version, len(snapshot.Deployments))

	changesDeployed := false

	// Step 2: Iterate over desired deployments
	for _, desired := range snapshot.Deployments {
		deployed, err := d.convergeArtifact(ctx, &desired)
		if err != nil {
			log.Printf("[resource] Deployment failed for %s/%s: %v",
				desired.App, desired.ArtifactID, err)

			d.evidenceSender.SubmitDeployStatus(&artifact.DeployStatusPayload{
				ArtifactID: desired.ArtifactID,
				App:        desired.App,
				Version:    desired.Version,
				SHA256:     desired.SHA256,
				Phase:      artifact.DeployPhaseFailed,
				Error:      err.Error(),
			})
			continue
		}

		if deployed {
			changesDeployed = true
		}
	}

	// Step 3: Flush evidence
	d.evidenceSender.Flush()

	return changesDeployed, nil
}

// convergeArtifact handles a single artifact deployment.
// Returns true if the artifact was actually deployed (not just up_to_date).
func (d *Deployer) convergeArtifact(ctx context.Context, dep *artifact.Deployment) (bool, error) {
	key := dep.App + "/" + string(dep.ArtifactID)

	// Step 1: Hash comparison — skip if already deployed with same hash
	localHash := d.localManifest.GetArtifactHash(key)
	if localHash != "" && localHash == dep.SHA256 {
		log.Printf("[resource]  ✓ %s: hash match (sha256=%s), up to date",
			key, dep.SHA256[:12])

		d.evidenceSender.SubmitDeployStatus(&artifact.DeployStatusPayload{
			ArtifactID: dep.ArtifactID,
			App:        dep.App,
			Version:    dep.Version,
			SHA256:     dep.SHA256,
			Phase:      artifact.DeployPhaseUpToDate,
		})
		return false, nil
	}

	log.Printf("[resource]  → %s: deploying v%d (sha256=%s)",
		key, dep.Version, dep.SHA256[:12])

	// Step 2: Fetch artifact envelope
	d.evidenceSender.SubmitDeployStatus(&artifact.DeployStatusPayload{
		ArtifactID: dep.ArtifactID,
		App:        dep.App,
		Version:    dep.Version,
		SHA256:     dep.SHA256,
		Phase:      artifact.DeployPhaseFetched,
	})

	envelope, err := d.artifactClient.FetchEnvelope(dep.ArtifactID)
	if err != nil {
		return false, fmt.Errorf("fetch: %w", err)
	}

	// Step 3: Verify integrity
	d.evidenceSender.SubmitDeployStatus(&artifact.DeployStatusPayload{
		ArtifactID: dep.ArtifactID,
		App:        dep.App,
		Version:    dep.Version,
		SHA256:     dep.SHA256,
		Phase:      artifact.DeployPhaseVerified,
	})

	yamlFiles, err := d.artifactClient.VerifyAndExtract(envelope)
	if err != nil {
		return false, fmt.Errorf("verify: %w", err)
	}

	// Step 4: If OnDeploy callback is set, invoke it
	if d.OnDeploy != nil {
		if err := d.OnDeploy(ctx, yamlFiles); err != nil {
			return false, fmt.Errorf("deploy callback: %w", err)
		}
	}

	// Step 5: Update local manifest
	d.localManifest.SetArtifactState(key, artifact.LocalArtifactState{
		ArtifactID: dep.ArtifactID,
		App:        dep.App,
		Version:    dep.Version,
		SHA256:     dep.SHA256,
		LoadedAt:   time.Now().UTC(),
		Status:     "active",
	})

	// Step 6: Emit loaded evidence
	d.evidenceSender.SubmitDeployStatus(&artifact.DeployStatusPayload{
		ArtifactID: dep.ArtifactID,
		App:        dep.App,
		Version:    dep.Version,
		SHA256:     dep.SHA256,
		Phase:      artifact.DeployPhaseLoaded,
	})

	log.Printf("[resource]  ✓ %s: deployed v%d successfully", key, dep.Version)
	return true, nil
}

// RunLoop runs the convergence loop at the configured poll interval.
// Blocks until the context is cancelled.
func (d *Deployer) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	mode := "dev"
	if !d.devMode {
		mode = "prod"
	}
	log.Printf("[resource] Deployer started (%s mode, poll every %v)", mode, d.pollInterval)

	// Run initial cycle immediately
	if _, err := d.RunOnce(ctx); err != nil {
		log.Printf("[resource] Initial deploy: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if _, err := d.RunOnce(ctx); err != nil {
				log.Printf("[resource] Deploy cycle: %v", err)
			}
		case <-ctx.Done():
			log.Printf("[resource] Deployer stopping: %v", ctx.Err())
			return
		}
	}
}

// ForcePoll can be called by the dev-mode poll endpoint to trigger
// an immediate convergence cycle (used by POST /v1/poll).
func (d *Deployer) ForcePoll(ctx context.Context) {
	if _, err := d.RunOnce(ctx); err != nil {
		log.Printf("[resource] Force poll: %v", err)
	}
}
