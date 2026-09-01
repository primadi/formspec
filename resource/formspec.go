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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/api"
	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/config"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/integrator"
	"github.com/primadi/formspec/internal/job"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/observability"
	"github.com/primadi/formspec/internal/period"
	"github.com/primadi/formspec/internal/permission"
	"github.com/primadi/formspec/internal/service"
	"github.com/primadi/formspec/internal/stream"
	"github.com/primadi/formspec/internal/subscription"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/internal/validation"
	"github.com/primadi/formspec/internal/vendor"
	"github.com/primadi/formspec/internal/webhook"
	"github.com/primadi/formspec/internal/workflow"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"

	"github.com/golang-jwt/jwt/v5"
)

// Config configures an App loaded from a manifest directory on disk.
type Config struct {
	DSN                string        // Database DSN, e.g. "sqlite:.formspec/data.db" (default: "sqlite:.formspec/data.db")
	SpecPath           string        // Path to the manifest directory (default: "./spec")
	Addr               string        // HTTP listen address for ListenAndServe (default: ":8080")
	ProdMode           bool          // Enable JWT auth and strict `uses` enforcement
	DevAuth            bool          // Enable real JWT auth even in dev mode (for testing authorization)
	JWTSecret          string        // HMAC secret for JWT validation (ProdMode/DevAuth, symmetric)
	JWTIssuer          string        // JWT issuer (default: "formspec")
	JWTPublicKeyPath   string        // PEM file for asymmetric JWT validation (ProdMode)
	StrictMode         bool          // Force strict `uses` enforcement even outside ProdMode
	IdempotencyTTL     time.Duration // TTL for idempotency keys (default: db.DefaultIdempotencyTTL)
	WorkspaceID        string        // Tenant scope used by script save/load/call handlers (default: "demo")
	MaxSessionsPerUser int           // Concurrent session limit per user (todo 6.5.3); 0 = unlimited
	// EnableAPIAuth mounts /api/v1/auth/* (login/refresh) on the external
	// surface. Default false — auth lives on the always-available UI surface
	// (/_ui/auth/*); /api/v1 is deny-by-default for external services
	// (01-core-basic.md §8.2). Opt-in for programmatic clients.
	EnableAPIAuth bool

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

	// ExternalDir is an additional manifest root for user-customized modules
	// (committed to git, unlike vendors/). Entities declared here win over
	// built-in formspec.core defaults (todo 6.1 merge strategy). Empty =
	// no external overrides.
	ExternalDir string

	// ── Observability (todo 8.2, spec platform/09-observability.md) ──

	// CORSOrigins is the origin allow-list (todo 8.1.5). Empty = permissive
	// dev CORS (`*`). Production must set explicit origins.
	CORSOrigins []string
	// Logger is the structured JSON-lines logger (todo 8.2.1). Nil = legacy
	// text logging (dev).
	Logger *observability.Logger
	// Metrics is the Prometheus metric set (todo 8.2.4). When non-nil,
	// requests are instrumented; expose via the admin listener.
	Metrics *observability.Metrics
	// Health is the machine-readable health registry (todo 8.2.6). When
	// non-nil, GET /health returns {status, reasons, checked_at} and a
	// datastore probe is registered automatically.
	Health *observability.Health
}

// projectRootOf derives the project root from the spec path: the spec dir
// conventionally lives at <root>/spec (08-project-layout.md); when the spec
// path IS the project root (no spec/ subdir), the parent is still the safest
// guess for formspec.lock/vendors/ discovery — vendor.ActiveModules tolerates
// a missing lock either way.
func projectRootOf(specPath string) string {
	abs, err := filepath.Abs(specPath)
	if err != nil {
		return specPath
	}
	if filepath.Base(abs) == "spec" {
		return filepath.Dir(abs)
	}
	// spec/ subdir present next to the given path?
	if fi, err := os.Stat(filepath.Join(abs, "spec")); err == nil && fi.IsDir() {
		return abs
	}
	return filepath.Dir(abs)
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
	// deliveryHandler is the outbox worker's DeliveryEventHandler. Held so a
	// ReloadSpec() can re-point its Subscriptions field to a freshly built
	// subscription dispatcher without recreating the worker (which would
	// interrupt in-flight outbox draining).
	deliveryHandler *db.DeliveryEventHandler
	idempotency     *db.IdempotencyStore
	httpServer      *http.Server
	// escalationWorker escalates stale workflow approvals (todo 7.4.4).
	escalationWorker *workflow.EscalationWorker
	// pubsub is the shared in-memory pub/sub bus backing both ctx.pubsub()
	// and the `pubsub` event delivery channel (todo 7.3.5). Held so a
	// ReloadSpec() reuses the same instance.
	pubsub *memory.PubSub
	// stream is the Tier 2 durable event-stream backend (todo 7.3.2). Held so
	// a ReloadSpec() reuses the same backend (and its consumer groups) while
	// rebuilding the streaming worker.
	stream stream.Stream
	// streamingWorker consumes durable (Tier 2) subscriptions from the stream
	// backend (todo 7.3.2).
	streamingWorker *subscription.StreamingWorker
	// dynamicRefresher periodically reloads dynamic subscriptions
	// (formspec.core.subscription, todo 7.3.4) into the subscription registry
	// so admin-panel CRUD changes take effect without a restart.
	dynamicRefresher *subscription.DynamicRefresher
	// jobTracker tracks async jobs (todo 7.13). Held so a ReloadSpec() reuses
	// the same store + hub while rebuilding the dispatcher.
	jobTracker *job.Tracker

	// nativeHandlers preserves user-registered native Go handlers across
	// ReloadSpec() calls so they are re-registered on the new dispatcher.
	// Stored as action.NativeHandler (which wraps the user's NativeHandler)
	// so they can be directly passed to the new dispatcher's NativeExecutor.
	nativeHandlers map[string]action.NativeHandler
	// specVersion is incremented on every ReloadSpec() call. Exposed via
	// the Meta API so the frontend can detect when the bundle changed.
	specVersion atomic.Int64
}

