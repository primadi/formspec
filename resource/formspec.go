// Package formspec is the FormSpec Resource engine: entity CRUD, permission
// enforcement, a generated REST API, and Starlark/native action dispatch
// driven by a Document/Entity manifest.
//
// The package name (formspec) intentionally differs from its directory
// (resource/): the directory keeps the engine separate from docs, web, and
// the other binaries in this repo, while the package name gives apps the
// branded API surface. Always import it with an explicit alias:
//
//	import formspec "github.com/primadi/formspec/resource"
//
//	app, err := formspec.New(formspec.Config{SpecPath: "./spec", DSN: "sqlite:data.db"})
//	if err != nil { log.Fatal(err) }
//	log.Fatal(app.ListenAndServe())
//
// This is the filesystem-loading path — manifests are read directly from
// SpecPath. It is the right choice for dev, tests, and standalone
// deployments. For apps participating in the Control Plane artifact
// pipeline (pull-based deployment across a fleet), see SyncAgent in
// syncagent.go — wiring a SyncAgent's pulled manifests into a live App's
// routes is not implemented yet (see docs/runtimes/02-formspec-resource.md §7).
//
// See docs/runtimes/02-formspec-resource.md for the full design and current
// implementation status.
package formspec

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/api"
	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/permission"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/internal/validation"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"

	"github.com/golang-jwt/jwt/v5"
)

// Config configures an App loaded from a manifest directory on disk.
type Config struct {
	DSN              string        // Database DSN, e.g. "sqlite:.formspec/data.db" (default: "sqlite:.formspec/data.db")
	SpecPath         string        // Path to the manifest directory (default: "./spec")
	Addr             string        // HTTP listen address for ListenAndServe (default: ":8080")
	ProdMode         bool          // Enable JWT auth and strict `uses` enforcement
	JWTSecret        string        // HMAC secret for JWT validation (ProdMode, symmetric)
	JWTIssuer        string        // JWT issuer (default: "formspec")
	JWTPublicKeyPath string        // PEM file for asymmetric JWT validation (ProdMode)
	StrictMode       bool          // Force strict `uses` enforcement even outside ProdMode
	IdempotencyTTL   time.Duration // TTL for idempotency keys (default: db.DefaultIdempotencyTTL)
	WorkspaceID      string        // Tenant scope used by script save/load/call handlers (default: "demo")

	// SidecarEndpoint is the app-process endpoint for impl: {type: sidecar}
	// actions ("unix:///tmp/formspec/app.sock" or "http://localhost:9000").
	// Empty means sidecar actions fail with a not-configured error — the
	// correct behavior for embedded Go apps, where no app process exists.
	// Set by cmd/formspec-sidecar (docs/runtimes/04-formspec-sidecar.md §4.2).
	SidecarEndpoint      string
	SidecarInvokeTimeout time.Duration // per-invoke timeout (default: action.DefaultSidecarInvokeTimeout)

	// WebDir is the built renderer SPA root (renderers/react-shadcn/dist). When set, the app
	// serves it at /{ws}/_admin and /{ws}/app with an index.html fallback
	// for client-side routes. Empty = API only (unless WebFS is set).
	WebDir string

	// WebFS is an embed.FS (or any fs.FS) containing the built SPA files
	// at the root. When set, serves SPA at /{ws}/_admin and /{ws}/app
	// with index.html fallback. Takes precedence over WebDir.
	WebFS fs.FS

	// ThemeDirs lists additional directories containing Theme manifests
	// (Frontend Spec §10). These are loaded alongside the main SpecPath
	// so theme modules can live outside the app's spec/modules/ tree.
	// Useful for shared/global theme registries. Paths can be absolute
	// or relative to the working directory.
	ThemeDirs []string
}

func (c *Config) applyDefaults() {
	if c.DSN == "" {
		c.DSN = "sqlite:.formspec/data.db"
	}
	if c.SpecPath == "" {
		c.SpecPath = "./spec"
	}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.JWTIssuer == "" {
		c.JWTIssuer = "formspec"
	}
	if c.IdempotencyTTL == 0 {
		c.IdempotencyTTL = db.DefaultIdempotencyTTL
	}
	if c.WorkspaceID == "" {
		c.WorkspaceID = "demo"
	}
}

