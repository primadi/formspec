// forma-sidecar embeds the Forma Resource engine in one process (entity
// engine, permission enforcement, generated REST API — the same packages a
// native Go app compiles in) and bridges it to a non-Go app process
// (PHP/Python/Node/...) in the same pod over a local socket.
//
// End-user REST traffic enters the sidecar, not the app. Only actions with
// impl: {type: sidecar} call out to the app process; during such a call the
// app calls back into the sidecar for ctx.* primitives. See
// docs/runtimes/04-forma-sidecar.md for the protocol.
//
// Usage:
//
//	forma-sidecar \
//	  --listen unix:///var/run/forma/sidecar.sock \
//	  --app-endpoint unix:///var/run/forma/app.sock \
//	  --control-cluster-url https://control-cluster.svc \
//	  --workspace-id bank-mandiri-prod \
//	  --runtime php:8.3 \
//	  --invoke-timeout 30s
//
// --runtime selects how the app process starts (docs/runtimes/04-forma-sidecar.md §5):
//
//   - "local" (default) — separate-container mode: the app runs in its own
//     process (a sibling K8s container, or started manually in dev) and
//     only needs to reach --app-endpoint on its own. Use this for any
//     language, whether or not a lib-forma-* SDK exists for it.
//   - "php" | "python" | "node" — child-process mode: forma-sidecar execs
//     the app itself (via the matching lib-forma-* SDK's conventions) from
//     --app-dir, entrypoint --app-entrypoint (default app.php/app.py/app.js).
//     Only worthwhile for lightweight runtimes where a second container's
//     overhead isn't worth it; requires unix:// for both --listen and
//     --app-endpoint.
//
// For local development without a Control Plane, pass --spec ./spec to load
// manifests straight from disk instead of pulling artifacts.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/primadi/forma/internal/artifact"
	"github.com/primadi/forma/internal/datastore"
	"github.com/primadi/forma/internal/resource"
	"github.com/primadi/forma/internal/sidecar"
	"github.com/primadi/forma/pkg/spec"
	forma "github.com/primadi/forma/resource"
)