// Database returns the app's underlying database handle. Exposed so CLI
// commands (e.g. `formspec repl`) can build the same ctx.* primitive resolver
// that newDispatcher wires into scripts (todo 2.9.1–2.9.3).
func (a *App) Database() db.DB {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.database
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

	// DevAuth: enable real JWT auth in dev mode. If no explicit secret is
	// configured, generate one so the validator (configureAuth) and the token
	// issuer (below) share the same key. When the user sets JWTSecret (e.g. via
	// formspec-app.yaml `jwt-secret`), it is used as-is so issued tokens survive
	// restarts. Config is passed by value, so set it on the local cfg before
	// configureAuth and the TokenIssuer read it.
	if cfg.DevAuth && cfg.JWTSecret == "" {
		cfg.JWTSecret = randomDevSecret()
	}

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
	// Load external/ overrides (user-customized modules, committed to git) —
	// they win over built-in formspec.core defaults (todo 6.1 merge strategy).
	if cfg.ExternalDir != "" {
		reg.AddManifestRoot(cfg.ExternalDir)
	}
	// Vendor modules (todo 13.1.4): register ACTIVE vendor modules from
	// vendors/ — activation state lives in the App manifest marker blocks,
	// integrity in formspec.lock. Inactive (commented) markers are skipped.
	// The vendored module.yaml already declares its effective (aliased)
	// name — normalized at install time — so entities register under the
	// name the App manifest references. The same roots are applied to the
	// App/Module resolution loader below, so `modules:` references to
	// vendor modules resolve.
	projectRoot := projectRootOf(cfg.SpecPath)
	var vendorRoots []string
	if activeVendors, err := vendor.ActiveModules(projectRoot, cfg.SpecPath); err != nil {
		return nil, fmt.Errorf("scan vendor modules: %w", err)
	} else {
		for _, name := range activeVendors {
			vendorRoots = append(vendorRoots, filepath.Join(projectRoot, "vendors", name))
			reg.AddManifestRoot(filepath.Join(projectRoot, "vendors", name))
		}
	}
	// Shadow copies (todo 13.2.2, §6.4): overrides/ wins over modules/ and
	// vendors/ (later roots win in the loader). The §5.4 whitelist is
	// enforced first — a non-presentation kind under overrides/ refuses
	// the boot.
	if err := vendor.ValidateOverridesDir(projectRoot); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "overrides")); err == nil {
		reg.AddManifestRoot(filepath.Join(projectRoot, "overrides"))
		vendorRoots = append(vendorRoots, filepath.Join(projectRoot, "overrides"))
	}
	// Drift detection (todo 13.2.4, §5.3): warn when an upstream file
	// changed since its shadow copy was adopted — never a hard failure.
	if drifts, err := vendor.CheckDrift(projectRoot); err == nil && len(drifts) > 0 {
		for _, d := range drifts {
			fmt.Fprintf(os.Stderr, "formspec: ⚠ drift: overrides for %s/%s (module %s): %s\n",
				strings.ToLower(d.Kind), d.Name, d.Module, d.Detail)
		}
	}
	// Register framework-owned auth entities (formspec.core.user/session)
	// before loading user manifests, so external/ overrides can replace them.
	if err := auth.RegisterCoreEntities(reg); err != nil {
		return nil, fmt.Errorf("register core entities: %w", err)
	}
	// Register the framework-owned dynamic-subscription entity
	// (formspec.core.subscription, todo 7.3.4) — data, not manifest.
	if err := subscription.RegisterCoreEntities(reg); err != nil {
		return nil, fmt.Errorf("register subscription core entities: %w", err)
	}
	// Register the framework-owned period-closing entity
	// (formspec.core.period-closing, todo 7.11) — submit = finalize, cancel =
	// reopen; transaction writes in a closed period are rejected.
	if err := period.RegisterCoreEntities(reg); err != nil {
		return nil, fmt.Errorf("register period core entities: %w", err)
	}
	for _, loadErr := range reg.LoadEntities() {
		fmt.Fprintf(os.Stderr, "formspec: load warning: %v\n", loadErr)
	}

	permReg := reg.GetPermissionRegistry()
	auth.SetPermissionChecker(permission.NewAuthChecker(permReg))

	if _, err := reg.SyncSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("sync schema: %w", err)
	}

	// Wire the period-closing guard (todo 7.11.5): transaction writes whose
	// transaction_date falls in a closed period are rejected with
	// FORMSPEC.PERIOD.CLOSED. The guard reads formspec.core.period-closing
	// (submitted = closed, cancelled = reopened).
	periodGuard := period.NewGuard(reg)
	reg.SetPeriodGuard(func(ctx context.Context, workspaceID, period string) (bool, error) {
		return periodGuard.IsClosed(ctx, workspaceID, period)
	})

	// Frontend UI kinds (Page/Form/Table/... — Frontend Spec §2) + Meta API.
	uiReg := ui.NewRegistry()
	for _, loadErr := range uiReg.LoadDir(cfg.SpecPath) {
		fmt.Fprintf(os.Stderr, "formspec: ui load warning: %v\n", loadErr)
	}
	// Load framework-bundled UI manifests (auth module forms/pages/tables) so
	// the admin surface gets the friendlier access-management forms.
	for _, loadErr := range uiReg.LoadEmbedded(auth.ModuleFS()) {
		fmt.Fprintf(os.Stderr, "formspec: ui load warning (auth module): %v\n", loadErr)
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
	// Vendor roots are applied here too (todo 13.1.4) so `modules:`
	// references to active vendor modules resolve.
	appLoader := manifest.NewLoader(cfg.SpecPath)
	for _, root := range vendorRoots {
		appLoader.AddRoot(root)
	}
	specManifests, err := appLoader.LoadAll()
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

	// Config registry (todo 7.2.1): load kind: Config manifests and resolve
	// their keys into ctx.config (non-secret) / ctx.secrets (secret) stores.
	cfgReg := buildConfigRegistry(specManifests.Manifests)
	// Service registry (todo 7.1.1): load kind: Service manifests for
	// stateless action dispatch.
	svcReg := buildServiceRegistry(specManifests.Manifests)
	// Webhook registry (todo 7.6.1): load kind: Webhook manifests for
	// verified inbound endpoints.
	whReg := buildWebhookRegistry(specManifests.Manifests)
	// Subscription registry (todo 7.3.1): load kind: Subscription manifests
	// for event → handler dispatch.
	subReg := buildSubscriptionRegistry(specManifests.Manifests)
	// Workflow registry (todo 7.4.1): load kind: Workflow manifests for
	// state-machine transition interception.
	wfReg := buildWorkflowRegistry(specManifests.Manifests)
	// Integrator registry (todo 7.7.1): load kind: Integrator manifests for
	// cross-module event → action bridging.
	itReg := buildIntegratorRegistry(specManifests.Manifests)

	rb := api.NewRouterBuilder(reg)
	// Set the service registry BEFORE BuildRoutes so GenerateServiceRoutes
	// sees it (todo 7.1).
	rb.SetServiceRegistry(svcReg)
	// Set the webhook registry + key resolver BEFORE BuildRoutes so
	// GenerateWebhookRoutes sees it (todo 7.6).
	rb.SetWebhookRegistry(whReg)
	rb.SetWebhookKeyResolver(cfgReg)
	rb.BuildRoutes()
	// Shared pubsub (todo 7.3.5): one instance backs both ctx.pubsub() in
	// scripts and the `pubsub` event delivery channel, so subscribers receive
	// published events.
	sharedPubSub := memory.NewPubSub()
	// Async job tracker (todo 7.13): tracked async actions (`call: async` +
	// `track: true`) create a job row, report progress via ctx.job.progress,
	// and end completed/failed on the `jobs` websocket channel. The hub is
	// wired once available (SetHub after rb.Hub()).
	jobStore := db.NewJobStore(database, driver)
	jobTracker := job.NewTracker(jobStore, nil, cfg.JWTSecret)
	// Datastore registry (todo 2.9.4): named kind: Datastore manifests +
	// per-module `spec.datastore` bindings for module-scoped ctx.* resolution.
	dsReg, err := buildDatastoreRegistry(specManifests.Manifests, database, stateDirFromDSN(cfg.DSN), sharedPubSub)
	if err != nil {
		return nil, err
	}
	// Tier 2 stream backend (todo 7.3.2): durable subscriptions append events
	// to a stream consumed by the StreamingWorker. Backend resolved via the
	// datastore registry (plan fase E): a module bound to a service serving
	// `queue`/`pubsub` backed by Redis/Valkey provides the Redis stream;
	// otherwise in-memory (dev default). Accessed only through stream.Stream.
	streamBackend, err := buildStreamBackend(dsReg)
	if err != nil {
		return nil, err
	}

	// Entity read-through cache (Fase 14, docs/plan/fase14-entity-cache.md):
	// opt-in per entity via spec.cache.ttl. Backend = the module-bound
	// datastore serving `cache` (Redis/Valkey in multi-instance deployments)
	// or a shared in-memory KV when unbound. Cached entries hold RAW records
	// — field-security sanitization stays per-request in the handlers.
	sharedCacheKV := memory.NewKV()
	rb.SetEntityCache(&api.EntityCache{
		Resolve: func(module, entity string) api.CacheKV {
			info, ok := reg.GetEntity(module, entity)
			if !ok || info.EntitySpec == nil || info.EntitySpec.Cache == nil {
				return nil // not opted in
			}
			if dsName := dsReg.Binding(module); dsName != "" {
				if conn, err := dsReg.Resolve("cache", "", module); err == nil {
					if kv, ok := conn.(api.CacheKV); ok {
						return kv
					}
				}
			}
			return sharedCacheKV
		},
		TTLFor: func(module, entity string) time.Duration {
			info, ok := reg.GetEntity(module, entity)
			if !ok || info.EntitySpec == nil || info.EntitySpec.Cache == nil {
				return time.Minute
			}
			if ttl, err := info.EntitySpec.Cache.CacheTTL(); err == nil {
				return ttl
			}
			return time.Minute
		},
	})

	disp := newDispatcher(reg, svcReg, database, cfg, cfgReg, jobTracker, dsReg, sharedPubSub)
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

	// Auth service (todo 6.1): login + refresh backed by formspec.core
	// entities (overridable via external/ or auth_config_ref). The issuer
	// uses the same secret/issuer as the JWT validator configured in
	// configureAuth, so issued tokens validate against the middleware.
	authRoles := auth.NewRoleResolver(reg)

	// Resolve per-App auth strategy from App.spec.auth_config_ref (todo 6.1.4).
	// The referenced Config may declare a strategy (basic-auth default) and
	// entity overrides (user_entity/session_entity/role_entity) applied via
	// RoleResolver.SetOverride. Single-server implements basic-auth; other
	// strategies are declared but not yet implemented (warn + fall back).
	configs := map[string]*spec.ConfigSpec{}
	for _, raw := range specManifests.Manifests {
		if spec.Kind(raw.Kind) == spec.KindConfig {
			if cs, err := manifest.RawSpecToConfigSpec(raw.Spec.(map[string]any)); err == nil {
				configs[raw.Metadata.Name] = cs
			}
		}
	}

	// Resolve the global settings namespace (spec §10 — "jangan pernah
	// menebak"). The first Config manifest that declares `settings:` wins;
	// otherwise standard defaults apply. Exposed on every /_meta/ui bundle so
	// renderers read formatting/presentation defaults instead of guessing.
	var declaredSettings *spec.Settings
	for _, cs := range configs {
		if cs.Settings != nil {
			declaredSettings = cs.Settings
			break
		}
	}
	rb.SetSettings(spec.ResolveSettings(declaredSettings))
	// Auth on the external surface (/api/v1/auth/*) is opt-in; the UI login
	// lives on /_ui/auth/* (always available).
	rb.SetEnableAPIAuth(cfg.EnableAPIAuth)

	// Module asset roots (todo 5.9.1) — serve custom UI components from
	// {root}/modules/{module}/assets/{path}.
	assetRoots := []string{cfg.SpecPath}
	if cfg.ExternalDir != "" {
		assetRoots = append(assetRoots, cfg.ExternalDir)
	}
	rb.SetAssetRoots(assetRoots)

	// Object store for file fields (todo 7.17.1). Resolved via the datastore
	// registry (plan fase E — env-var implicit path removed): a module bound
	// to a service serving `storage` backed by minio/s3 provides the object
	// store; otherwise filesystem under the state dir (.formspec/storage).
	storageResolved := false
	for _, name := range sortedServiceNames(dsReg) {
		e := dsReg.services[name]
		if e == nil || e.spec == nil {
			continue
		}
		drv := e.spec.Driver
		if drv != spec.DatastoreDriverMinio && drv != spec.DatastoreDriverS3 {
			continue
		}
		servesStorage := false
		for _, p := range e.spec.Serves {
			if p == spec.PrimitiveStorage {
				servesStorage = true
				break
			}
		}
		if !servesStorage {
			continue
		}
		conn, err := dsReg.Resolve("storage", name, "")
		if err != nil {
			return nil, fmt.Errorf("init storage service %q: %w", name, err)
		}
		mc, ok := conn.(api.Storage)
		if !ok {
			return nil, fmt.Errorf("storage service %q does not implement api.Storage", name)
		}
		rb.SetStorageResolver(func() (api.Storage, error) { return mc, nil })
		storageResolved = true
		break
	}
	if !storageResolved {
		fsStore, err := memory.NewStorage(filepath.Join(StateDirFromDSN(cfg.DSN), "storage"))
		if err != nil {
			return nil, fmt.Errorf("init storage: %w", err)
		}
		rb.SetStorageResolver(func() (api.Storage, error) { return fsStore, nil })
	}

	appAuths, authErrs := auth.ResolveAppAuth(resolvedApps, configs)
	for _, e := range authErrs {
		fmt.Fprintf(os.Stderr, "formspec: auth config warning: %v\n", e)
	}
	for _, ac := range appAuths {
		if ac.Strategy != auth.StrategyBasicAuth {
			fmt.Fprintf(os.Stderr, "formspec: app %q auth strategy %q — not implemented in single-server (basic-auth only); login falls back to basic-auth\n", ac.App, ac.Strategy)
		}
		for role, ref := range ac.Overrides {
			authRoles.SetOverride(role, ref)
		}
	}

	// In dev mode with no explicit secret, generate a random one so issued
	// tokens are signed with a real secret (still dev-only; ProdMode
	// requires an explicit JWTSecret or JWTPublicKeyPath).
	issuerSecret := cfg.JWTSecret
	if issuerSecret == "" {
		issuerSecret = randomDevSecret()
	}
	authSvc, err := auth.NewService(authRoles, auth.NewTokenIssuer(
		issuerSecret, cfg.JWTIssuer, "", 0, 0))
	if err != nil {
		return nil, fmt.Errorf("construct auth service: %w", err)
	}
	// Wire role-based permission materialization (todo 6.3/5.12.5): the role
	// store reads formspec.core.role grants, and the materializer expands
	// page/tab/action grants into concrete {module}.{entity}.{action}
	// permission strings using the UI + entity registries.
	if roleStore, err := authRoles.Resolve(auth.RoleRole); err == nil {
		authSvc.SetRoleStore(auth.NewRoleStore(roleStore))
	}
	authSvc.SetMaterializer(auth.NewMaterializer(uiReg, reg))
	// Concurrent session limit per user (todo 6.5.3); 0 = unlimited.
	if cfg.MaxSessionsPerUser > 0 {
		authSvc.SetMaxSessionsPerUser(cfg.MaxSessionsPerUser)
	}
	api.SetAuthService(authSvc)

	// Wire API key auth (todo 6.4): the X-FormSpec-Key header on the external
	// surface (/api/v1/) resolves against formspec.core.api-key. The store is
	// resolved via RoleResolver so a user override (external/) wins.
	if apiKeyStore, err := authRoles.Resolve(auth.RoleApiKey); err == nil {
		api.SetApiKeyStore(auth.NewApiKeyStore(apiKeyStore))
	}

	// Seed a default dev user so login works out of the box in dev mode.
	// Never seeds in ProdMode.
	if !cfg.ProdMode {
		if err := authSvc.SeedDevUser(context.Background(), cfg.WorkspaceID, "admin", "admin"); err != nil {
			fmt.Fprintf(os.Stderr, "formspec: warning: seed dev user: %v\n", err)
		}
		// Seed the 4 symmetric owner roles (todo 6.3.4) so there's a
		// baseline to grant from.
		if err := authSvc.SeedOwnerRoles(context.Background(), cfg.WorkspaceID); err != nil {
			fmt.Fprintf(os.Stderr, "formspec: warning: seed owner roles: %v\n", err)
		}
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
	// Wire the async job tracker (todo 7.13): the hub is now available for
	// the `jobs` websocket channel, and the router builder serves the job
	// status polling route.
	jobTracker.SetHub(hub)
	rb.SetJobTracker(jobTracker)

	// Workflow approval store (todo 7.4): persists in-flight approval
	// requests for intercepted state-machine transitions.
	wfApprovalStore := db.NewWorkflowApprovalStore(database, driver)
	rb.SetWorkflowRegistry(wfReg)
	rb.SetWorkflowApprovalStore(wfApprovalStore)
	// Audit writer (todo 7.4.6): records workflow approval decisions as
	// signed statements in the audit trail.
	rb.SetAuditWriter(func(ctx context.Context, workspaceID, entity, entityID, action, actor, changes, requestID string) error {
		return db.WriteAuditLog(ctx, database, driver, workspaceID, entity, entityID, action, actor, changes, requestID)
	})
	// Escalation worker (todo 7.4.4): escalates stale workflow approvals.
	escalationWorker := workflow.NewEscalationWorker(wfApprovalStore, wfReg, func(ctx context.Context, workspaceID, entity, entityID, action, actor, changes, requestID string) error {
		return db.WriteAuditLog(ctx, database, driver, workspaceID, entity, entityID, action, actor, changes, requestID)
	})

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
	// Subscription dispatch (todo 7.3.1): deliver emitted events to matching
	// kind: Subscription handlers via the action dispatcher. Tier 2 durable
	// subscriptions (todo 7.3.2) append to the stream backend instead.
	subDispatch := subscription.NewDispatcher(subReg, disp)
	subDispatch.SetStream(streamBackend)
	// Streaming worker (todo 7.3.2): consumes durable subscriptions from the
	// stream backend with at-least-once, positioned replay, filter/transform,
	// retry and dead-letter.
	streamingWorker := subscription.NewStreamingWorker(subReg, streamBackend, subDispatch)
	// Dynamic subscriptions (todo 7.3.4): runtime-created subscriptions as
	// data in formspec.core.subscription (not manifests). The DynamicSource
	// reads the entity store; the DynamicRefresher merges them into the
	// registry at boot + periodically so admin-panel CRUD takes effect
	// without a restart.
	dynamicSource := func(ctx context.Context, workspaceID string) ([]subscription.DynamicSubscription, error) {
		store, err := reg.GetEntityStore(subscription.CoreModule, "subscription")
		if err != nil {
			return nil, err
		}
		result, err := store.List(ctx, db.ListParams{WorkspaceID: workspaceID, PerPage: 100})
		if err != nil {
			return nil, err
		}
		var out []subscription.DynamicSubscription
		for _, rec := range result.Data {
			if ds, ok := subscription.RecordToSubscription(rec.Data); ok {
				out = append(out, ds)
			}
		}
		return out, nil
	}
	dynamicRefresher := subscription.NewDynamicRefresher(subReg, dynamicSource, cfg.WorkspaceID)
	// Load dynamic subscriptions once at boot (before the first poll).
	if err := dynamicRefresher.Refresh(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "formspec: warning: load dynamic subscriptions: %v\n", err)
	}
	// Integrator dispatch (todo 7.7.1): bridge emitted events to matching
	// kind: Integrator target actions. Saga store (todo 7.7.4) records
	// cross-boundary calls with a declared compensate.
	sagaStore := db.NewSagaStore(database, driver)
	itDispatch := integrator.NewDispatcher(itReg, reg, svcReg, disp, sagaStore)

	// Compose subscription + integrator dispatch into the outbox worker's
	// single Subscriptions callback.
	composedDispatch := func(ctx context.Context, workspaceID, eventName, resource string, payload map[string]any) error {
		var errs []string
		if err := subDispatch.Dispatch(ctx, workspaceID, eventName, resource, payload); err != nil {
			errs = append(errs, err.Error())
		}
		if err := itDispatch.Dispatch(ctx, workspaceID, eventName, resource, payload); err != nil {
			errs = append(errs, err.Error())
		}
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, "; "))
		}
		return nil
	}

	deliveryHandler := &db.DeliveryEventHandler{
		Hub:           hub,
		EventLog:      eventLogStore,
		Lookup:        eventChannelLookup,
		PubSub:        sharedPubSub,
		Subscriptions: composedDispatch,
	}
	outboxWorker := db.NewOutboxWorker(outboxStore, deliveryHandler)

	idempotencyStore := db.NewIdempotencyStore(database, driver).WithTTL(cfg.IdempotencyTTL)
	// Wire the idempotency store into the router so idempotent actions are
	// enforced and the prepare endpoint (todo 2.7) is served.
	rb.SetIdempotencyStore(idempotencyStore)

	app := &App{
		cfg: cfg, database: database, driver: driver,
		reg: reg, rb: rb, disp: disp, nativeEx: nativeEx,
		outboxWorker:     outboxWorker,
		deliveryHandler:  deliveryHandler,
		escalationWorker: escalationWorker,
		pubsub:           sharedPubSub,
		stream:           streamBackend,
		streamingWorker:  streamingWorker,
		dynamicRefresher: dynamicRefresher,
		jobTracker:       jobTracker,
		idempotency:      idempotencyStore,
		nativeHandlers:   make(map[string]action.NativeHandler),
	}
	// Register native handlers for auth entity hooks (password hashing on
	// formspec.core.user create/update).
	app.RegisterNative("formspec.core.user.hash-password", hashUserPassword)
	// specVersionFn must be set before BuildHTTP() so HandleMetaVersion
	// captures it. The closure reads from the live App's specVersion.
	rb.SetSpecVersionFn(func() int64 { return app.specVersion.Load() })

	// ── Observability wiring (todo 8.2) ──
	rb.SetCORSOrigins(cfg.CORSOrigins)
	rb.SetLogger(cfg.Logger)
	rb.SetMetrics(cfg.Metrics)
	rb.SetHealth(cfg.Health)
	if cfg.Health != nil {
		// Datastore probe (todo 8.2.6): datastore_unreachable when the DB
		// does not answer within 2s. Hard failure → unhealthy.
		cfg.Health.Register(observability.ReasonDatastoreUnreachable, func() (string, bool) {
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := database.Ping(pingCtx); err != nil {
				return observability.ReasonDatastoreUnreachable, true
			}
			return "", false
		})
		// DB pool probe (todo 8.2.6): db_pool_exhausted when no connection
		// is idle and all are in use. Degraded (still serving), not hard.
		if sqlDB := database.Driver(); sqlDB != nil {
			cfg.Health.Register(observability.ReasonDBPoolExhausted, func() (string, bool) {
				stats := sqlDB.Stats()
				if stats.OpenConnections > 0 && stats.Idle == 0 && stats.InUse == stats.OpenConnections {
					return observability.ReasonDBPoolExhausted, false
				}
				return "", false
			})
			// Pool gauges (todo 8.2.4: db_pool_open/idle).
			if cfg.Metrics != nil {
				go func() {
					t := time.NewTicker(10 * time.Second)
					defer t.Stop()
					for range t.C {
						s := sqlDB.Stats()
						cfg.Metrics.DBPoolOpen.Set(float64(s.OpenConnections))
						cfg.Metrics.DBPoolIdle.Set(float64(s.Idle))
						cfg.Metrics.DBPoolWaitTotal.Add(float64(s.WaitCount))
					}
				}()
			}
		}
	}

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

