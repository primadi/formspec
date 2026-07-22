// Command `forma dev` — development server.
//
// Satu binary untuk semua persona:
//   - Persona A: `forma dev` → API + SPA embedded, tanpa npm
//   - Persona B: `forma dev --dev-ui` → + Vite HMR; SPA di :8080
//     di-proxy otomatis ke Vite dev server (:5173). User cukup akses
//     satu port (:8080) untuk semuanya.
//   - Non-Go: auto-detect runtime (PHP/Python/Node) dan spawn app process
//
// Logika di file ini adalah hasil migrasi dari cmd/forma-sidecar/main.go
// dengan perubahan:
//   - Default --listen = unix socket (sidecar selalu jalan)
//   - --dev / --dev-ui implied --force
//   - Config file auto-discover (forma-app.yaml / forma-sidecar.yaml)
//   - Runtime auto-detect dari project files
//   - Auto-create .forma/ directory
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/primadi/forma/internal/sidecar"
	"github.com/primadi/forma/pkg/spec"
	"github.com/primadi/forma/renderers/jsonbpersist/datastore"
	forma "github.com/primadi/forma/resource"
)

// ─── DevConfig ───

// DevConfig holds the merged configuration from defaults, config file, and CLI flags.
type DevConfig struct {
	SpecPath       string
	DSN            string
	Addr           string
	Listen         string // "none" | "local_http" | "unix_socket" | custom URL
	AppEndpoint    string // "none" | "local_http" | "unix_socket" | custom URL
	ListenURL      string // override resolved listen URL
	AppEndpointURL string // override resolved app endpoint URL
	WorkspaceID    string
	Runtime        string
	StateDir       string
	DevMode        bool
	DevUI          bool
	Force          bool
	WebDir         string
	InvokeTimeout  time.Duration
	AppDir         string
	AppEntrypoint  string
	ControlURL     string
	ThemeDirs      []string // additional directories containing theme manifests
}

// ─── Flag defaults ───

const (
	defaultSpecPath    = "./spec"
	defaultDSN         = "sqlite:.forma/data.db"
	defaultAddr        = ":8080"
	defaultListen      = "unix:///tmp/forma/sidecar.sock"
	defaultAppEndpoint = "none"
	defaultWorkspaceID = "default"
	defaultStateDir    = ".forma"
	defaultRuntime     = "auto"

	pidFileName = "dev.pid"
)

// ─── Entry point: runDev ───