// ─── Public Native Handler API ───

// NativeHandler is a Go function that implements a business-logic action.
// Register it via App.RegisterNative so the framework can dispatch YAML
// actions with spec: { impl: { type: native, ref: "..." } } to your code.
//
// The ref format is "Module.Entity.Action" or "TypeName.MethodName".
// Example: "Billing.Order.CalculateTax"
//
// See examples/reference-app/ for a complete usage example.
type NativeHandler func(ctx context.Context, params NativeParams) (any, error)

// NativeParams mirrors internal/action.ExecuteParams for public consumption.
type NativeParams struct {
	Module      string         // owning module (e.g. "billing")
	Entity      string         // entity or service name (e.g. "order")
	ActionName  string         // action being invoked (e.g. "checkout")
	ResourceID  string         // entity record ID (empty for service actions)
	Resource    map[string]any // current entity record data
	Params      map[string]any // action parameters from request body
	WorkspaceID string         // current workspace identifier
	UserID      string         // authenticated user
}

// App is a running FormSpec entity engine: loaded manifests, a synced schema,
// and a generated REST API handler.
//
// mu protects reg, rb, handler, disp for atomic reload. Read lock is
// taken on every HTTP request (Handler()), write lock during ReloadSpec().
type App struct {
	mu           sync.RWMutex
	cfg          Config
	database     db.DB
	driver       db.DriverType
	reg          *entity.Registry
	rb           *api.RouterBuilder
	handler      http.Handler
	disp         *action.Dispatcher
	nativeEx     *action.NativeExecutor
	outboxWorker *db.OutboxWorker
	idempotency  *db.IdempotencyStore
	httpServer   *http.Server

	// nativeHandlers preserves user-registered native Go handlers across
	// ReloadSpec() calls so they are re-registered on the new dispatcher.
	// Stored as action.NativeHandler (which wraps the user's NativeHandler)
	// so they can be directly passed to the new dispatcher's NativeExecutor.
	nativeHandlers map[string]action.NativeHandler
	// specVersion is incremented on every ReloadSpec() call. Exposed via
	// the Meta API so the frontend can detect when the bundle changed.
	specVersion atomic.Int64
}

// Idempotency returns the app's idempotency-key store, configured with
// cfg.IdempotencyTTL (default db.DefaultIdempotencyTTL — Core Basic §5:
// "the TTL MUST be configurable via WithTTL; hard-coding is a spec
// violation"). Exposed for the two-step prepare flow
// (POST /{resource}/{action}/prepare, Fase 2.7) to wire against once it
// lands — construction is done here, once, so the TTL is resolved from
// Config consistently regardless of which caller ends up using the store.
//
// core.idempotency_retention (a manifest-level Config key) is the intended
// long-term source for this value once the Config-kind runtime exists
// (Fase 7.2 — no such registry is loaded today, see internal/entity.Registry
// .LoadEntities, which only registers Document/Entity kinds). Until then,
// cfg.IdempotencyTTL is the equivalent Go-level configuration seam, same
// pattern as JWTSecret and the other Config fields above.
func (a *App) Idempotency() *db.IdempotencyStore { return a.idempotency }

// GetEntityStore returns the EntityStore for a given module/entity pair.
// This is used by the sidecar ctx handler for entity primitive operations
// (fetch, save, update, increment, decrement).
func (a *App) GetEntityStore(module, name string) (*db.EntityStore, error) {
	return a.reg.GetEntityStore(module, name)
}

// RegisterNative registers a Go native handler so the action dispatcher can
// route impl: { type: native, ref: "Module.Entity.Action" } calls to it.
//
// Panics if a handler is already registered for the same ref.
func (a *App) RegisterNative(ref string, handler NativeHandler) {
	// Preserve in nativeHandlers map for re-registration on ReloadSpec().
	wrapped := action.NativeHandler(func(ctx context.Context, params action.ExecuteParams) (any, error) {
		return handler(ctx, NativeParams{
			Module:      params.Module,
			Entity:      params.Entity,
			ActionName:  params.ActionName,
			ResourceID:  params.ResourceID,
			Resource:    params.Resource,
			Params:      params.Params,
			WorkspaceID: params.WorkspaceID,
			UserID:      params.UserID,
		})
	})
	a.mu.Lock()
	if a.nativeHandlers == nil {
		a.nativeHandlers = make(map[string]action.NativeHandler)
	}
	a.nativeHandlers[ref] = wrapped
	a.mu.Unlock()
	a.nativeEx.Register(ref, wrapped)
}

