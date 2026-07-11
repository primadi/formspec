// Package forma is the Forma Resource engine: entity CRUD, permission
// enforcement, a generated REST API, and Starlark/native action dispatch
// driven by a Document/Entity manifest.
//
// The package name (forma) intentionally differs from its directory
// (resource/): the directory keeps the engine separate from docs, web, and
// the other binaries in this repo, while the package name gives apps the
// branded API surface. Always import it with an explicit alias:
//
//	import forma "github.com/primadi/forma/resource"
//
//	app, err := forma.New(forma.Config{SpecPath: "./spec", DSN: "sqlite:data.db"})
//	if err != nil { log.Fatal(err) }
//	log.Fatal(app.ListenAndServe())
//
// This is the filesystem-loading path — manifests are read directly from
// SpecPath. It is the right choice for dev, tests, and standalone
// deployments. For apps participating in the Control Plane artifact
// pipeline (pull-based deployment across a fleet), see SyncAgent in
// syncagent.go — wiring a SyncAgent's pulled manifests into a live App's
// routes is not implemented yet (see docs/runtimes/02-forma-resource.md §7).
//
// See docs/runtimes/02-forma-resource.md for the full design and current
// implementation status.
package forma

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/primadi/forma/internal/action"
	"github.com/primadi/forma/internal/api"
	"github.com/primadi/forma/internal/auth"
	"github.com/primadi/forma/internal/db"
	"github.com/primadi/forma/internal/entity"
	"github.com/primadi/forma/internal/permission"
	"github.com/primadi/forma/internal/ui"
	"github.com/primadi/forma/internal/validation"
	"github.com/primadi/forma/pkg/spec"

	"github.com/golang-jwt/jwt/v5"
)

// Config configures an App loaded from a manifest directory on disk.
type Config struct {
	DSN              string        // Database DSN, e.g. "sqlite:.forma/data.db" (default: "sqlite:.forma/data.db")
	SpecPath         string        // Path to the manifest directory (default: "./spec")
	Addr             string        // HTTP listen address for ListenAndServe (default: ":8080")
	ProdMode         bool          // Enable JWT auth and strict `uses` enforcement
	JWTSecret        string        // HMAC secret for JWT validation (ProdMode, symmetric)
	JWTIssuer        string        // JWT issuer (default: "forma")
	JWTPublicKeyPath string        // PEM file for asymmetric JWT validation (ProdMode)
	StrictMode       bool          // Force strict `uses` enforcement even outside ProdMode
	IdempotencyTTL   time.Duration // TTL for idempotency keys (default: db.DefaultIdempotencyTTL)
	TenantID         string        // Tenant scope used by script save/load/call handlers (default: "demo")

	// SidecarEndpoint is the app-process endpoint for impl: {type: sidecar}
	// actions ("unix:///var/run/forma/app.sock" or "http://localhost:9000").
	// Empty means sidecar actions fail with a not-configured error — the
	// correct behavior for embedded Go apps, where no app process exists.
	// Set by cmd/forma-sidecar (docs/runtimes/04-forma-sidecar.md §4.2).
	SidecarEndpoint      string
	SidecarInvokeTimeout time.Duration // per-invoke timeout (default: action.DefaultSidecarInvokeTimeout)

	// WebDir is the built renderer SPA root (web/dist). When set, the app
	// serves it at /{ws}/_admin and /{ws}/app with an index.html fallback
	// for client-side routes. Empty = API only.
	WebDir string
}