func runDev(args []string) {
	// ── 0. Kill previous instance via PID file ──
	autoKillPrevious()

	// ── 0b. Optional positional directory argument ──
	// `forma dev .` or `forma dev /path/to/project`
	// Changes working directory before flag/config discovery.
	args = chdirIfPositionalArg(args)

	// ── 1. Parse flags with defaults ──
	cfg := parseDevFlags(args)

	// ── 2. Try config file (forma-app.yaml / forma-sidecar.yaml) ──
	cfg = mergeConfigFile(cfg)

	// ── 3. Apply defaults for values not set by CLI or config file ──
	if cfg.AppDir == "" {
		cfg.AppDir = filepath.Join(cfg.StateDir, "app")
	}

	// ── 4. Auto-detect runtime (scoped to app directory) ──
	if cfg.Runtime == "auto" {
		cfg.Runtime = detectRuntime(cfg.AppDir)
	}

	// ── 5. Implied --force for dev/dev-ui ──
	if cfg.DevMode || cfg.DevUI {
		cfg.Force = true
	}

	// ── 6. Resolve listen URL ──
	listenURL := resolveEndpoint(cfg.Listen, cfg.ListenURL,
		"unix:///tmp/forma/sidecar.sock",
		"http://127.0.0.1:9090")

	appEndpointURL := resolveEndpoint(cfg.AppEndpoint, cfg.AppEndpointURL,
		"unix:///tmp/forma/app.sock",
		"http://127.0.0.1:9091")

	// ── 7. Auto-create state directory ──
	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		log.Fatalf("[forma] create state dir %s: %v", cfg.StateDir, err)
	}

	// ── 7b. Write PID file for auto-kill on next run ──
	writePIDFile()

	// ── 8. Port conflict resolution ──
	// Main REST API port: always resolve (auto-kill previous forma instance)
	if err := ensurePort(cfg.Addr, "forma"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Extra listen/app ports: only resolve with --force
	if cfg.Force {
		addrs := []string{}
		if listenURL != "" {
			addrs = append(addrs, listenURL)
		}
		if appEndpointURL != "" {
			addrs = append(addrs, appEndpointURL)
		}
		for _, a := range addrs {
			if err := ensurePort(a, "forma"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}

	// ── 9. Validate runtime vs listen mode ──
	if !isLocalRuntime(cfg.Runtime) && listenURL == "" {
		log.Fatalf("[forma] cannot use --runtime %q with --listen none (app process needs ctx listener)", cfg.Runtime)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── 10. Boot Forma engine ──
	log.Printf("[forma] starting — spec=%s dsn=%s addr=%s", cfg.SpecPath, cfg.DSN, cfg.Addr)

	formaCfg := forma.Config{
		SpecPath:             cfg.SpecPath,
		DSN:                  cfg.DSN,
		Addr:                 cfg.Addr,
		ProdMode:             !cfg.DevMode && cfg.ControlURL != "",
		SidecarEndpoint:      appEndpointURL,
		SidecarInvokeTimeout: cfg.InvokeTimeout,
		WorkspaceID:          cfg.WorkspaceID,
		ThemeDirs:            cfg.ThemeDirs,
	}

	// SPA serving priority: --web-dir > auto-detect renderers/web/dist/ > embedded FS
	// When --dev-ui is active the backend does NOT serve the SPA directly;
	// instead SPA requests are reverse-proxied to the Vite dev server (see §13b).
	if cfg.DevUI {
		log.Printf("[forma] dev UI mode — SPA will be proxied to Vite dev server")
	} else if cfg.WebDir != "" {
		formaCfg.WebDir = cfg.WebDir
	} else if found := findWebDist(); found != "" {
		// Auto-detect renderers/web/dist/ — picks up npm run rebuilds immediately
		cfg.WebDir = found
		formaCfg.WebDir = cfg.WebDir
		log.Printf("[forma] SPA from folder: %s", cfg.WebDir)
	} else {
		// embed.FS stores files with their relative path (dist/index.html).
		// Use fs.Sub to strip the dist/ prefix so the FS is rooted at dist/.
		subFS, err := fs.Sub(spaFS, "dist")
		if err == nil {
			formaCfg.WebFS = subFS
		} else {
			log.Printf("[forma] warning: cannot init SPA from embed: %v", err)
		}
	}

	app, err := forma.New(formaCfg)
	if err != nil {
		log.Fatalf("[forma] engine boot: %v", err)
	}
	log.Printf("[forma] engine loaded: %d routes", app.RouteCount())

	// ── 11. Ctx listener (opsional) ──
	var socketSrv *sidecar.Server
	var monitor *sidecar.AppMonitor
	if listenURL != "" {
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
			if primitiveType == "entity" {
				// name format: "module/entity" — split and resolve via entity store
				parts := strings.SplitN(name, "/", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid entity reference %q (want module/entity)", name)
				}
				module, entityName := parts[0], parts[1]
				return app.GetEntityStore(module, entityName)
			}
			return dsRegistry.Resolve(spec.PrimitiveType(primitiveType), name)
		}

		var err error
		monitor, err = sidecar.NewAppMonitor(appEndpointURL, 10*time.Second, 3)
		if err != nil {
			log.Fatalf("[forma] app monitor: %v", err)
		}
		go monitor.Run(ctx)

		socketSrv = sidecar.NewServer(listenURL, sidecar.NewCtxHandler(resolver, "demo"), monitor, nil)
		if err := socketSrv.Listen(); err != nil {
			log.Fatalf("[forma] ctx listener: %v", err)
		}
		log.Printf("[forma] ctx listener on %s", listenURL)
	}

	// ── 12. App process (non-Go runtimes) ──
	var appProc *appProcess
	if !isLocalRuntime(cfg.Runtime) {
		var err error
		appProc, err = startAppProcess(ctx, cfg.Runtime, cfg.AppDir, cfg.AppEntrypoint, appEndpointURL, listenURL, cfg.DevMode)
		if err != nil {
			log.Fatalf("[forma] app process: %v", err)
		}
	}

	// ── 13. Dev UI: Vite dev server ──
	var viteProc *appProcess
	var vitePort string
	var viteProxyURL string
	if cfg.DevUI {
		webDir, err := findWebDir()
		if err != nil {
			log.Fatalf("[forma] %v", err)
		}
		viteProc, vitePort, err = startVite(ctx, webDir)
		if err != nil {
			log.Fatalf("[forma] vite: %v", err)
		}
		viteProxyURL = "http://localhost:" + vitePort
		log.Printf("[forma] Vite dev server started (%s)", viteProxyURL)
	}

	// ── 13b. Wrap handler dengan Vite proxy ──
	// SPA routes di :8080 di-proxy ke Vite dev server, sehingga user
	// cukup akses satu port (:8080) untuk API + frontend dengan HMR.
	handler := app.Handler()
	if viteProxyURL != "" {
		log.Printf("[forma] SPA routes at %s proxied to Vite (%s)", cfg.Addr, viteProxyURL)
		handler = viteSPAProxy(handler, viteProxyURL, cfg.WorkspaceID)
	}

	// ── 14. SPA info ──
	if cfg.DevUI {
		log.Printf("[forma] Frontend: http://localhost%s/default/_admin (proxied to Vite HMR)", cfg.Addr)
		log.Printf("[forma]   Vite langsung: http://localhost:%s/default/_admin", vitePort)
	} else if cfg.WebDir == "" {
		log.Printf("[forma] SPA embedded — buka http://localhost%s/default/_admin", cfg.Addr)
	}

	// ── 15. Serve ──
	restSrv := &http.Server{Addr: cfg.Addr, Handler: handler}
	errCh := make(chan error, 2)
	if socketSrv != nil {
		go func() { errCh <- socketSrv.Serve() }()
	}
	go func() {
		log.Printf("[forma] REST API on %s", cfg.Addr)
		if err := restSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("[forma] shutting down...")
	case err := <-errCh:
		log.Printf("[forma] server error: %v", err)
	}

	// Cleanup PID file
	cleanupPIDFile()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	restSrv.Shutdown(shutdownCtx)
	if socketSrv != nil {
		socketSrv.Shutdown(shutdownCtx)
	}
	if appProc != nil {
		appProc.Shutdown(5 * time.Second)
	}
	if viteProc != nil {
		viteProc.Shutdown(5 * time.Second)
	}
}

// ─── Flag Parsing ───

// ─── PID File Helpers ───

// pidFilePath returns the full path to the PID file in the state directory.
func pidFilePath() string {
	return filepath.Join(".forma", pidFileName)
}

// autoKillPrevious reads the PID file from .forma/dev.pid, kills the process
// if it still exists and is a forma dev instance, then removes the stale file.
func autoKillPrevious() {
	pidPath := pidFilePath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("[forma] warning: cannot read PID file %s: %v", pidPath, err)
		return
	}

	oldPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		log.Printf("[forma] warning: invalid PID in %s, removing...", pidPath)
		os.Remove(pidPath)
		return
	}

	proc, err := os.FindProcess(oldPID)
	if err != nil {
		// Process not found, clean up stale PID file
		os.Remove(pidPath)
		return
	}

	// Send SIGTERM; if it fails, process is already dead
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		os.Remove(pidPath)
		return
	}

	log.Printf("[forma] killing previous instance (PID %d)...", oldPID)

	// Give it a real chance to run its own graceful shutdown (which stops
	// its app/Vite children cleanly) instead of racing it with a fixed
	// short sleep — that previously SIGKILLed it before it got around to
	// signaling its children, leaving them orphaned in their own process
	// group (see ensurePort for the same fix).
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && processAlive(oldPID) {
		time.Sleep(150 * time.Millisecond)
	}

	if processAlive(oldPID) {
		log.Printf("[forma] previous instance (PID %d) did not exit gracefully — forcing", oldPID)
		killDescendants(oldPID)
		proc.Signal(syscall.SIGKILL)
		time.Sleep(200 * time.Millisecond)
	}

	os.Remove(pidPath)
}

