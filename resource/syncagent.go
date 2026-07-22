package forma

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/primadi/forma/internal/artifact"
	"github.com/primadi/forma/internal/entity"
	"github.com/primadi/forma/internal/manifest"
	"github.com/primadi/forma/internal/resource"
	db "github.com/primadi/forma/renderers/jsonbpersist"
)

// SyncAgentConfig configures a SyncAgent — the client side of the Forma
// plane protocol (see docs/architecture/03-deployment-flow.md and
// docs/runtimes/01-forma-control.md).
type SyncAgentConfig struct {
	DSN        string // Database DSN (default: "sqlite:.forma/data.db")
	ControlURL string // Cluster/Region Control URL (default: "http://localhost:8443")
	StateDir   string // Local state directory for manifest/evidence buffer (default: ".forma")
	Dev        bool   // Dev mode: 10s poll interval, unsigned artifact acceptance
	PollPort   int    // Dev-only /v1/poll listener port (default: 8081)
}

func (c *SyncAgentConfig) applyDefaults() {
	if c.DSN == "" {
		c.DSN = "sqlite:.forma/data.db"
	}
	if c.ControlURL == "" {
		c.ControlURL = "http://localhost:8443"
	}
	if c.StateDir == "" {
		c.StateDir = ".forma"
	}
	if c.PollPort == 0 {
		c.PollPort = 8081
	}
}

// SyncAgent pulls artifacts from a Forma Control Plane and keeps a local
// entity registry converged with the desired state (schema synced,
// manifests registered).
//
// SyncAgent does NOT serve an HTTP API from the entities it loads — that
// gap is tracked in docs/runtimes/02-forma-resource.md §7 (unifying this
// pull-based convergence loop with the REST API generator in forma.go is
// the top follow-up item for this package).
type SyncAgent struct {
	cfg      SyncAgentConfig
	reg      *entity.Registry
	deployer *resource.Deployer
	evidence *resource.EvidenceSender
}

// NewSyncAgent wires the plane-protocol client: local manifest state,
// evidence buffering, snapshot fetching, artifact fetch+verify, and the
// convergence deployer. It does not start polling — call Run.
func NewSyncAgent(cfg SyncAgentConfig) (*SyncAgent, error) {
	cfg.applyDefaults()

	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	database, err := db.Open(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}

	reg := entity.NewRegistry(database, driver, "") // empty spec path = no filesystem loading

	localManifest, err := resource.NewLocalManifestManager(cfg.StateDir)
	if err != nil {
		return nil, fmt.Errorf("create local manifest: %w", err)
	}

	instanceID := fmt.Sprintf("sync-agent-%d", os.Getpid())
	evidenceSender, err := resource.NewEvidenceSender(cfg.ControlURL, instanceID, cfg.StateDir)
	if err != nil {
		return nil, fmt.Errorf("create evidence sender: %w", err)
	}

	snapshotFetcher := resource.NewSnapshotFetcher(cfg.ControlURL, "default", localManifest)

	signer, err := artifact.NewDevSigner()
	if err != nil {
		return nil, fmt.Errorf("create signer: %w", err)
	}
	artifactClient := resource.NewArtifactClient(cfg.ControlURL, signer, cfg.Dev)

	pollInterval := 5 * time.Minute
	if cfg.Dev {
		pollInterval = 10 * time.Second
	}

	deployer := resource.NewDeployer(snapshotFetcher, artifactClient, localManifest, evidenceSender, pollInterval, cfg.Dev)
	deployer.OnDeploy = func(ctx context.Context, yamlFiles []artifact.FileManifest) error {
		return loadYAMLIntoRegistry(ctx, reg, yamlFiles)
	}

	return &SyncAgent{cfg: cfg, reg: reg, deployer: deployer, evidence: evidenceSender}, nil
}

// Registry returns the entity registry kept converged by this agent.
func (a *SyncAgent) Registry() *entity.Registry { return a.reg }

// Run starts the health ticker and convergence loop, and — in dev mode —
// the local /v1/poll listener used by `forma apply --watch` for fast
// refresh. It blocks until ctx is cancelled, then flushes buffered
// evidence before returning.
func (a *SyncAgent) Run(ctx context.Context) error {
	stopHealth := make(chan struct{})
	go a.evidence.HealthTicker(5*time.Minute, stopHealth)
	defer close(stopHealth)

	if a.cfg.Dev {
		go a.startDevPollListener(a.cfg.PollPort)
	}

	go a.deployer.RunLoop(ctx)

	<-ctx.Done()
	a.evidence.Flush()
	return nil
}

// ForcePoll triggers an immediate convergence cycle, bypassing the poll
// interval — used by the dev poll listener and available for callers that
// want to force a sync (e.g. after a known deploy).
func (a *SyncAgent) ForcePoll(ctx context.Context) {
	a.deployer.ForcePoll(ctx)
}

func (a *SyncAgent) startDevPollListener(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/poll", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		a.deployer.ForcePoll(context.Background())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","triggered":true}`)
	})

	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("forma: dev poll listener error: %v", err)
	}
}

// loadYAMLIntoRegistry parses artifact YAML files and registers Document/
// Entity kinds into reg, then syncs the schema. Other kinds (Service,
// Config, ...) are not yet consumed by any registry — see
// docs/runtimes/02-forma-resource.md §7.
func loadYAMLIntoRegistry(ctx context.Context, reg *entity.Registry, yamlFiles []artifact.FileManifest) error {
	loader := manifest.NewLoader("")

	for _, yf := range yamlFiles {
		raws, errs := loader.ParseBytes(yf.Content, yf.Path)
		if len(errs) > 0 {
			for _, e := range errs {
				log.Printf("forma: sync agent parse warning %s: %v", yf.Path, e)
			}
			continue
		}

		for _, raw := range raws {
			if raw.Kind != "Document" && raw.Kind != "Entity" {
				continue
			}
			entitySpec, err := manifest.RawSpecToEntitySpec(raw.Spec.(map[string]any))
			if err != nil {
				log.Printf("forma: sync agent skip %s: parse spec: %v", raw.Source, err)
				continue
			}
			if err := reg.RegisterArtifactManifest(raw, entitySpec); err != nil {
				log.Printf("forma: sync agent skip %s: register: %v", raw.Source, err)
				continue
			}
		}
	}

	applied, err := reg.SyncSchema(ctx)
	if err != nil {
		return fmt.Errorf("sync schema: %w", err)
	}
	if applied > 0 {
		log.Printf("forma: sync agent applied %d migration(s)", applied)
	}
	return nil
}
