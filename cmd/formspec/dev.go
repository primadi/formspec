// Command `formspec dev` — development server.
//
// Satu binary untuk semua persona:
//   - Persona A: `formspec dev` → API + SPA embedded, tanpa npm
//   - Persona B: `formspec dev --dev-ui` → + Vite HMR; SPA di :8080
//     di-proxy otomatis ke Vite dev server (:5173). User cukup akses
//     satu port (:8080) untuk semuanya.
//   - Non-Go: auto-detect runtime (PHP/Python/Node) dan spawn app process
//
// Logika di file ini adalah hasil migrasi dari cmd/formspec-sidecar/main.go
// dengan perubahan:
//   - Default --listen = unix socket (sidecar selalu jalan)
//   - --dev / --dev-ui implied --force
//   - Config file auto-discover (formspec-app.yaml / formspec-sidecar.yaml)
//   - Runtime auto-detect dari project files
//   - Auto-create .formspec/ directory
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/primadi/formspec/internal/devserver"
	"github.com/primadi/formspec/internal/sidecar"
	"github.com/primadi/formspec/pkg/spec"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore"
	formspec "github.com/primadi/formspec/resource"
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
	DevAuth        bool   // Enable real JWT auth in dev mode (for testing authorization)
	JWTSecret      string // HMAC secret for JWT signing (persist across restarts)
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
	defaultDSN         = "sqlite:.formspec/data.db"
	defaultAddr        = ":8080"
	defaultListen      = "unix:///tmp/formspec/sidecar.sock"
	defaultAppEndpoint = "none"
	defaultWorkspaceID = "default"
	defaultStateDir    = ".formspec"
	defaultRuntime     = "auto"

	pidFileName = "dev.pid"
)

// ─── Entry point: runDev ───