// processAlive reports whether pid still exists, via a zero-signal probe.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// writePIDFile writes the current PID to the PID file.
func writePIDFile() {
	pidPath := pidFilePath()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		log.Printf("[forma] warning: cannot create state dir for PID file: %v", err)
		return
	}
	if err := os.WriteFile(pidPath, fmt.Appendf(nil, "%d\n", os.Getpid()), 0644); err != nil {
		log.Printf("[forma] warning: cannot write PID file: %v", err)
	}
}

// cleanupPIDFile removes the PID file.
func cleanupPIDFile() {
	if err := os.Remove(pidFilePath()); err != nil && !os.IsNotExist(err) {
		log.Printf("[forma] warning: cannot remove PID file: %v", err)
	}
}

// chdirIfPositionalArg checks if the first arg is a directory (not a flag).
// If so, it changes to that directory and returns the remaining args.
func chdirIfPositionalArg(args []string) []string {
	if len(args) == 0 || args[0] == "" || args[0][0] == '-' {
		return args
	}
	dir := args[0]
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return args
	}
	if err := os.Chdir(dir); err != nil {
		log.Fatalf("[forma] cannot chdir to %s: %v", dir, err)
	}
	log.Printf("[forma] working directory: %s", dir)
	return args[1:]
}

// findWebDist walks up from CWD looking for renderers/web/dist/ directory.
// Returns the absolute path if found, empty string otherwise.
func findWebDist() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, "renderers", "web", "dist")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}
	return ""
}