// RegisterNatives registers multiple native handlers at once (convenience).
func (a *App) RegisterNatives(handlers map[string]NativeHandler) {
	for ref, handler := range handlers {
		a.RegisterNative(ref, handler)
	}
}

// New loads entities from cfg.SpecPath, syncs the database schema, and
// builds the REST API router. It does not start listening — call
// ListenAndServe, or mount Handler() into your own http.Server.
//
// Auth validator, strict-mode, and `uses`-lookup wiring set process-global
// state in the underlying packages (internal/api, internal/auth,
// internal/validation) — only one App should be constructed per process.
func New(cfg Config) (*App, error) {
	cfg.applyDefaults()

	if err := configureAuth(cfg); err != nil {
		return nil, err
	}
	api.SetStrictMode(cfg.StrictMode || cfg.ProdMode)

	database, err := db.Open(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}

	reg := entity.NewRegistry(database, driver, cfg.SpecPath)
	for _, loadErr := range reg.LoadEntities() {
		fmt.Fprintf(os.Stderr, "formspec: load warning: %v\n", loadErr)
	}

	permReg := reg.GetPermissionRegistry()
	auth.SetPermissionChecker(permission.NewAuthChecker(permReg))

	if _, err := reg.SyncSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("sync schema: %w", err)
	}

	// Frontend UI kinds (Page/Form/Table/... — Frontend Spec §2) + Meta API.
	uiReg := ui.NewRegistry()
	for _, loadErr := range uiReg.LoadDir(cfg.SpecPath) {
		fmt.Fprintf(os.Stderr, "formspec: ui load warning: %v\n", loadErr)
	}
	// Load additional theme directories (Frontend Spec §10).
	// These share the same registry, so theme names must be unique
	// across all loaded paths.
	for _, themeDir := range cfg.ThemeDirs {
		resolved := themeDir
		if !filepath.IsAbs(themeDir) {
			wd, _ := os.Getwd()
			resolved = filepath.Join(wd, themeDir)
		}
		for _, loadErr := range uiReg.LoadDir(resolved) {
			fmt.Fprintf(os.Stderr, "formspec: ui load warning (theme %s): %v\n", themeDir, loadErr)
		}
	}
	resolveEntity := func(module, name string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	}
	for _, valErr := range uiReg.Validate(resolveEntity) {
		fmt.Fprintf(os.Stderr, "formspec: ui validate warning: %v\n", valErr)
	}

	// Resolve kind: App / kind: Module manifests (Core §4.4/§4.5). A
	// workspace MAY declare more than one App; all of them run
	// simultaneously in this one process, distinguished by root_url.
	specManifests, err := manifest.NewLoader(cfg.SpecPath).LoadAll()
	if err != nil {
		return nil, fmt.Errorf("load manifests for app resolution: %w", err)
	}
	resolvedApps, err := formspec_app.Resolve(specManifests.Manifests, uiReg)
	if err != nil {
		return nil, fmt.Errorf("resolve apps: %w", err)
	}
	if len(resolvedApps) == 0 {
		fmt.Fprintf(os.Stderr, "formspec: warning: no kind: App manifest found under %s — /_meta/ui will 400 until one is added\n", cfg.SpecPath)
	}

	rb := api.NewRouterBuilder(reg)
	rb.BuildRoutes()
	disp := newDispatcher(reg, cfg)
	nativeEx := disp.NativeExecutor() // get the native executor from dispatcher
	rb.SetDispatcher(disp)
	rb.SetUIRegistry(uiReg)
	rb.SetApps(resolvedApps)
	if cfg.WebDir != "" {
		rb.SetWebDir(cfg.WebDir)
	}
	if cfg.WebFS != nil {
		rb.SetWebFS(cfg.WebFS)
	}

	validation.SetEntityLookup(func(module, entityName, id string) (bool, error) {
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return false, err
		}
		if _, err := store.GetByID(context.Background(), db.GetByIDParams{WorkspaceID: cfg.WorkspaceID, ID: id}); err != nil {
			return false, nil // not found = exists check fails
		}
		return true, nil
	})

	// Event delivery (Core §12): hub for immediate websocket push, outbox
	// for durable (publish.durable: true) at-least-once redelivery, event
	// log for the audit_log channel's durable record.
	outboxStore := db.NewOutboxStore(database, driver)
	eventLogStore := db.NewEventLogStore(database, driver)
	hub := rb.Hub()
	rb.SetDeliveryDeps(action.DeliveryDeps{Hub: hub, Outbox: outboxStore, EventLog: eventLogStore})

	// eventChannelLookup re-resolves an event's declared deliver: channels
	// from the live registry at delivery time (not a snapshot taken at
	// enqueue time), so a hot-reloaded manifest fix is picked up by outbox
	// retries automatically.
	eventChannelLookup := func(resource, eventName string) ([]spec.EventDeliveryDecl, bool) {
		module, name, ok := strings.Cut(resource, "/")
		if !ok {
			return nil, false
		}
		info, ok := reg.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		for _, e := range info.EntitySpec.Events {
			if e.Name == eventName {
				return e.Deliver, true
			}
		}
		return nil, false
	}
	outboxWorker := db.NewOutboxWorker(outboxStore, &db.DeliveryEventHandler{
		Hub:      hub,
		EventLog: eventLogStore,
		Lookup:   eventChannelLookup,
	})

	idempotencyStore := db.NewIdempotencyStore(database, driver).WithTTL(cfg.IdempotencyTTL)

	app := &App{
		cfg: cfg, database: database, driver: driver,
		reg: reg, rb: rb, disp: disp, nativeEx: nativeEx,
		outboxWorker:   outboxWorker,
		idempotency:    idempotencyStore,
		nativeHandlers: make(map[string]action.NativeHandler),
	}
	// specVersionFn must be set before BuildHTTP() so HandleMetaVersion
	// captures it. The closure reads from the live App's specVersion.
	rb.SetSpecVersionFn(func() int64 { return app.specVersion.Load() })
	app.handler = rb.BuildHTTP()
	return app, nil
}