// StartBackgroundWorkers starts the outbox worker (background delivery of
// durable events, todo 7.3.1), the workflow escalation worker (todo 7.4.4),
// the subscription streaming worker (todo 7.3.2), and the dynamic-subscription
// refresher (todo 7.3.4). Started explicitly rather than in New() so building
// an App for tests (which typically only call Handler()) never spins up a
// background poller. ListenAndServe calls it automatically; the dev/serve CLI
// commands call it before serving on their own http.Server.
func (a *App) StartBackgroundWorkers() {
	a.outboxWorker.Start(context.Background())
	if a.escalationWorker != nil {
		a.escalationWorker.Start(context.Background())
	}
	if a.streamingWorker != nil {
		a.streamingWorker.Start(context.Background())
	}
	if a.dynamicRefresher != nil {
		a.dynamicRefresher.Start(context.Background())
	}
}

// ListenAndServe starts the HTTP server on cfg.Addr. It also starts the
// outbox worker (background delivery of durable events) — started here
// rather than in New() so building an App for tests (which typically only
// call Handler()) never spins up a background poller.
func (a *App) ListenAndServe() error {
	a.StartBackgroundWorkers()
	// Use the dynamic Handler() wrapper (not a.handler directly) so a
	// ReloadSpec() swap of a.handler takes effect on the live server —
	// otherwise the http.Server keeps serving the boot-time handler with
	// the stale UI registry (hot-reload would "complete" but serve old
	// manifests). `formspec dev` already uses Handler(); this makes the
	// native binaries (formspec-registry) behave the same.
	a.httpServer = &http.Server{Addr: a.cfg.Addr, Handler: a.Handler()}
	return a.httpServer.ListenAndServe()
}