func runDev(args []string) {
	// ── 0. Kill previous instance via PID file ──
	autoKillPrevious()

	// ── 0b. Optional positional directory argument ──
	// `formspec dev .` or `formspec dev /path/to/project`
	// Changes working directory before flag/config discovery.
	args = chdirIfPositionalArg(args)

	// ── 1. Parse flags with defaults ──
	cfg := parseDevFlags(args)

	// ── 2. Try config file (formspec-app.yaml / formspec-sidecar.yaml) ──
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
		"unix:///tmp/formspec/sidecar.sock",
		"http://127.0.0.1:9090")

	appEndpointURL := resolveEndpoint(cfg.AppEndpoint, cfg.AppEndpointURL,
		"unix:///tmp/formspec/app.sock",
		"http://127.0.0.1:9091")

	// ── 7. Auto-create state directory ──
	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		log.Fatalf("[formspec] create state dir %s: %v", cfg.StateDir, err)
	}

	// ── 7b. Write PID file for auto-kill on next run ──
	writePIDFile()

	// ── 8. Port conflict resolution ──
	// Main REST API port: always resolve (auto-kill previous formspec instance)
	if err := ensurePort(cfg.Addr, "formspec"); err != nil {
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
			if err := ensurePort(a, "formspec"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}

	// ── 9. Validate runtime vs listen mode ──
	if !isLocalRuntime(cfg.Runtime) && listenURL == "" {
		log.Fatalf("[formspec] cannot use --runtime %q with --listen none (app process needs ctx listener)", cfg.Runtime)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── 10. Boot FormSpec engine ──
	log.Printf("[formspec] starting — spec=%s dsn=%s addr=%s", cfg.SpecPath, cfg.DSN, cfg.Addr)

	formaCfg := formspec.Config{
		SpecPath:             cfg.SpecPath,
		DSN:                  cfg.DSN,
		Addr:                 cfg.Addr,
		ProdMode:             !cfg.DevMode && cfg.ControlURL != "",
		DevAuth:              cfg.DevAuth,
		JWTSecret:            cfg.JWTSecret,
		SidecarEndpoint:      appEndpointURL,
		SidecarInvokeTimeout: cfg.InvokeTimeout,
		WorkspaceID:          cfg.WorkspaceID,
		ThemeDirs:            cfg.ThemeDirs,
	}

	// SPA serving priority: --web-dir > auto-detect renderers/react-shadcn/dist/ > embedded FS
	// When --dev-ui is active the backend does NOT serve the SPA directly;
	// instead SPA requests are reverse-proxied to the Vite dev server (see §13b).
	if cfg.DevUI {
		log.Printf("[formspec] dev UI mode — SPA will be proxied to Vite dev server")
	} else if cfg.WebDir != "" {
		formaCfg.WebDir = cfg.WebDir
	} else if found := findWebDist(); found != "" {
		// Auto-detect renderers/react-shadcn/dist/ — picks up npm run rebuilds immediately
		cfg.WebDir = found
		formaCfg.WebDir = cfg.WebDir
		log.Printf("[formspec] SPA from folder: %s", cfg.WebDir)
	} else {
		// embed.FS stores files with their relative path (dist/index.html).
		// Use fs.Sub to strip the dist/ prefix so the FS is rooted at dist/.
		subFS, err := fs.Sub(spaFS, "dist")
		if err == nil {
			formaCfg.WebFS = subFS
		} else {
			log.Printf("[formspec] warning: cannot init SPA from embed: %v", err)
		}
	}

	app, err := formspec.New(formaCfg)
	if err != nil {
		log.Fatalf("[formspec] engine boot: %v", err)
	}
	log.Printf("[formspec] engine loaded: %d routes", app.RouteCount())

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

		if appEndpointURL != "" {
			var err error
			monitor, err = sidecar.NewAppMonitor(appEndpointURL, 10*time.Second, 3)
			if err != nil {
				log.Fatalf("[formspec] app monitor: %v", err)
			}
			go monitor.Run(ctx)
		}

		socketSrv = sidecar.NewServer(listenURL, sidecar.NewCtxHandler(resolver, "demo"), monitor, nil)
		if err := socketSrv.Listen(); err != nil {
			log.Fatalf("[formspec] ctx listener: %v", err)
		}
		log.Printf("[formspec] ctx listener on %s", listenURL)
	}

	// ── 12. App process (non-Go runtimes) ──
	var appProc *appProcess
	if !isLocalRuntime(cfg.Runtime) {
		var err error
		appProc, err = startAppProcess(ctx, cfg.Runtime, cfg.AppDir, cfg.AppEntrypoint, appEndpointURL, listenURL, cfg.DevMode)
		if err != nil {
			log.Fatalf("[formspec] app process: %v", err)
		}
	}

	// ── 13. Dev UI: Vite dev server ──
	var viteProc *appProcess
	var vitePort string
	var viteProxyURL string
	if cfg.DevUI {
		webDir, err := findWebDir()
		if err != nil {
			log.Fatalf("[formspec] %v", err)
		}
		viteProc, vitePort, err = startVite(ctx, webDir)
		if err != nil {
			log.Fatalf("[formspec] vite: %v", err)
		}
		viteProxyURL = "http://localhost:" + vitePort
		log.Printf("[formspec] Vite dev server started (%s)", viteProxyURL)
	}

	// ── 13b. Wrap handler dengan Vite proxy ──
	// SPA routes di :8080 di-proxy ke Vite dev server, sehingga user
	// cukup akses satu port (:8080) untuk API + frontend dengan HMR.
	handler := app.Handler()
	if viteProxyURL != "" {
		log.Printf("[formspec] SPA routes at %s proxied to Vite (%s)", cfg.Addr, viteProxyURL)
		handler = viteSPAProxy(handler, viteProxyURL, cfg.WorkspaceID)
	}

	// ── 13c. Spec hot-reload watcher (dev mode only) ──
	// Watches the spec directory for YAML/STAR file changes and triggers
	// a full reload of all registries without restarting the process.
	// In --dev-ui mode, also notifies Vite HMR so browsers refresh instantly.
	viteHMRURL := ""
	if viteProxyURL != "" {
		viteHMRURL = viteProxyURL + "/_dev/hmr-reload"
	}
	go watchSpecForChanges(ctx, app, cfg.SpecPath, viteHMRURL)

	// ── 14. SPA info ──
	if cfg.DevUI {
		log.Printf("[formspec] Frontend: http://localhost%s/default/_admin (proxied to Vite HMR)", cfg.Addr)
		log.Printf("[formspec]   Vite direct: http://localhost:%s/default/_admin", vitePort)
	} else if cfg.WebDir == "" {
		log.Printf("[formspec] SPA embedded — open http://localhost%s/default/_admin", cfg.Addr)
	}

	// ── 15. Serve ──
	// Start the outbox worker (durable event delivery, todo 7.3.1) before
	// serving — the dev command serves on its own http.Server rather than
	// app.ListenAndServe(), so the worker must be started explicitly.
	app.StartBackgroundWorkers()
	restSrv := &http.Server{Addr: cfg.Addr, Handler: handler}
	errCh := make(chan error, 2)
	if socketSrv != nil {
		go func() { errCh <- socketSrv.Serve() }()
	}
	go func() {
		log.Printf("[formspec] REST API on %s", cfg.Addr)
		if err := restSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("[formspec] shutting down...")
	case err := <-errCh:
		log.Printf("[formspec] server error: %v", err)
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
	return filepath.Join(".formspec", pidFileName)
}

// autoKillPrevious / writePIDFile / cleanupPIDFile delegate to the shared
// internal/devserver package (also used by cmd/formspec-registry).
func autoKillPrevious() { devserver.AutoKillPrevious(pidFilePath()) }

func writePIDFile() { devserver.WritePIDFile(pidFilePath()) }

func cleanupPIDFile() { devserver.CleanupPIDFile(pidFilePath()) }

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
		log.Fatalf("[formspec] cannot chdir to %s: %v", dir, err)
	}
	log.Printf("[formspec] working directory: %s", dir)
	return args[1:]
}