func (c *Config) applyDefaults() {
	if c.DSN == "" {
		c.DSN = "sqlite:.forma/data.db"
	}
	if c.SpecPath == "" {
		c.SpecPath = "./spec"
	}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.JWTIssuer == "" {
		c.JWTIssuer = "forma"
	}
	if c.IdempotencyTTL == 0 {
		c.IdempotencyTTL = db.DefaultIdempotencyTTL
	}
	if c.TenantID == "" {
		c.TenantID = "demo"
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
	Module     string         // owning module (e.g. "billing")
	Entity     string         // entity or service name (e.g. "order")
	ActionName string         // action being invoked (e.g. "checkout")
	ResourceID string         // entity record ID (empty for service actions)
	Resource   map[string]any // current entity record data
	Params     map[string]any // action parameters from request body
	TenantID   string         // current workspace tenant
	UserID     string         // authenticated user
}

// App is a running Forma entity engine: loaded manifests, a synced schema,
// and a generated REST API handler.
type App struct {
	cfg      Config
	reg      *entity.Registry
	rb       *api.RouterBuilder
	handler  http.Handler
	disp     *action.Dispatcher
	nativeEx *action.NativeExecutor
}

// RegisterNative registers a Go native handler so the action dispatcher can
// route impl: { type: native, ref: "Module.Entity.Action" } calls to it.
//
// Panics if a handler is already registered for the same ref.
func (a *App) RegisterNative(ref string, handler NativeHandler) {
	a.nativeEx.Register(ref, func(ctx context.Context, params action.ExecuteParams) (any, error) {
		return handler(ctx, NativeParams{
			Module:     params.Module,
			Entity:     params.Entity,
			ActionName: params.ActionName,
			ResourceID: params.ResourceID,
			Resource:   params.Resource,
			Params:     params.Params,
			TenantID:   params.TenantID,
			UserID:     params.UserID,
		})
	})
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
		fmt.Fprintf(os.Stderr, "forma: load warning: %v\n", loadErr)
	}

	permReg := reg.GetPermissionRegistry()
	auth.SetPermissionChecker(permission.NewAuthChecker(permReg))

	if _, err := reg.SyncSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("sync schema: %w", err)
	}

	// Frontend UI kinds (Page/Form/Table/... — Frontend Spec §2) + Meta API.
	uiReg := ui.NewRegistry()
	for _, loadErr := range uiReg.LoadDir(cfg.SpecPath) {
		fmt.Fprintf(os.Stderr, "forma: ui load warning: %v\n", loadErr)
	}
	resolveEntity := func(module, name string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	}
	for _, valErr := range uiReg.Validate(resolveEntity) {
		fmt.Fprintf(os.Stderr, "forma: ui validate warning: %v\n", valErr)
	}

	rb := api.NewRouterBuilder(reg)
	rb.BuildRoutes()
	disp := newDispatcher(reg, cfg)
	nativeEx := disp.NativeExecutor() // get the native executor from dispatcher
	rb.SetDispatcher(disp)
	rb.SetUIRegistry(uiReg)
	if cfg.WebDir != "" {
		rb.SetWebDir(cfg.WebDir)
	}

	validation.SetEntityLookup(func(module, entityName, id string) (bool, error) {
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return false, err
		}
		if _, err := store.GetByID(context.Background(), db.GetByIDParams{TenantID: cfg.TenantID, ID: id}); err != nil {
			return false, nil // not found = exists check fails
		}
		return true, nil
	})

	return &App{cfg: cfg, reg: reg, rb: rb, handler: rb.BuildHTTP(), disp: disp, nativeEx: nativeEx}, nil
}

// Handler returns the generated REST API handler, for mounting into your
// own http.Server or alongside other routes.
func (a *App) Handler() http.Handler { return a.handler }

// Routes returns the routes generated from the loaded manifests.
func (a *App) Routes() []api.RouteDescriptor { return a.rb.Routes() }

// RouteCount returns the number of generated routes.
func (a *App) RouteCount() int { return a.rb.RouteCount() }

// Registry returns the underlying entity registry, for advanced use
// (inspecting loaded entities, fetching an EntityStore directly).
func (a *App) Registry() *entity.Registry { return a.reg }

// ListenAndServe starts the HTTP server on cfg.Addr.
func (a *App) ListenAndServe() error {
	return http.ListenAndServe(a.cfg.Addr, a.handler)
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
	scriptEx.SetSaveHandler(func(module, entityName, id string, data map[string]any) error {
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return fmt.Errorf("get store: %w", err)
		}
		_, err = store.Update(context.Background(), db.UpdateParams{
			TenantID:  cfg.TenantID,
			ID:        id,
			Version:   0, // TODO: fetch current version for CAS
			UpdatedBy: "script",
			Data:      data,
		})
		return err
	})
	scriptEx.SetCallHandler(func(fromModule, targetModule, targetEntity, actionName string, _ map[string]any) (any, error) {
		if targetModule == "" {
			targetModule = fromModule
		}
		// TODO: dispatch to the action dispatcher for cross-resource calls
		return map[string]any{"status": "called", "target": fmt.Sprintf("%s.%s.%s", targetModule, targetEntity, actionName)}, nil
	})
	scriptEx.SetLoadHandler(func(module, entityName, id string) (map[string]any, error) {
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return nil, fmt.Errorf("get store: %w", err)
		}
		rec, err := store.GetByID(context.Background(), db.GetByIDParams{TenantID: cfg.TenantID, ID: id})
		if err != nil {
			return nil, err
		}
		if rec == nil {
			return nil, fmt.Errorf("record not found")
		}
		return rec.Data, nil
	})
	scriptEx.SetNextKeyHandler(func(_ string) (string, error) {
		// TODO: delegate to db.NaturalKeyCounter instead of a timestamp placeholder
		return fmt.Sprintf("KEY-%d", time.Now().UnixNano()), nil
	})
	disp.RegisterExecutor(spec.ImplScript, scriptEx)
	disp.RegisterExecutor(spec.ImplScriptRef, scriptEx)

	nativeEx := action.NewNativeExecutor()
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.RegisterExecutor(spec.ImplSidecar, newSidecarExecutor(cfg))

	disp.SetNativeExecutor(nativeEx)
	return disp
}

func newSidecarExecutor(cfg Config) action.Executor {
	if cfg.SidecarEndpoint == "" {
		return action.NewSidecarExecutor()
	}
	ex, err := action.NewSidecarExecutorWithEndpoint(cfg.SidecarEndpoint, cfg.SidecarInvokeTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forma: invalid SidecarEndpoint %q (%v) — sidecar actions will fail\n", cfg.SidecarEndpoint, err)
		return action.NewSidecarExecutor()
	}
	return ex
}