func main() {
	var (
		listen        = flag.String("listen", "unix:///var/run/forma/sidecar.sock", "Local listener for app → sidecar ctx.* calls (unix:///path.sock or http://localhost:PORT)")
		appEndpoint   = flag.String("app-endpoint", "unix:///var/run/forma/app.sock", "App process lib-forma listener for sidecar → app invokes (unix:///path.sock or http://localhost:PORT)")
		controlURL    = flag.String("control-cluster-url", "", "Cluster Control URL to pull artifacts from (empty: load manifests from --spec)")
		workspaceID   = flag.String("workspace-id", "default", "Workspace ID for snapshot fetching")
		runtimeName   = flag.String("runtime", "local", `App runtime: "local" (default, app runs externally) or "php"/"python"/"node" (sidecar execs the app itself); a version suffix like "php:8.3" is accepted and informational only`)
		appDirFlag    = flag.String("app-dir", "", "App source directory for child-process --runtime modes (default: {state-dir}/app; ignored for --runtime local)")
		appEntrypoint = flag.String("app-entrypoint", "", "Entrypoint filename inside --app-dir (default: app.php / app.py / app.js per --runtime)")
		invokeTimeout = flag.Duration("invoke-timeout", 30*time.Second, "Timeout for a single app handler invocation")
		specPath      = flag.String("spec", "", "Manifest directory for local development (ignored when --control-cluster-url is set)")
		dsn           = flag.String("dsn", "sqlite:.forma/data.db", "Database DSN")
		addr          = flag.String("addr", ":8080", "REST API listen address (end-user traffic)")
		stateDir      = flag.String("state-dir", ".forma", "Local state directory (artifact manifest, evidence buffer, extracted specs)")
		devMode       = flag.Bool("dev", false, "Development mode (dev auth, unsigned artifacts, fast poll)")
	)
	flag.Parse()

	if *controlURL == "" && *specPath == "" {
		fmt.Fprintln(os.Stderr, "forma-sidecar: need --control-cluster-url (artifact pull) or --spec (local manifests)")
		os.Exit(1)
	}

	appDir := *appDirFlag
	if appDir == "" {
		appDir = filepath.Join(*stateDir, "app")
	}
	if !isLocalRuntime(*runtimeName) {
		log.Printf("[forma-sidecar] child-process mode: runtime=%s app-dir=%s", *runtimeName, appDir)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── Startup: resolve where manifests come from ──
	manifestDir := *specPath
	var deployer *resource.Deployer
	if *controlURL != "" {
		var err error
		manifestDir, deployer, err = pullArtifacts(ctx, *controlURL, *workspaceID, *stateDir, appDir, *devMode)
		if err != nil {
			log.Fatalf("[forma-sidecar] artifact pull: %v", err)
		}
		log.Printf("[forma-sidecar] artifacts extracted to %s", manifestDir)
	}

	// ── Boot the embedded Forma Resource engine ──
	app, err := forma.New(forma.Config{
		SpecPath:             manifestDir,
		DSN:                  *dsn,
		Addr:                 *addr,
		ProdMode:             !*devMode && *controlURL != "",
		SidecarEndpoint:      *appEndpoint,
		SidecarInvokeTimeout: *invokeTimeout,
		TenantID:             *workspaceID,
	})
	if err != nil {
		log.Fatalf("[forma-sidecar] engine boot: %v", err)
	}
	log.Printf("[forma-sidecar] engine loaded: %d routes", app.RouteCount())

	// ── App → Sidecar direction: ctx.* proxy + health aggregation ──
	// The primitive resolver is the same registry contract Starlark uses
	// (internal/starlark CtxAPI.SetDatastoreResolver). Named datastores are
	// registered from the Control snapshot — until that wiring lands
	// (docs/runtimes/02-forma-resource.md §7), resolution fails per-call.
	dsRegistry := datastore.NewRegistry()
	for _, driver := range []spec.DatastoreDriver{
		spec.DatastoreDriverSQLite, spec.DatastoreDriverPostgres,
		spec.DatastoreDriverValkey, spec.DatastoreDriverRedis,
		spec.DatastoreDriverS3, spec.DatastoreDriverMinio,
		spec.DatastoreDriverNATS, spec.DatastoreDriverMemory, spec.DatastoreDriverFS,
	} {
		if f, err := datastore.NewFactory(driver); err == nil {
			dsRegistry.RegisterFactory(driver, f)
		}
	}
	resolver := func(primitiveType, name string) (any, error) {
		return dsRegistry.Resolve(spec.PrimitiveType(primitiveType), name)
	}

	monitor, err := sidecar.NewAppMonitor(*appEndpoint, 10*time.Second, 3)
	if err != nil {
		log.Fatalf("[forma-sidecar] app endpoint: %v", err)
	}
	go monitor.Run(ctx)

	socketSrv := sidecar.NewServer(*listen, sidecar.NewCtxHandler(resolver), monitor, nil)
	if err := socketSrv.Listen(); err != nil {
		log.Fatalf("[forma-sidecar] %v", err)
	}
	log.Printf("[forma-sidecar] ctx listener on %s", *listen)

	// ── Child-process mode: exec the app now that the ctx listener is up ──
	// Separate-container mode (--runtime local, the default) skips this —
	// the app is started by something else (a sibling K8s container, or
	// manually in dev) and only needs to reach --app-endpoint on its own.
	var appProc *appProcess
	if !isLocalRuntime(*runtimeName) {
		var err error
		appProc, err = startAppProcess(ctx, *runtimeName, appDir, *appEntrypoint, *appEndpoint, *listen)
		if err != nil {
			log.Fatalf("[forma-sidecar] %v", err)
		}
	}

	// ── Background convergence loop (control mode) ──
	if deployer != nil {
		go deployer.RunLoop(ctx)
	}

	// ── Serve ──
	restSrv := &http.Server{Addr: *addr, Handler: app.Handler()}
	errCh := make(chan error, 2)
	go func() { errCh <- socketSrv.Serve() }()
	go func() {
		log.Printf("[forma-sidecar] REST API on %s", *addr)
		if err := restSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("[forma-sidecar] shutting down...")
	case err := <-errCh:
		log.Printf("[forma-sidecar] server error: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	restSrv.Shutdown(shutdownCtx)
	socketSrv.Shutdown(shutdownCtx)
	appProc.Shutdown(5 * time.Second) // no-op if nil (separate-container mode)
}

// pullArtifacts runs one convergence cycle against Cluster Control and
// materializes the artifact's files, so the engine (and, in child-process
// runtime mode, the app) can boot from disk exactly like local directories.
// The artifact envelope carries both kinds of file together
// (docs/runtimes/04-forma-sidecar.md §3.1: "YAML specs + source code app +
// runtime info") — YAML manifests go to stateDir/spec, everything else
// (app.php, vendor/, ...) goes to appDir. The returned deployer keeps
// polling in the background: later artifact changes are written to disk but
// require a process restart to take effect (route hot-rebuild is not
// implemented — the operator handles rolling restarts, see
// docs/architecture/06-k8s-operator.md §5).
func pullArtifacts(ctx context.Context, controlURL, workspaceID, stateDir, appDir string, devMode bool) (string, *resource.Deployer, error) {
	specDir := filepath.Join(stateDir, "spec")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", nil, err
	}

	localManifest, err := resource.NewLocalManifestManager(stateDir)
	if err != nil {
		return "", nil, fmt.Errorf("local manifest: %w", err)
	}
	instanceID := fmt.Sprintf("forma-sidecar-%d", os.Getpid())
	evidenceSender, err := resource.NewEvidenceSender(controlURL, instanceID, stateDir)
	if err != nil {
		return "", nil, fmt.Errorf("evidence sender: %w", err)
	}
	snapshotFetcher := resource.NewSnapshotFetcher(controlURL, workspaceID, localManifest)
	signer, err := artifact.NewDevSigner()
	if err != nil {
		return "", nil, fmt.Errorf("signer: %w", err)
	}
	artifactClient := resource.NewArtifactClient(controlURL, signer, devMode)

	pollInterval := 5 * time.Minute
	if devMode {
		pollInterval = 10 * time.Second
	}

	booted := false
	deployer := resource.NewDeployer(snapshotFetcher, artifactClient, localManifest, evidenceSender, pollInterval, devMode)
	deployer.OnDeploy = func(_ context.Context, files []artifact.FileManifest) error {
		for _, f := range files {
			dir := appDir
			if strings.HasSuffix(f.Path, ".yaml") || strings.HasSuffix(f.Path, ".yml") {
				dir = specDir
			}
			dest := filepath.Join(dir, filepath.Clean(f.Path))
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, f.Content, 0644); err != nil {
				return err
			}
		}
		if booted {
			log.Printf("[forma-sidecar] new artifact received (%d files) — restart required to apply", len(files))
		}
		return nil
	}

	if _, err := deployer.RunOnce(ctx); err != nil {
		return "", nil, fmt.Errorf("initial convergence: %w", err)
	}
	booted = true
	return specDir, deployer, nil
}