// findWebDist walks up from CWD looking for renderers/react-shadcn/dist/ directory.
// Returns the absolute path if found, empty string otherwise.
func findWebDist() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, "renderers", "react-shadcn", "dist")
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
	dsn := fs.String("dsn", "", "Database DSN (default: sqlite:.formspec/data.db)")
	addr := fs.String("addr", "", "REST API listen address (default: :8080)")
	listen := fs.String("listen", "", `Ctx listener mode: "none", "local_http", "unix_socket" (default: none)`)
	appEndpoint := fs.String("app-endpoint", "", `App endpoint mode: "none", "local_http", "unix_socket" (default: none)`)
	listenURL := fs.String("listen-url", "", "Custom listen URL (override mode auto-resolve)")
	appEndpointURL := fs.String("app-endpoint-url", "", "Custom app endpoint URL (override mode auto-resolve)")
	workspaceID := fs.String("workspace-id", "", "Workspace ID (default: default)")
	runtime := fs.String("runtime", "", `App runtime: "auto" (default), "local", "php", "python", "ruby", "java", "dotnet", "go", "rust", "node"`)
	stateDir := fs.String("state-dir", "", "State directory (default: .formspec)")
	devMode := fs.Bool("dev", false, "Development mode (implied by --dev-ui)")
	devUI := fs.Bool("dev-ui", false, "Development UI: start Vite HMR (implies --dev)")
	devAuth := fs.Bool("dev-auth", false, "Enable real JWT auth in dev mode (login + authorization enforced)")
	jwtSecret := fs.String("jwt-secret", "", "HMAC secret for JWT signing (persists tokens across restarts)")
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
		DevAuth:        *devAuth,
		JWTSecret:      *jwtSecret,
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

// ensurePort / killDescendants / extractPort / findProcessOnPort delegate to
// the shared internal/devserver package (also used by cmd/formspec-registry).
func ensurePort(addr, ownProcessName string) error {
	return devserver.EnsurePort(addr, ownProcessName)
}

func processAlive(pid int) bool { return devserver.ProcessAlive(pid) }

func killDescendants(pid int) { devserver.KillDescendants(pid) }

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
		log.Fatalf("[formspec] invalid Vite target URL %q: %v", viteTarget, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// API & Meta routes → passthrough ke backend.
		//   /{ws}/_ui/...    UI surface (meta API, entity CRUD, WebSocket)
		//   /{ws}/api/...    External API
		//   /health          Health check
		if strings.HasPrefix(path, "/"+workspaceID+"/_ui/") ||
			strings.HasPrefix(path, "/"+workspaceID+"/api/") ||
			path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Semua yang lain (/{ws}/_admin/..., /{ws}/app/...) → Vite dev server
		proxy.ServeHTTP(w, r)
	})
}

// watchSpecForChanges delegates to the shared internal/devserver watcher
// (also used by cmd/formspec-registry), adding the Vite HMR notify callback
// used in --dev-ui mode.
func watchSpecForChanges(ctx context.Context, app *formspec.App, specPath string, viteHMRURL string) {
	var onReload func()
	if viteHMRURL != "" {
		onReload = func() {
			if resp, err := http.Get(viteHMRURL); err != nil {
				log.Printf("[formspec] vite hmr notify: %v", err)
			} else {
				resp.Body.Close()
			}
		}
	}
	devserver.WatchSpec(ctx, app, specPath, onReload)
}