func parseDevFlags(args []string) DevConfig {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)

	specPath := fs.String("spec", "", "Path to spec directory (default: ./spec)")
	dsn := fs.String("dsn", "", "Database DSN (default: sqlite:.forma/data.db)")
	addr := fs.String("addr", "", "REST API listen address (default: :8080)")
	listen := fs.String("listen", "", `Ctx listener mode: "none", "local_http", "unix_socket" (default: none)`)
	appEndpoint := fs.String("app-endpoint", "", `App endpoint mode: "none", "local_http", "unix_socket" (default: none)`)
	listenURL := fs.String("listen-url", "", "Custom listen URL (override mode auto-resolve)")
	appEndpointURL := fs.String("app-endpoint-url", "", "Custom app endpoint URL (override mode auto-resolve)")
	workspaceID := fs.String("workspace-id", "", "Workspace ID (default: default)")
	runtime := fs.String("runtime", "", `App runtime: "auto" (default), "local", "php", "python", "ruby", "java", "dotnet", "go", "rust", "node"`)
	stateDir := fs.String("state-dir", "", "State directory (default: .forma)")
	devMode := fs.Bool("dev", false, "Development mode (implied by --dev-ui)")
	devUI := fs.Bool("dev-ui", false, "Development UI: start Vite HMR (implies --dev)")
	force := fs.Bool("force", false, "Force kill previous instance on same ports")
	webDir := fs.String("web-dir", "", "Override SPA directory. Auto-detect if empty")
	invokeTimeout := fs.Duration("invoke-timeout", 30*time.Second, "Timeout for sidecar action invocation")
	appDir := fs.String("app-dir", "", "App source directory (child-process runtime)")
	appEntrypoint := fs.String("app-entrypoint", "", "Entrypoint filename (default: app.php / app.py / app.js)")
	controlURL := fs.String("control-cluster-url", "", "Control Plane URL (artifact pull mode)")
	themeDirs := fs.String("theme-dir", "", "Additional theme directory (repeatable, comma-separated)")

	fs.Parse(args)

	// Apply defaults for empty flags
	cfg := DevConfig{
		SpecPath:       orDefault(*specPath, defaultSpecPath),
		DSN:            orDefault(*dsn, defaultDSN),
		Addr:           orDefault(*addr, defaultAddr),
		Listen:         orDefault(*listen, defaultListen),
		AppEndpoint:    orDefault(*appEndpoint, defaultAppEndpoint),
		ListenURL:      *listenURL,
		AppEndpointURL: *appEndpointURL,
		WorkspaceID:    orDefault(*workspaceID, defaultWorkspaceID),
		Runtime:        orDefault(*runtime, defaultRuntime),
		StateDir:       orDefault(*stateDir, defaultStateDir),
		DevMode:        *devMode || *devUI,
		DevUI:          *devUI,
		Force:          *force,
		WebDir:         *webDir,
		InvokeTimeout:  *invokeTimeout,
		AppDir:         *appDir,
		AppEntrypoint:  *appEntrypoint,
		ControlURL:     *controlURL,
		ThemeDirs:      splitAndClean(*themeDirs),
	}

	// AppDir default is applied AFTER config file merge (in runDev),
	// so the config file can override it. See mergeConfigFile's empty-string check.
	return cfg
}

