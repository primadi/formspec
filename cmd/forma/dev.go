// Command `forma dev` — development server.
//
// Satu binary untuk semua persona:
//   - Persona A: `forma dev` → API + SPA embedded, tanpa npm
//   - Persona B: `forma dev --dev-ui` → + Vite HMR untuk edit frontend
//   - Non-Go: auto-detect runtime (PHP/Python/Node) dan spawn app process
//
// Logika di file ini adalah hasil migrasi dari cmd/forma-sidecar/main.go
// dengan perubahan:
//   - Default --listen dan --app-endpoint = "none" (single process)
//   - --dev / --dev-ui implied --force
//   - Config file auto-discover (forma-sidecar.yaml)
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
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/primadi/forma/internal/datastore"
	"github.com/primadi/forma/internal/sidecar"
	"github.com/primadi/forma/pkg/spec"
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
}

// ─── Flag defaults ───

const (
	defaultSpecPath    = "./spec"
	defaultDSN         = "sqlite:.forma/data.db"
	defaultAddr        = ":8080"
	defaultListen      = "none"
	defaultAppEndpoint = "none"
	defaultWorkspaceID = "default"
	defaultStateDir    = ".forma"
	defaultRuntime     = "auto"
)

// ─── Entry point: runDev ───

func runDev(args []string) {
	// ── 1. Parse flags with defaults ──
	cfg := parseDevFlags(args)

	// ── 2. Try config file (forma-sidecar.yaml / forma-sidecar.yml) ──
	cfg = mergeConfigFile(cfg)

	// ── 3. Re-parse flags to override config file (CLI wins) ──
	// We already have final values from flag.Parse; config file only fills
	// in values that were NOT set via CLI. This is handled in mergeConfigFile.

	// ── 4. Auto-detect runtime ──
	if cfg.Runtime == "auto" {
		cfg.Runtime = detectRuntime()
	}

	// ── 5. Implied --force for dev/dev-ui ──
	if cfg.DevMode || cfg.DevUI {
		cfg.Force = true
	}

	// ── 6. Resolve listen URL ──
	listenURL := resolveEndpoint(cfg.Listen, cfg.ListenURL,
		"unix:///var/run/forma/sidecar.sock",
		"http://127.0.0.1:9090")

	appEndpointURL := resolveEndpoint(cfg.AppEndpoint, cfg.AppEndpointURL,
		"unix:///var/run/forma/app.sock",
		"http://127.0.0.1:9091")

	// ── 7. Auto-create state directory ──
	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		log.Fatalf("[forma] create state dir %s: %v", cfg.StateDir, err)
	}

	// ── 8. Port conflict resolution ──
	if cfg.Force {
		addrs := []string{cfg.Addr}
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
		TenantID:             cfg.WorkspaceID,
	}

	// SPA serving priority: --web-dir > embedded FS
	if cfg.WebDir != "" {
		formaCfg.WebDir = cfg.WebDir
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
			return dsRegistry.Resolve(spec.PrimitiveType(primitiveType), name)
		}

		var err error
		monitor, err = sidecar.NewAppMonitor(appEndpointURL, 10*time.Second, 3)
		if err != nil {
			log.Fatalf("[forma] app monitor: %v", err)
		}
		go monitor.Run(ctx)

		socketSrv = sidecar.NewServer(listenURL, sidecar.NewCtxHandler(resolver), monitor, nil)
		if err := socketSrv.Listen(); err != nil {
			log.Fatalf("[forma] ctx listener: %v", err)
		}
		log.Printf("[forma] ctx listener on %s", listenURL)
	}

	// ── 12. App process (non-Go runtimes) ──
	var appProc *appProcess
	if !isLocalRuntime(cfg.Runtime) {
		var err error
		appProc, err = startAppProcess(ctx, cfg.Runtime, cfg.AppDir, cfg.AppEntrypoint, appEndpointURL, listenURL)
		if err != nil {
			log.Fatalf("[forma] app process: %v", err)
		}
	}

	// ── 13. Dev UI: Vite dev server ──
	var viteProc *appProcess
	if cfg.DevUI {
		webDir, err := findWebDir()
		if err != nil {
			log.Fatalf("[forma] %v", err)
		}
		viteProc, err = startVite(ctx, webDir)
		if err != nil {
			log.Fatalf("[forma] vite: %v", err)
		}
		log.Printf("[forma] Vite dev server started (http://localhost:5173)")
	}

	// ── 14. SPA info ──
	if cfg.WebDir != "" {
		log.Printf("[forma] SPA dari folder: %s", cfg.WebDir)
	} else if cfg.DevUI {
		log.Printf("[forma] Frontend: http://localhost:5173/default/_admin")
	} else {
		log.Printf("[forma] SPA embedded — buka http://localhost%s/default/_admin", cfg.Addr)
	}

	// ── 15. Serve ──
	restSrv := &http.Server{Addr: cfg.Addr, Handler: app.Handler()}
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
	runtime := fs.String("runtime", "", `App runtime: "auto" (default), "local", "php", "python", "node"`)
	stateDir := fs.String("state-dir", "", "State directory (default: .forma)")
	devMode := fs.Bool("dev", false, "Development mode (implied by --dev-ui)")
	devUI := fs.Bool("dev-ui", false, "Development UI: start Vite HMR (implies --dev)")
	force := fs.Bool("force", false, "Force kill previous instance on same ports")
	webDir := fs.String("web-dir", "", "Override SPA directory. Auto-detect if empty")
	invokeTimeout := fs.Duration("invoke-timeout", 30*time.Second, "Timeout for sidecar action invocation")
	appDir := fs.String("app-dir", "", "App source directory (child-process runtime)")
	appEntrypoint := fs.String("app-entrypoint", "", "Entrypoint filename (default: app.php / app.py / app.js)")
	controlURL := fs.String("control-cluster-url", "", "Control Plane URL (artifact pull mode)")

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
	}

	if cfg.AppDir == "" {
		cfg.AppDir = filepath.Join(cfg.StateDir, "app")
	}

	return cfg
}

// ─── Helpers ───

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
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
		if err == nil {
			proc.Signal(syscall.SIGTERM)
			time.Sleep(500 * time.Millisecond)
			proc.Signal(syscall.SIGKILL)
			time.Sleep(200 * time.Millisecond)
		}
		return nil
	}

	return fmt.Errorf("port %d is already in use by %q (PID %d). Stop it manually first", port, procName, pid)
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