// Close gracefully stops the outbox worker, the escalation worker, the
// streaming worker, the dynamic-subscription refresher, and, if ListenAndServe
// started one, the HTTP server. Safe to call even when ListenAndServe was
// never used — OutboxWorker.Stop()/EscalationWorker.Stop()/
// StreamingWorker.Stop()/DynamicRefresher.Stop() are no-ops if never started,
// and httpServer is nil.
func (a *App) Close(ctx context.Context) error {
	a.outboxWorker.Stop()
	if a.escalationWorker != nil {
		a.escalationWorker.Stop()
	}
	if a.streamingWorker != nil {
		a.streamingWorker.Stop()
	}
	if a.dynamicRefresher != nil {
		a.dynamicRefresher.Stop()
	}
	if a.stream != nil {
		_ = a.stream.Close()
	}
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
	// Mirror startup ordering (NewApp): external/ overrides + embedded
	// framework-owned auth entities (formspec.core.user/session/role/...) are
	// registered BEFORE loading user manifests, so external/ overrides can
	// replace them and the embedded entities survive a hot-reload. Without
	// this, reload drops formspec.core entities while the UI registry still
	// references them (user-table/role-table/role-form) → "entity not found".
	if a.cfg.ExternalDir != "" {
		newReg.AddManifestRoot(a.cfg.ExternalDir)
	}
	if err := auth.RegisterCoreEntities(newReg); err != nil {
		fmt.Fprintf(os.Stderr, "formspec: reload register core entities: %v\n", err)
	}
	if err := subscription.RegisterCoreEntities(newReg); err != nil {
		fmt.Fprintf(os.Stderr, "formspec: reload register subscription core entities: %v\n", err)
	}
	if err := period.RegisterCoreEntities(newReg); err != nil {
		fmt.Fprintf(os.Stderr, "formspec: reload register period core entities: %v\n", err)
	}
	for _, loadErr := range newReg.LoadEntities() {
		fmt.Fprintf(os.Stderr, "formspec: reload: %v\n", loadErr)
	}

	permReg := newReg.GetPermissionRegistry()
	auth.SetPermissionChecker(permission.NewAuthChecker(permReg))

	if _, err := newReg.SyncSchema(context.Background()); err != nil {
		return fmt.Errorf("reload sync schema: %w", err)
	}

	// Re-wire the period-closing guard (todo 7.11.5) on the fresh registry.
	reloadPeriodGuard := period.NewGuard(newReg)
	newReg.SetPeriodGuard(func(ctx context.Context, workspaceID, period string) (bool, error) {
		return reloadPeriodGuard.IsClosed(ctx, workspaceID, period)
	})

	// ── 2. Build fresh UI registry ──
	newUIReg := ui.NewRegistry()
	for _, loadErr := range newUIReg.LoadDir(a.cfg.SpecPath) {
		fmt.Fprintf(os.Stderr, "formspec: reload ui: %v\n", loadErr)
	}
	for _, loadErr := range newUIReg.LoadEmbedded(auth.ModuleFS()) {
		fmt.Fprintf(os.Stderr, "formspec: reload ui (auth module): %v\n", loadErr)
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

	// Config registry (todo 7.2.1): re-resolve on reload so a changed Config
	// manifest's keys take effect without a full restart.
	newCfgReg := buildConfigRegistry(specManifests.Manifests)
	// Service registry (todo 7.1.1): re-resolve on reload.
	newSvcReg := buildServiceRegistry(specManifests.Manifests)
	// Webhook registry (todo 7.6.1): re-resolve on reload.
	newWhReg := buildWebhookRegistry(specManifests.Manifests)
	// Subscription registry (todo 7.3.1): re-resolve on reload.
	newSubReg := buildSubscriptionRegistry(specManifests.Manifests)
	// Workflow registry (todo 7.4.1): re-resolve on reload.
	newWfReg := buildWorkflowRegistry(specManifests.Manifests)
	// Integrator registry (todo 7.7.1): re-resolve on reload.
	newItReg := buildIntegratorRegistry(specManifests.Manifests)
	// Datastore registry (todo 2.9.4): re-resolve on reload so new/changed
	// kind: Datastore manifests and module bindings take effect.
	newDsReg, err := buildDatastoreRegistry(specManifests.Manifests, a.database, stateDirFromDSN(a.cfg.DSN), a.pubsub)
	if err != nil {
		return err
	}

	newDisp := newDispatcher(newReg, newSvcReg, a.database, a.cfg, newCfgReg, a.jobTracker, newDsReg, a.pubsub)

	// Re-register native Go handlers on the new dispatcher.
	a.mu.RLock()
	for ref, h := range a.nativeHandlers {
		newDisp.NativeExecutor().Register(ref, h)
	}
	a.mu.RUnlock()

	// Set the service registry BEFORE BuildRoutes so GenerateServiceRoutes
	// sees it (todo 7.1).
	newRB.SetServiceRegistry(newSvcReg)
	// Set the webhook registry + key resolver BEFORE BuildRoutes (todo 7.6).
	newRB.SetWebhookRegistry(newWhReg)
	newRB.SetWebhookKeyResolver(newCfgReg)
	// Set the workflow registry + approval store BEFORE BuildRoutes (todo 7.4).
	newRB.SetWorkflowRegistry(newWfReg)
	newRB.SetWorkflowApprovalStore(db.NewWorkflowApprovalStore(a.database, a.driver))
	// Audit writer (todo 7.4.6): records workflow approval decisions.
	newRB.SetAuditWriter(func(ctx context.Context, workspaceID, entity, entityID, action, actor, changes, requestID string) error {
		return db.WriteAuditLog(ctx, a.database, a.driver, workspaceID, entity, entityID, action, actor, changes, requestID)
	})
	newRB.BuildRoutes()

	newRB.SetDispatcher(newDisp)
	newRB.SetUIRegistry(newUIReg)
	newRB.SetApps(resolvedApps)
	// Re-resolve the global settings namespace on reload so a changed
	// `settings:` in a Config manifest takes effect without a full restart.
	var declaredSettings *spec.Settings
	for _, raw := range specManifests.Manifests {
		if spec.Kind(raw.Kind) == spec.KindConfig {
			if cs, err := manifest.RawSpecToConfigSpec(raw.Spec.(map[string]any)); err == nil && cs.Settings != nil {
				declaredSettings = cs.Settings
				break
			}
		}
	}
	newRB.SetSettings(spec.ResolveSettings(declaredSettings))
	// Auth on the external surface is opt-in (same as boot).
	newRB.SetEnableAPIAuth(a.cfg.EnableAPIAuth)
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

	// Re-point the outbox worker's subscription + integrator dispatch (todo
	// 7.3.1/7.7.1) to dispatchers built from the freshly reloaded registries —
	// without recreating the worker, so in-flight outbox draining is
	// uninterrupted.
	var newStreamingWorker *subscription.StreamingWorker
	var newDynamicRefresher *subscription.DynamicRefresher
	if a.deliveryHandler != nil {
		newSubDispatch := subscription.NewDispatcher(newSubReg, newDisp)
		newSubDispatch.SetStream(a.stream)
		newItDispatch := integrator.NewDispatcher(newItReg, newReg, newSvcReg, newDisp, db.NewSagaStore(a.database, a.driver))
		a.deliveryHandler.Subscriptions = func(ctx context.Context, workspaceID, eventName, resource string, payload map[string]any) error {
			var errs []string
			if err := newSubDispatch.Dispatch(ctx, workspaceID, eventName, resource, payload); err != nil {
				errs = append(errs, err.Error())
			}
			if err := newItDispatch.Dispatch(ctx, workspaceID, eventName, resource, payload); err != nil {
				errs = append(errs, err.Error())
			}
			if len(errs) > 0 {
				return fmt.Errorf("%s", strings.Join(errs, "; "))
			}
			return nil
		}
		// Rebuild the streaming worker (todo 7.3.2) so new durable
		// subscriptions take effect. The stream backend is reused — its
		// consumer groups and pending entries persist across reloads.
		if a.stream != nil {
			wasRunning := a.streamingWorker != nil && a.streamingWorker.IsRunning()
			if a.streamingWorker != nil {
				a.streamingWorker.Stop()
			}
			newStreamingWorker = subscription.NewStreamingWorker(newSubReg, a.stream, newSubDispatch)
			if wasRunning {
				newStreamingWorker.Start(context.Background())
			}
		}
		// Rebuild the dynamic-subscription refresher (todo 7.3.4) so new
		// dynamic subscriptions take effect. The source reads the freshly
		// reloaded entity registry.
		if a.dynamicRefresher != nil {
			newDynamicSource := func(ctx context.Context, workspaceID string) ([]subscription.DynamicSubscription, error) {
				store, err := newReg.GetEntityStore(subscription.CoreModule, "subscription")
				if err != nil {
					return nil, err
				}
				result, err := store.List(ctx, db.ListParams{WorkspaceID: workspaceID, PerPage: 100})
				if err != nil {
					return nil, err
				}
				var out []subscription.DynamicSubscription
				for _, rec := range result.Data {
					if ds, ok := subscription.RecordToSubscription(rec.Data); ok {
						out = append(out, ds)
					}
				}
				return out, nil
			}
			wasRunning := a.dynamicRefresher.IsRunning()
			a.dynamicRefresher.Stop()
			newDynamicRefresher = subscription.NewDynamicRefresher(newSubReg, newDynamicSource, a.cfg.WorkspaceID)
			if err := newDynamicRefresher.Refresh(context.Background()); err != nil {
				fmt.Fprintf(os.Stderr, "formspec: warning: reload dynamic subscriptions: %v\n", err)
			}
			if wasRunning {
				newDynamicRefresher.Start(context.Background())
			}
		}
	}

	// Wire the idempotency store (todo 2.7) — same store instance, so
	// in-flight keys survive a spec reload.
	newRB.SetIdempotencyStore(a.idempotency)
	// Wire the async job tracker (todo 7.13) — same store + hub, so in-flight
	// jobs survive a spec reload.
	newRB.SetJobTracker(a.jobTracker)

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
	a.streamingWorker = newStreamingWorker
	a.dynamicRefresher = newDynamicRefresher
	a.mu.Unlock()

	a.specVersion.Add(1)

	fmt.Fprintf(os.Stderr, "formspec: reload complete — %d routes, %d entities (v%d)\n",
		len(newRB.Routes()), newReg.Count(), a.specVersion.Load())
	return nil
}

func configureAuth(cfg Config) error {
	// Dev mode without DevAuth: bypass auth entirely (synthetic developer
	// identity with wildcard permissions). DevAuth opts into real JWT auth so
	// authorization behavior can be tested in dev.
	if !cfg.ProdMode && !cfg.DevAuth {
		api.SetAuthValidator(auth.NewDevValidator())
		return nil
	}
	if cfg.JWTSecret == "" && cfg.JWTPublicKeyPath == "" {
		return fmt.Errorf("JWT auth requires JWTSecret or JWTPublicKeyPath")
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

// randomDevSecret generates a random HMAC secret for dev-mode token signing
// when no explicit JWTSecret is configured. Dev-only — ProdMode requires an
// explicit secret or public key.
func randomDevSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a fixed dev secret if the OS entropy source fails
		// (should never happen in practice).
		return "formspec-dev-secret-change-me"
	}
	return hex.EncodeToString(b)
}

// buildConfigRegistry loads kind: Config manifests and resolves their typed
// keys into a config.Registry backing ctx.config (non-secret) and ctx.secrets
// (secret) in the script runtime (todo 7.2.1/7.2.2, 6.8.1). Single-server mode
// has no Control Plane environment resolution, so each key resolves to its
// declared default (spec §10: "spec wajib menetapkan default standar").
func buildConfigRegistry(manifests []manifest.RawManifest) *config.Registry {
	reg := config.NewRegistry()
	for _, raw := range manifests {
		if spec.Kind(raw.Kind) != spec.KindConfig {
			continue
		}
		specMap, ok := raw.Spec.(map[string]any)
		if !ok {
			continue
		}
		cs, err := manifest.RawSpecToConfigSpec(specMap)
		if err != nil {
			continue
		}
		reg.Add(raw.Metadata.Name, cs)
	}
	return reg
}

// buildServiceRegistry loads kind: Service manifests into a service.Registry
// keyed by {module}.{name} (todo 7.1.1). Services are stateless computation
// resources dispatched through the same action dispatcher as entity actions.
func buildServiceRegistry(manifests []manifest.RawManifest) *service.Registry {
	reg := service.NewRegistry()
	for _, raw := range manifests {
		if spec.Kind(raw.Kind) != spec.KindService {
			continue
		}
		specMap, ok := raw.Spec.(map[string]any)
		if !ok {
			continue
		}
		svc, err := manifest.RawSpecToServiceSpec(specMap)
		if err != nil {
			continue
		}
		reg.Add(raw.Metadata.Module, raw.Metadata.Name, svc)
	}
	return reg
}

// buildWebhookRegistry loads kind: Webhook manifests into a webhook.Registry
// keyed by {module}.{name} (todo 7.6.1). Webhooks declare verified inbound
// endpoints that dispatch to a referenced Service action.
func buildWebhookRegistry(manifests []manifest.RawManifest) *webhook.Registry {
	reg := webhook.NewRegistry()
	for _, raw := range manifests {
		if spec.Kind(raw.Kind) != spec.KindWebhook {
			continue
		}
		specMap, ok := raw.Spec.(map[string]any)
		if !ok {
			continue
		}
		wh, err := manifest.RawSpecToWebhookSpec(specMap)
		if err != nil {
			continue
		}
		reg.Add(raw.Metadata.Module, raw.Metadata.Name, wh)
	}
	return reg
}

// buildSubscriptionRegistry loads kind: Subscription manifests into a
// subscription.Registry keyed by {module}.{name} and indexed by event name
// (todo 7.3.1). Subscriptions make one module react to another resource's
// events by dispatching to a referenced Service action.
func buildSubscriptionRegistry(manifests []manifest.RawManifest) *subscription.Registry {
	reg := subscription.NewRegistry()
	for _, raw := range manifests {
		if spec.Kind(raw.Kind) != spec.KindSubscription {
			continue
		}
		specMap, ok := raw.Spec.(map[string]any)
		if !ok {
			continue
		}
		sub, err := manifest.RawSpecToSubscriptionSpec(specMap)
		if err != nil {
			continue
		}
		reg.Add(raw.Metadata.Module, raw.Metadata.Name, sub)
	}
	return reg
}

// buildStreamBackend constructs the Tier 2 durable event-stream backend (todo
// 7.3.2). Resolved via the datastore registry (plan fase E — env-var
// implicit path removed): a module bound to a Redis/Valkey service serving
// `queue`/`pubsub` provides the Redis stream backend; otherwise in-memory
// (dev default, auto-provisioned like the other ctx.* primitives).
//
// The backend is accessed only through the stream.Stream abstraction — never
// directly — so the implementation can be swapped (memory, redis, kafka, ...)
// without touching subscription code.
func buildStreamBackend(dsReg *DatastoreRegistry) (stream.Stream, error) {
	// Probe the registry: any registered service backed by Redis/Valkey
	// serving queue or pubsub can host the durable stream. Deterministic
	// order for reproducibility.
	for _, name := range sortedServiceNames(dsReg) {
		e := dsReg.services[name]
		if e == nil || e.spec == nil {
			continue
		}
		drv := e.spec.Driver
		if drv != spec.DatastoreDriverValkey && drv != spec.DatastoreDriverRedis {
			continue
		}
		servesStream := false
		for _, p := range e.spec.Serves {
			if p == spec.PrimitiveQueue || p == spec.PrimitivePubSub {
				servesStream = true
				break
			}
		}
		if !servesStream {
			continue
		}
		addr := fmt.Sprintf("%s:%d", e.spec.Connection.Host, e.spec.Connection.Port)
		if e.spec.Connection.Host == "" {
			addr = "localhost:6379"
		}
		return stream.NewRedis(addr)
	}
	return stream.NewMemory(), nil
}

// sortedServiceNames returns registry service names in deterministic order
// (helper for buildStreamBackend probing).
func sortedServiceNames(dsReg *DatastoreRegistry) []string {
	dsReg.mu.RLock()
	defer dsReg.mu.RUnlock()
	names := make([]string, 0, len(dsReg.services))
	for name := range dsReg.services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// buildWorkflowRegistry loads kind: Workflow manifests into a workflow.Registry
// keyed by {module}.{name} and indexed by intercepted transition (todo 7.4.1).
// Workflows attach role-based approval to state-machine transitions without
// modifying the Entity.
func buildWorkflowRegistry(manifests []manifest.RawManifest) *workflow.Registry {
	reg := workflow.NewRegistry()
	for _, raw := range manifests {
		if spec.Kind(raw.Kind) != spec.KindWorkflow {
			continue
		}
		specMap, ok := raw.Spec.(map[string]any)
		if !ok {
			continue
		}
		wf, err := manifest.RawSpecToWorkflowSpec(specMap)
		if err != nil {
			continue
		}
		reg.Add(raw.Metadata.Module, raw.Metadata.Name, wf)
	}
	return reg
}

// buildIntegratorRegistry loads kind: Integrator manifests into an
// integrator.Registry keyed by {module}.{name} and indexed by listened event
// (todo 7.7.1). Integrators bridge two entities/modules that do not know each
// other directly: listen.resource+event triggers call.resource+action.
func buildIntegratorRegistry(manifests []manifest.RawManifest) *integrator.Registry {
	reg := integrator.NewRegistry()
	for _, raw := range manifests {
		if spec.Kind(raw.Kind) != spec.KindIntegrator {
			continue
		}
		specMap, ok := raw.Spec.(map[string]any)
		if !ok {
			continue
		}
		it, err := manifest.RawSpecToIntegratorSpec(specMap)
		if err != nil {
			continue
		}
		reg.Add(raw.Metadata.Module, raw.Metadata.Name, it)
	}
	return reg
}

func newDispatcher(reg *entity.Registry, svcReg *service.Registry, _ db.DB, cfg Config, cfgReg *config.Registry, jobTracker *job.Tracker, dsReg *DatastoreRegistry, _ ...*memory.PubSub) *action.Dispatcher {
	disp := action.NewDispatcher()

	scriptEx := action.NewScriptExecutor(cfg.SpecPath)
	// Wire the ctx.* primitive resolver (todo 2.9.1–2.9.4): the closed set of
	// 9 primitives resolves through the DatastoreRegistry — 'default' is
	// auto-provisioned (db → app's primary database, cache/lock/queue/pubsub/
	// kvstore → in-memory, storage → filesystem), named datastores come from
	// kind: Datastore manifests, and per-module `spec.datastore` bindings make
	// ctx.db() module-scoped (platform/06-datastore.md §1.1).
	scriptEx.SetDatastoreResolver(resolverFromRegistry(dsReg))
	// Named logical primitives (plan fase C): ctx.db.named("analytics")
	// resolves via the App Registry named map, gated by uses.datastores.
	scriptEx.SetDatastoreResolverNamed(dsReg)
	// Strict ctx.* primitive enforcement (todo 2.6.4): in ProdMode/StrictMode,
	// a script may only use ctx.* primitives it declared in uses.primitives.
	scriptEx.SetStrictPrimitives(cfg.StrictMode || cfg.ProdMode)
	// ctx.config / ctx.secrets (todo 7.2.2, 6.8.1): resolve Config manifest
	// keys into the script runtime. Non-secret keys back ctx.config.get;
	// secret keys back ctx.secrets (gated by uses.secrets).
	if cfgReg != nil {
		scriptEx.SetConfigStore(cfgReg.NonSecret())
		scriptEx.SetSecretsStore(cfgReg.Secrets())
	}
	// ctx.job.progress (todo 7.13): tracked async jobs report progress to the
	// job tracker, which updates the job row + `jobs` websocket channel.
	if jobTracker != nil {
		scriptEx.SetJobProgressReporter(func(ctx context.Context, workspaceID, jobID string, pct int, message string) error {
			return jobTracker.Progress(ctx, workspaceID, jobID, pct, message)
		})
	}
	scriptEx.SetSaveHandler(func(ctx context.Context, workspaceID, module, entityName, id string, version int, data map[string]any) error {
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return fmt.Errorf("get store: %w", err)
		}
		// resource.new() produces a handle with ID "" — save() on it performs
		// an INSERT (todo 7.14.4). A non-empty ID updates the existing record.
		if id == "" {
			_, err = store.Insert(ctx, db.InsertParams{
				WorkspaceID: workspaceID,
				CreatedBy:   "script",
				Data:        data,
				Permissions: auth.PermissionsFromContext(ctx),
			})
			return err
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
	scriptEx.SetCallHandler(func(ctx context.Context, workspaceID, fromModule, targetModule, targetEntity, actionName string, params map[string]any, callerResources []string) (any, error) {
		if targetModule == "" {
			targetModule = fromModule
		}
		if err := checkCrossModuleUses(fromModule, targetModule, targetEntity, callerResources); err != nil {
			return nil, err
		}
		// Service actions (todo 7.1): resolve {module}.{service} first, then
		// fall back to entity actions. A Service is stateless — no resourceID.
		if svcReg != nil {
			if _, ok := svcReg.Get(targetModule, targetEntity); ok {
				return invokeServiceAction(ctx, svcReg, disp, workspaceID, targetModule, targetEntity, actionName, params)
			}
		}
		return invokeAction(ctx, reg, disp, workspaceID, targetModule, targetEntity, actionName, "", params)
	})
	scriptEx.SetLoadHandler(func(ctx context.Context, workspaceID, fromModule, module, entityName, id string, callerResources []string) (map[string]any, int, error) {
		if err := checkCrossModuleUses(fromModule, module, entityName, callerResources); err != nil {
			return nil, 0, err
		}
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
	scriptEx.SetCreateHandler(func(ctx context.Context, workspaceID, fromModule, module, entityName string, data map[string]any, callerResources []string) (string, error) {
		if err := checkCrossModuleUses(fromModule, module, entityName, callerResources); err != nil {
			return "", err
		}
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

// checkCrossModuleUses enforces the uses.resources contract (01-core-basic §5)
// for cross-resource access from Starlark scripts (todo 2.6.4):
//
//   - same-module or unqualified access is always allowed (a module owns its
//     own resources);
//   - cross-module access is allowed only when the target resource is declared
//     in the caller action's uses.resources — as "{module}.{entity}",
//     "{module}/{entity}", a module wildcard "{module}.*", or "*".
//
// declared is the caller's uses.resources (nil-safe). A violation returns a
// USES_VIOLATION error that aborts the script action.
func checkCrossModuleUses(fromModule, targetModule, targetEntity string, declared []string) error {
	if targetModule == "" || targetModule == fromModule {
		return nil
	}

	declaredRefs := func() map[string]bool {
		m := make(map[string]bool, len(declared))
		for _, d := range declared {
			m[d] = true
		}
		return m
	}
	refs := declaredRefs()

	target := targetModule + "." + targetEntity
	slashTarget := targetModule + "/" + targetEntity
	wildcard := targetModule + ".*"
	if refs[target] || refs[slashTarget] || refs[wildcard] || refs["*"] {
		return nil
	}

	return fmt.Errorf("USES_VIOLATION: undeclared cross-module access to %s from module %s — add it to the action's uses.resources", target, fromModule)
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

// invokeServiceAction dispatches a stateless Service action (todo 7.1.2/7.1.3).
// A Service has no persisted record, so there is no resourceID/resourceData —
// the action runs against the caller's params only. Impl types
// (native/script/script_ref/compiled/sidecar) are resolved by the dispatcher
// exactly as for entity custom actions, so permission/uses enforcement is
// uniform across all five impl types.
func invokeServiceAction(ctx context.Context, svcReg *service.Registry, disp *action.Dispatcher, workspaceID, module, serviceName, actionName string, params map[string]any) (any, error) {
	actionSpec, ok := svcReg.GetAction(module, serviceName, actionName)
	if !ok {
		return nil, fmt.Errorf("resource.call: service action %s.%s.%s not found", module, serviceName, actionName)
	}

	result, err := disp.Dispatch(ctx, *actionSpec, action.ExecuteParams{
		Module:      module,
		Entity:      serviceName,
		ActionName:  actionName,
		Params:      params,
		WorkspaceID: workspaceID,
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
