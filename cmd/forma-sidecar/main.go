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
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
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
		force         = flag.Bool("force", false, "Kill previous forma-sidecar on the same ports instead of failing (errors if ports are held by a different program)")
		webDir        = flag.String("web-dir", "", "Built SPA directory (e.g. web/dist). When set, serves SPA at /{ws}/_admin and /{ws}/app")
		devUI         = flag.Bool("dev-ui", false, "Development UI: start Vite dev server as a child process (implies --dev, ignores --web-dir)")
	)
	flag.Parse()

	if *devUI {
		*devMode = true
		*webDir = ""
	}

	if *controlURL == "" && *specPath == "" {
		fmt.Fprintln(os.Stderr, "forma-sidecar: need --control-cluster-url (artifact pull) or --spec (local manifests)")
		os.Exit(1)
	}

	// ── Port conflict resolution (--force) ──
	// If --force is set, check all ports and kill previous forma-sidecar
	// instances holding them. Error out if a different program owns the port.
	if *force {
		addrs := []string{*addr, *listen, *appEndpoint}
		for _, a := range addrs {
			if err := ensurePort(a, "forma-sidecar"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
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
		WebDir:               *webDir,
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

	// ── Dev UI: start Vite dev server as a child process ──
	// When --dev-ui is set, spawn npm run dev in the web/ directory so the
	// developer gets HMR without a second terminal. The Vite proxy in
	// vite.config.ts routes API calls back to this sidecar.
	var viteProc *appProcess
	if *devUI {
		webDir, err := findWebDir()
		if err != nil {
			log.Fatalf("[forma-sidecar] %v", err)
		}
		viteProc, err = startVite(ctx, webDir)
		if err != nil {
			log.Fatalf("[forma-sidecar] vite: %v", err)
		}
		log.Printf("[forma-sidecar] Vite dev server started (http://localhost:5173)")
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
	if viteProc != nil {
		viteProc.Shutdown(5 * time.Second)
	}
}

// ─── Dev UI Helpers ───

// findWebDir locates the web/ directory relative to the sidecar binary.
func findWebDir() (string, error) {
	// Try several locations in order
	candidates := []string{"web", "../web", "./web"}
	for _, c := range candidates {
		info, err := os.Stat(filepath.Join(c, "package.json"))
		if err == nil && !info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	// Fall back to ./web and let the user get a clear error
	abs, _ := filepath.Abs("web")
	if _, err := os.Stat(filepath.Join(abs, "package.json")); err != nil {
		return "", fmt.Errorf("cannot find web/ directory (checked %v): %w", candidates, err)
	}
	return abs, nil
}

// startVite spawns npm run dev in the given directory as a managed child process.
func startVite(ctx context.Context, webDir string) (*appProcess, error) {
	cmd := exec.CommandContext(ctx, "npm", "run", "dev")
	cmd.Dir = webDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start npm run dev: %w", err)
	}
	return &appProcess{cmd: cmd}, nil
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

// ─── Port Conflict Resolution ───

// ensurePort checks whether the given address is free. If it is in use and
// --force was passed, it kills previous forma-sidecar instances holding it;
// if a different program holds the port, it returns a descriptive error.
func ensurePort(addr, ownProcessName string) error {
	// Extract port from various address formats:
	//   ":8080", "http://127.0.0.1:9090", "unix:///path.sock"
	port, err := extractPort(addr)
	if err != nil {
		// Unix sockets can't conflict across programs easily; skip check
		return nil
	}
	if port == 0 {
		return nil
	}

	// Probe the port — if listen succeeds, it's free (close immediately)
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		ln.Close()
		return nil // port is free
	}

	// Port is in use — find the owner
	pid, procName, err := findProcessOnPort(port)
	if err != nil {
		return fmt.Errorf("port %d is in use but cannot identify the owner: %w", port, err)
	}

	if procName == ownProcessName || procName == "exe" || strings.Contains(procName, ownProcessName) {
		fmt.Fprintf(os.Stderr, "port %d is held by a previous %s (PID %d) — killing it...\n", port, ownProcessName, pid)
		proc, err := os.FindProcess(pid)
		if err == nil {
			proc.Signal(syscall.SIGTERM)
			// Wait briefly for graceful shutdown
			time.Sleep(500 * time.Millisecond)
			// Force kill if still alive
			proc.Signal(syscall.SIGKILL)
		}
		return nil
	}

	// Different program — return a clear error
	return fmt.Errorf(
		"port %d is already in use by %q (PID %d). Use --force to kill a previous %s, or stop the other program manually",
		port, procName, pid, ownProcessName,
	)
}

// extractPort extracts the TCP port number from various address formats.
// Returns 0 for Unix sockets or invalid formats.
func extractPort(addr string) (int, error) {
	// Strip scheme prefix
	raw := addr
	if strings.Contains(raw, "://") {
		parts := strings.Split(raw, "://")
		if len(parts) < 2 {
			return 0, nil
		}
		scheme := parts[0]
		rest := parts[1]
		switch scheme {
		case "unix":
			return 0, nil // Unix socket — skip
		case "http", "https", "tcp":
			raw = rest
		default:
			return 0, nil
		}
	}

	// raw is now "host:port" or ":port"
	_, portStr, err := net.SplitHostPort(raw)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, err
	}
	return port, nil
}

// findProcessOnPort returns the PID and process name of the program listening
// on the given TCP port. Uses /proc filesystem on Linux.
func findProcessOnPort(port int) (int, string, error) {
	// Strategy: read /proc/<pid>/net/tcp to find the inode, then match
	// against /proc/<pid>/fd/ entries. Simpler: try lsof first, fall back
	// to /proc scanning.

	// Attempt lsof (most reliable)
	pid, name, err := findProcessByLsof(port)
	if err == nil {
		return pid, name, nil
	}

	// Fall back to /proc scanning
	return findProcessByProcFS(port)
}

func findProcessByLsof(port int) (int, string, error) {
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-sTCP:LISTEN", "-F", "pcn")
	out, err := cmd.Output()
	if err != nil {
		return 0, "", fmt.Errorf("lsof failed: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var pid int
	var name string
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'c':
			name = line[1:]
		case 'n':
			// File descriptor — skip
		}
	}
	if pid == 0 {
		return 0, "", fmt.Errorf("no PID found for port %d", port)
	}
	return pid, name, nil
}

func findProcessByProcFS(port int) (int, string, error) {
	// Read all /proc/*/net/tcp to find the inode matching our port
	portHex := strings.ToUpper(fmt.Sprintf("%04X", port))

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, "", fmt.Errorf("cannot read /proc: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		tcpFile := fmt.Sprintf("/proc/%d/net/tcp", pid)
		data, err := os.ReadFile(tcpFile)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			// Field 2 is "local_address:port" in hex
			localAddr := fields[1]
			parts := strings.Split(localAddr, ":")
			if len(parts) != 2 {
				continue
			}
			if parts[1] == portHex {
				// Found the PID — read process name
				comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
				if err != nil {
					return pid, "unknown", nil
				}
				return pid, strings.TrimSpace(string(comm)), nil
			}
		}
	}
	return 0, "", fmt.Errorf("no process found listening on port %d", port)
}