// ─── Helpers ───

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

// splitAndClean splits a comma-separated string and trims whitespace.
// Returns nil for empty input.
func splitAndClean(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// resolveEndpoint converts a mode string + custom URL into a listenable URL.
//   - "none" → "" (disabled)
//   - "local_http" → httpDefault
//   - "unix_socket" → unixDefault
//   - Any other string is treated as a raw URL (backward compat)
func resolveEndpoint(mode, customURL, unixDefault, httpDefault string) string {
	if customURL != "" {
		return customURL
	}
	switch mode {
	case "none":
		return ""
	case "unix_socket":
		return unixDefault
	case "local_http":
		return httpDefault
	default:
		// Backward compat: if mode looks like a URL, use it directly
		if strings.HasPrefix(mode, "http://") || strings.HasPrefix(mode, "unix://") {
			return mode
		}
		return ""
	}
}

// isLocalRuntime reports whether runtime selects the default, no-child-process behavior.
func isLocalRuntime(runtime string) bool {
	return runtime == "" || runtime == "local" || runtime == "auto"
}

// ─── Port Conflict Resolution (from ex-sidecar) ───

// ensurePort checks whether the given address is free. If it is in use and
// --force was passed, it kills previous forma instances holding it;
// if a different program holds the port, it returns a descriptive error.
func ensurePort(addr, ownProcessName string) error {
	port, err := extractPort(addr)
	if err != nil {
		return nil // Unix sockets skip check
	}
	if port == 0 {
		return nil
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		ln.Close()
		return nil
	}

	pid, procName, err := findProcessOnPort(port)
	if err != nil {
		return fmt.Errorf("port %d is in use but cannot identify the owner: %w", port, err)
	}

	if procName == ownProcessName || procName == "exe" || strings.Contains(procName, ownProcessName) {
		fmt.Fprintf(os.Stderr, "port %d is held by a previous %s (PID %d) — killing it...\n", port, ownProcessName, pid)
		proc, err := os.FindProcess(pid)
		if err != nil {
			return nil
		}
		proc.Signal(syscall.SIGTERM)

		// Give the old instance a real chance to run its own graceful
		// shutdown (which stops its app/Vite children cleanly) instead of
		// racing it with a fixed short sleep — that previously SIGKILLed it
		// before it got around to signaling its children, leaving them
		// orphaned in their own process group.
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
				ln.Close()
				return nil
			}
			time.Sleep(150 * time.Millisecond)
		}

		// It's still holding the port after a generous wait — force it,
		// and sweep any children it leaked (e.g. an app/Vite process
		// detached in its own process group) so they don't accumulate
		// across restarts.
		fmt.Fprintf(os.Stderr, "port %d: previous instance (PID %d) did not exit gracefully — forcing\n", port, pid)
		killDescendants(pid)
		proc.Signal(syscall.SIGKILL)
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	return fmt.Errorf("port %d is already in use by %q (PID %d). Stop it manually first", port, procName, pid)
}