// Handler returns a reload-safe REST API handler. The returned handler
// always delegates to the current internal handler — after a ReloadSpec()
// call, subsequent requests automatically use the freshly-built routes.
// The read lock is held only for the pointer read, not for the full
// request lifecycle, so ReloadSpec() never blocks in-flight requests long.
func (a *App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.RLock()
		h := a.handler
		a.mu.RUnlock()
		h.ServeHTTP(w, r)
	})
}

// Routes returns the routes generated from the loaded manifests.
func (a *App) Routes() []api.RouteDescriptor {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rb.Routes()
}

// RouteCount returns the number of generated routes.
func (a *App) RouteCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rb.RouteCount()
}

// Registry returns the underlying entity registry, for advanced use
// (inspecting loaded entities, fetching an EntityStore directly).
func (a *App) Registry() *entity.Registry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.reg
}

// SpecVersion returns the spec version counter. Incremented on every
// successful ReloadSpec() call. The frontend polls this via the Meta API
// to detect when the meta bundle needs re-fetching.
func (a *App) SpecVersion() int64 { return a.specVersion.Load() }

// ListenAndServe starts the HTTP server on cfg.Addr. It also starts the
// outbox worker (background delivery of durable events) — started here
// rather than in New() so building an App for tests (which typically only
// call Handler()) never spins up a background poller.
func (a *App) ListenAndServe() error {
	a.outboxWorker.Start(context.Background())
	a.httpServer = &http.Server{Addr: a.cfg.Addr, Handler: a.handler}
	return a.httpServer.ListenAndServe()
}