// killDescendants force-kills every descendant of pid (depth-first) so that
// children left behind in their own process group — e.g. an app/Vite
// process started with Setpgid, which doesn't die just because its parent
// does — don't survive a forced kill of that parent.
func killDescendants(pid int) {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		return
	}
	for _, field := range strings.Fields(string(out)) {
		childPid, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		killDescendants(childPid)
		if proc, err := os.FindProcess(childPid); err == nil {
			proc.Signal(syscall.SIGKILL)
		}
	}
}

// extractPort extracts the TCP port from an address string.
// Supports :8080, http://127.0.0.1:9090, etc. Unix sockets return 0.
func extractPort(addr string) (int, error) {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, "unix://") {
		return 0, nil
	}
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "tcp://")

	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			portStr = addr[1:]
		} else {
			return 0, fmt.Errorf("cannot parse address %q: %w", addr, err)
		}
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q in address %q", portStr, addr)
	}
	return port, nil
}

// findProcessOnPort returns the PID and process name for the process listening on the given port.
func findProcessOnPort(port int) (int, string, error) {
	// Try lsof first (Linux/macOS)
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	out, err := cmd.Output()
	if err == nil {
		pidStr := strings.TrimSpace(string(out))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			// Get process name
			proc, err := os.FindProcess(pid)
			if err == nil {
				// Try to read /proc/PID/comm on Linux
				comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
				if err == nil {
					return pid, strings.TrimSpace(string(comm)), nil
				}
				return pid, fmt.Sprintf("PID %d", pid), nil
			}
			_ = proc
		}
	}

	// Fallback: try fuser
	cmd = exec.Command("fuser", fmt.Sprintf("%d/tcp", port))
	out, err = cmd.Output()
	if err == nil {
		pidStr := strings.TrimSpace(string(out))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			return pid, fmt.Sprintf("PID %d", pid), nil
		}
	}

	return 0, "", fmt.Errorf("cannot determine owner of port %d", port)
}

// ─── Vite SPA Proxy ───

// viteSPAProxy wraps an http.Handler so that SPA-frontend requests are
// reverse-proxied to a Vite dev server instead of being served from the
// backend's built SPA (which may be stale or absent in --dev-ui mode).
//
// Strategy: proxy ALL request paths to Vite EXCEPT known API routes.
// This ensures Vite source files (/src/*, /@vite/*, /node_modules/.vite/*,
// etc.) are always served correctly without maintaining an allowlist.
//
// Non-proxied (backend-handled) paths:
//   - /{workspaceID}/api/* — REST API + meta + WebSocket
//   - /health              — health check
func viteSPAProxy(next http.Handler, viteTarget, workspaceID string) http.Handler {
	target, err := url.Parse(viteTarget)
	if err != nil {
		log.Fatalf("[forma] invalid Vite target URL %q: %v", viteTarget, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// API routes → passthrough ke backend
		if strings.HasPrefix(path, "/"+workspaceID+"/api/") ||
			path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Semua yang lain → Vite dev server
		proxy.ServeHTTP(w, r)
	})
}