// Close gracefully stops the outbox worker and, if ListenAndServe started
// one, the HTTP server. Safe to call even when ListenAndServe was never
// used — OutboxWorker.Stop() is a no-op if never started, and httpServer
// is nil.
func (a *App) Close(ctx context.Context) error {
	a.outboxWorker.Stop()
	if a.httpServer != nil {
		return a.httpServer.Shutdown(ctx)
	}
	return nil
}

// ReloadSpec re-reads all YAML manifests from cfg.SpecPath, rebuilds the
// entity registry, UI registry, App resolution, and HTTP router, then
// atomically swaps them into the live App. In-flight requests finish
// against the old registries; new requests use the fresh ones.
//
// Safe to call concurrently with request serving. The heavy loading work
// (manifest parsing, schema sync, route building) happens without holding
// the write lock — only the pointer swap is locked. Native Go handlers
// registered via RegisterNative / RegisterNatives are preserved across
// the reload.
//
// Not safe to call concurrently with itself — callers must serialise.
func (a *App) ReloadSpec() error {
	// ── 1. Build fresh entity registry ──
	newReg := entity.NewRegistry(a.database, a.driver, a.cfg.SpecPath)
	for _, loadErr := range newReg.LoadEntities() {
		fmt.Fprintf(os.Stderr, "formspec: reload: %v\n", loadErr)
	}

	permReg := newReg.GetPermissionRegistry()
	auth.SetPermissionChecker(permission.NewAuthChecker(permReg))

	if _, err := newReg.SyncSchema(context.Background()); err != nil {
		return fmt.Errorf("reload sync schema: %w", err)
	}

	// ── 2. Build fresh UI registry ──
	newUIReg := ui.NewRegistry()
	for _, loadErr := range newUIReg.LoadDir(a.cfg.SpecPath) {
		fmt.Fprintf(os.Stderr, "formspec: reload ui: %v\n", loadErr)
	}
	for _, themeDir := range a.cfg.ThemeDirs {
		resolved := themeDir
		if !filepath.IsAbs(themeDir) {
			wd, _ := os.Getwd()
			resolved = filepath.Join(wd, themeDir)
		}
		for _, loadErr := range newUIReg.LoadDir(resolved) {
			fmt.Fprintf(os.Stderr, "formspec: reload ui (theme %s): %v\n", themeDir, loadErr)
		}
	}
	resolveEntity := func(module, name string) (*spec.EntitySpec, bool) {
		info, ok := newReg.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	}
	for _, valErr := range newUIReg.Validate(resolveEntity) {
		fmt.Fprintf(os.Stderr, "formspec: reload ui validate: %v\n", valErr)
	}

	// ── 3. Re-resolve App/Module manifests ──
	specManifests, err := manifest.NewLoader(a.cfg.SpecPath).LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec: reload app resolution: %v\n", err)
	}
	resolvedApps, err := formspec_app.Resolve(specManifests.Manifests, newUIReg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec: reload resolve apps: %v\n", err)
	}
	if len(resolvedApps) == 0 {
		fmt.Fprintf(os.Stderr, "formspec: reload warning: no kind: App manifest found\n")
	}

	// ── 4. Build fresh router, dispatcher, and handler ──
	// Preserve the existing WSHub so in-flight WebSocket connections
	// continue receiving events after the reload.
	a.mu.RLock()
	oldHub := a.rb.Hub()
	a.mu.RUnlock()

	newRB := api.NewRouterBuilder(newReg)
	// Transfer existing hub (and its live WebSocket connections) to the
	// new router — SetHub must precede BuildRoutes so the WS handler
	// registered by BuildHTTP uses the correct hub reference.
	newRB.SetHub(oldHub)
	// Wire spec version function BEFORE BuildHTTP() so HandleMetaVersion
	// captures it. Reads from the live App's atomic counter.
	newRB.SetSpecVersionFn(func() int64 { return a.specVersion.Load() })
	newRB.BuildRoutes()
	newDisp := newDispatcher(newReg, a.cfg)

	// Re-register native Go handlers on the new dispatcher.
	a.mu.RLock()
	for ref, h := range a.nativeHandlers {
		newDisp.NativeExecutor().Register(ref, h)
	}
	a.mu.RUnlock()

	newRB.SetDispatcher(newDisp)
	newRB.SetUIRegistry(newUIReg)
	newRB.SetApps(resolvedApps)
	if a.cfg.WebDir != "" {
		newRB.SetWebDir(a.cfg.WebDir)
	}
	if a.cfg.WebFS != nil {
		newRB.SetWebFS(a.cfg.WebFS)
	}

	// Wire event delivery. The outboxStore and eventLogStore share the
	// same database tables as before, so the existing outboxWorker
	// continues to drain pending events without interruption.
	outboxStore := db.NewOutboxStore(a.database, a.driver)
	eventLogStore := db.NewEventLogStore(a.database, a.driver)
	newRB.SetDeliveryDeps(action.DeliveryDeps{Hub: oldHub, Outbox: outboxStore, EventLog: eventLogStore})

	newHandler := newRB.BuildHTTP()

	// ── 5. Re-wire process-global validation entity lookup ──
	validation.SetEntityLookup(func(module, entityName, id string) (bool, error) {
		store, err := newReg.GetEntityStore(module, entityName)
		if err != nil {
			return false, err
		}
		if _, err := store.GetByID(context.Background(), db.GetByIDParams{WorkspaceID: a.cfg.WorkspaceID, ID: id}); err != nil {
			return false, nil
		}
		return true, nil
	})

	// ── 6. Atomic swap — only the pointer assignment is locked ──
	a.mu.Lock()
	a.reg = newReg
	a.rb = newRB
	a.handler = newHandler
	a.disp = newDisp
	a.nativeEx = newDisp.NativeExecutor()
	a.mu.Unlock()

	a.specVersion.Add(1)

	fmt.Fprintf(os.Stderr, "formspec: reload complete — %d routes, %d entities (v%d)\n",
		len(newRB.Routes()), newReg.Count(), a.specVersion.Load())
	return nil
}

func configureAuth(cfg Config) error {
	if !cfg.ProdMode {
		api.SetAuthValidator(auth.NewDevValidator())
		return nil
	}
	if cfg.JWTSecret == "" && cfg.JWTPublicKeyPath == "" {
		return fmt.Errorf("ProdMode requires JWTSecret or JWTPublicKeyPath")
	}
	if cfg.JWTPublicKeyPath != "" {
		pemData, err := os.ReadFile(cfg.JWTPublicKeyPath)
		if err != nil {
			return fmt.Errorf("read JWT public key file: %w", err)
		}
		var key any
		key, err = jwt.ParseECPublicKeyFromPEM(pemData)
		if err != nil {
			key, err = jwt.ParseRSAPublicKeyFromPEM(pemData)
			if err != nil {
				return fmt.Errorf("parse JWT public key (tried ECDSA and RSA): %w", err)
			}
		}
		api.SetAuthValidator(auth.NewJWTValidatorWithKey(key, cfg.JWTIssuer, ""))
		return nil
	}
	api.SetAuthValidator(auth.NewJWTValidator(cfg.JWTSecret, cfg.JWTIssuer, ""))
	return nil
}

func newDispatcher(reg *entity.Registry, cfg Config) *action.Dispatcher {
	disp := action.NewDispatcher()

	scriptEx := action.NewScriptExecutor(cfg.SpecPath)
	scriptEx.SetSaveHandler(func(ctx context.Context, workspaceID, module, entityName, id string, version int, data map[string]any) error {
		if id == "" {
			return fmt.Errorf("resource.save: cannot save before the record exists — use resource.set() during a before-create hook/impl; the framework persists automatically")
		}
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return fmt.Errorf("get store: %w", err)
		}
		_, err = store.Update(ctx, db.UpdateParams{
			WorkspaceID: workspaceID,
			ID:          id,
			Version:     version,
			UpdatedBy:   "script",
			Data:        data,
			Permissions: auth.PermissionsFromContext(ctx),
		})
		return err
	})
	scriptEx.SetCallHandler(func(ctx context.Context, workspaceID, fromModule, targetModule, targetEntity, actionName string, params map[string]any) (any, error) {
		if targetModule == "" {
			targetModule = fromModule
		}
		return invokeAction(ctx, reg, disp, workspaceID, targetModule, targetEntity, actionName, "", params)
	})
	scriptEx.SetLoadHandler(func(ctx context.Context, workspaceID, module, entityName, id string) (map[string]any, int, error) {
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return nil, 0, fmt.Errorf("get store: %w", err)
		}
		rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
		if err != nil {
			return nil, 0, err
		}
		if rec == nil {
			return nil, 0, fmt.Errorf("record not found")
		}
		return rec.Data, rec.Version, nil
	})
	scriptEx.SetCreateHandler(func(ctx context.Context, workspaceID, module, entityName string, data map[string]any) (string, error) {
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return "", fmt.Errorf("get store: %w", err)
		}
		return store.Insert(ctx, db.InsertParams{
			WorkspaceID: workspaceID,
			CreatedBy:   "script",
			Data:        data,
		})
	})
	scriptEx.SetNextKeyHandler(func(ctx context.Context, workspaceID, module, entityName, fieldName string) (string, error) {
		return generateNextKey(ctx, reg, workspaceID, module, entityName, fieldName)
	})
	disp.RegisterExecutor(spec.ImplScript, scriptEx)
	disp.RegisterExecutor(spec.ImplScriptRef, scriptEx)

	nativeEx := action.NewNativeExecutor()
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.RegisterExecutor(spec.ImplSidecar, newSidecarExecutor(cfg))

	disp.SetNativeExecutor(nativeEx)
	return disp
}

// generateNextKey is the ctx.next_key(field) backing for scripts — delegates
// to the entity registry's natural-key counter. Automatic natural-key
// generation on plain Create is separate (wired directly into
// db.EntityStore.Insert, since it must run before required-field validation).
func generateNextKey(ctx context.Context, reg *entity.Registry, workspaceID, module, entityName, fieldName string) (string, error) {
	return reg.GenerateNaturalKey(ctx, workspaceID, module, entityName, fieldName)
}

// invokeAction runs a named action on module/entity — the resource.call()
// backing for cross-resource script calls. If resourceID is non-empty, the
// current record (data + version) is loaded first so the target action's
// script sees real data and can save with correct CAS, exactly like an
// HTTP-triggered custom action would.
//
// Note: unlike the HTTP path (internal/api/handler.go's HandleCustomAction),
// this does not re-run EvaluateConditions before dispatch — no current
// script exercises resource.call(), so that parity gap is deferred rather
// than spending plumbing on an untested path.
func invokeAction(ctx context.Context, reg *entity.Registry, disp *action.Dispatcher, workspaceID, module, entityName, actionName, resourceID string, params map[string]any) (any, error) {
	actionSpec, ok := reg.GetActionSpec(module, entityName, actionName)
	if !ok {
		return nil, fmt.Errorf("resource.call: action %s.%s.%s not found", module, entityName, actionName)
	}

	resourceData := make(map[string]any)
	resourceVersion := 0
	if resourceID != "" {
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return nil, fmt.Errorf("resource.call: get store: %w", err)
		}
		rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: resourceID})
		if err != nil {
			return nil, fmt.Errorf("resource.call: load %s.%s(%s): %w", module, entityName, resourceID, err)
		}
		if rec != nil {
			resourceData = rec.Data
			resourceVersion = rec.Version
		}
	}

	result, err := disp.Dispatch(ctx, *actionSpec, action.ExecuteParams{
		Module:          module,
		Entity:          entityName,
		ActionName:      actionName,
		ResourceID:      resourceID,
		Resource:        resourceData,
		ResourceVersion: resourceVersion,
		Params:          params,
		WorkspaceID:     workspaceID,
	})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

func newSidecarExecutor(cfg Config) action.Executor {
	if cfg.SidecarEndpoint == "" {
		return action.NewSidecarExecutor()
	}
	ex, err := action.NewSidecarExecutorWithEndpoint(cfg.SidecarEndpoint, cfg.SidecarInvokeTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec: invalid SidecarEndpoint %q (%v) — sidecar actions will fail\n", cfg.SidecarEndpoint, err)
		return action.NewSidecarExecutor()
	}
	return ex
}
