package spec

import (
	"fmt"
	"strings"
)

// ─── Backend Kind Specs ───

// ServiceSpec defines a stateless, pure computation resource (Core §4.2).
type ServiceSpec struct {
	// @schema {example: "v1"}
	Version string      `yaml:"version" json:"version"`
	Actions []Action    `yaml:"actions" json:"actions"`
	Auth    *EntityAuth `yaml:"auth,omitempty" json:"auth,omitempty"`
}

// ModuleSpec defines a package of manifests (Core §4.3, Ref D19).
type ModuleSpec struct {
	// @schema {example: "1.0.0"}
	Version string `yaml:"version" json:"version"`
	// @schema {example: "acme-corp"}
	Vendor string `yaml:"vendor,omitempty" json:"vendor,omitempty"` // publishing vendor (platform/02-workspace-app-module.md §2)
	// @schema {example: "formspec/core", description: "Module dependencies — array of {module, version?}"}
	Depends []Dependency `yaml:"depends,omitempty" json:"depends,omitempty"`
	// Datastore binds the module to a named kind: Datastore for ctx.db()
	// (platform/06-datastore.md §1.1). Empty = resolve to Datastore 'default'.
	// Legacy single-primitive form — equivalent to datastores: {db: <name>}.
	// @schema {example: "default"}
	Datastore string `yaml:"datastore,omitempty" json:"datastore,omitempty"`
	// Datastores overrides the App-level datastore selection for this module
	// (plan docs/plan/infra-registry-3-level.md fase B). Key is either a
	// primitive type ("db") to set that primitive's default service, or
	// "primitive/alias" ("db/analytics") to register a named logical
	// primitive. Values are registered service names (kind: Datastore).
	// @schema {example: "db: pg-main"}
	Datastores map[string]string `yaml:"datastores,omitempty" json:"datastores,omitempty"`
	Config     map[string]any    `yaml:"config,omitempty" json:"config,omitempty"`     // module-specific configuration (02-workspace-app-module.md §2)
	AiIndex    *AiIndexDecl      `yaml:"ai_index,omitempty" json:"ai_index,omitempty"` // AI discovery metadata (ai/04-formspec-remote-mcp.md §3)
	// Menu is a default navigation suggestion, module-relative (no `Module`
	// field on its items — it's implicitly this module). An App adopts it
	// wholesale via a `type: module` MenuItem (platform/02-workspace-app-module.md §4).
	Menu []MenuItem `yaml:"menu,omitempty" json:"menu,omitempty"`
}

// AiIndexDecl is the optional AI discovery index on a Module
// (ai/04-formspec-remote-mcp.md §3). skills_for_ai is untrusted third-party input.
type AiIndexDecl struct {
	Category           string   `yaml:"category,omitempty" json:"category,omitempty"`
	Features           []string `yaml:"features,omitempty" json:"features,omitempty"`
	Aliases            []string `yaml:"aliases,omitempty" json:"aliases,omitempty"`
	IntegrationPattern string   `yaml:"integration_pattern,omitempty" json:"integration_pattern,omitempty"`
	SkillsForAI        string   `yaml:"skills_for_ai,omitempty" json:"skills_for_ai,omitempty"`
}

// Dependency declares a module dependency.
type Dependency struct {
	Module string `yaml:"module" json:"module"`
	// Version is an optional semver constraint (e.g. ">=1.0 <2.0")
	// (02-workspace-app-module.md §2).
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// AppSpec is the root project manifest (Core §4.4). A Workspace MAY contain
// more than one App; all Apps in a workspace run simultaneously, mounted at
// their own RootURL.
type AppSpec struct {
	// Version is optional — only meaningful when publishing the App to the
	// marketplace (docs/spec/07-marketplace.md); the runtime does not
	// consume it.
	// @schema {example: "1.0.0"}
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	// Vendor is optional — the publishing vendor identity, only required
	// when publishing the App to the marketplace (platform/02 §2).
	// @schema {example: "acme-corp"}
	Vendor string `yaml:"vendor,omitempty" json:"vendor,omitempty"`
	// Title is the human-readable display name (spaces allowed) — used for
	// the shell brand bar, document.title, and menus. metadata.name stays
	// the machine identifier (kebab-case, no spaces).
	// @schema {example: "Acme Corp Portal", description: "Human-readable display name (spaces allowed) — brand bar + document.title; metadata.name stays the machine identifier"}
	Title string `yaml:"title,omitempty" json:"title,omitempty"`
	// Logo is the brand mark shown next to the title in the shell brand bar
	// — a lucide icon name (consistent with menu icons, e.g. "package").
	// @schema {example: "package", description: "Brand mark icon (lucide name) next to the title in the shell brand bar"}
	Logo string `yaml:"logo,omitempty" json:"logo,omitempty"`
	// RootURL is the App's mount prefix inside the workspace: "/" (workspace
	// root) or any "/segment/..." path, unique per workspace. Reserved first
	// segments (_ui, api, _admin, assets, health, login, register, _ws,
	// print) are rejected at resolve time. "app" remains the conventional
	// prefix (docs/plan/flexible-root-url.md).
	// @schema {example: "/app/klinik", pattern: "^(/|/[^/]+(/[^/]+)*)$", description: "Mount prefix inside the workspace: \"/\" or any \"/path\" — unique per workspace; reserved segments (_ui, api, _admin, assets, health, login, register, _ws, print) are rejected"}
	RootURL string `yaml:"root_url" json:"root_url"`
	// @schema {example: "[clinic, pharmacy]", description: "Modules mounted by this App — manifests outside these modules are excluded from the App bundle"}
	Modules []string `yaml:"modules" json:"modules"`
	// Datastores is the App-level App Registry selection — the App Registry
	// (plan docs/plan/infra-registry-3-level.md fase B). Key is either a
	// primitive type ("db") to set that primitive's default service for
	// every module of this App, or "primitive/alias" ("db/analytics") to
	// register a named logical primitive reachable via ctx.db.named()
	// (fase C). Values are registered service names (kind: Datastore).
	// Every App in the workspace may pick different defaults (app1 →
	// pg-main, app2 → pg-analytics). Modules may override per-primitive
	// via ModuleSpec.Datastores.
	// @schema {example: "db: pg-main"}
	Datastores map[string]string `yaml:"datastores,omitempty" json:"datastores,omitempty"`
	// @schema {example: "no-nav", enum: ["sidebar-nav", "topnav", "no-nav"], description: "Chrome archetype (frontend/05-app-kinds.md): sidebar-nav | topnav | no-nav — no-nav means truly no navigation"}
	AppRenderer string `yaml:"app_renderer,omitempty" json:"app_renderer,omitempty"` // chrome archetype (frontend/05-app-kinds.md): sidebar-nav | topnav | no-nav
	// @schema {example: "private", enum: ["private", "public"], description: "Auth axis: private (default, secure by default) | public — orthogonal to app_renderer"}
	Access AppAccess `yaml:"access,omitempty" json:"access,omitempty"` // auth: private (default) | public — orthogonal to app_renderer
	// @schema {example: "react-shadcn", description: "Shell implementation (frontend/03-renderer-kind.md), e.g. react-shadcn"}
	StackFamily string `yaml:"stack_family,omitempty" json:"stack_family,omitempty"` // shell implementation (frontend/03-renderer-kind.md)
	// @schema {example: "jsonb-persist", description: "Entity persist backend (backend/04-persist-backend.md), e.g. jsonb-persist"}
	PersistBackend string `yaml:"persist_backend,omitempty" json:"persist_backend,omitempty"` // entity persist backend (backend/04-persist-backend.md)
	// @schema {example: "ocean-blue", description: "Theme kind name applied per-App (frontend/05-app-kinds.md §6)"}
	ThemeRef string `yaml:"theme_ref,omitempty" json:"theme_ref,omitempty"` // per-App Theme resolution (platform/02 §3)
	// @schema {description: "Per-App auth strategy config (kind: Config)"}
	AuthConfigRef string `yaml:"auth_config_ref,omitempty" json:"auth_config_ref,omitempty"` // per-App auth strategy config
	// Renderers maps a VisualSpecKind name → renderer for the whole App
	// (frontend/03-renderer-kind.md §3): e.g. `{kanban: community/super-kanban}`.
	// Applies to every instance of that kind in the App; individual instances
	// may override via their own `renderer:` field.
	Renderers map[string]string `yaml:"renderers,omitempty" json:"renderers,omitempty"`
	// Chrome fine-tunes which shell chrome elements render (frontend/
	// 05-app-kinds.md §5) — orthogonal to app_renderer (layout archetype)
	// and access (auth axis). Every element defaults to "auto", meaning the
	// archetype's own default; explicit values override. Resolved to effective
	// values by the meta API — renderers never guess.
	// @schema {example: "nav: menu", description: "Chrome composition: brand/nav/auth/footer/breadcrumbs/theme_switcher, each auto|show|hide (auth: auto|links|button|none) — see frontend/05-app-kinds.md §5"}
	Chrome    *AppChrome     `yaml:"chrome,omitempty" json:"chrome,omitempty"`
	Menu      []MenuItem     `yaml:"menu,omitempty" json:"menu,omitempty"`
	Publishes []AppInterface `yaml:"publishes,omitempty" json:"publishes,omitempty"` // cross-app interfaces offered
	Consumes  []AppConsume   `yaml:"consumes,omitempty" json:"consumes,omitempty"`   // cross-app interfaces needed → grant request
}

// Chrome element values (frontend/05-app-kinds.md §4.1). "auto" means the
// archetype's own default; the rest are explicit overrides.
const (
	ChromeAuto   = "auto"
	ChromeShow   = "show"
	ChromeHide   = "hide"
	ChromeMenu   = "menu"
	ChromeNone   = "none"
	ChromeLinks  = "links"
	ChromeButton = "button"
)

// AppChrome fine-tunes which chrome elements the App shell renders
// (frontend/05-app-kinds.md §4.1). Orthogonal to AppRenderer (layout
// archetype) and Access (auth axis): every element defaults to "auto" —
// the archetype's own default — and explicit values override it. The meta
// API resolves the effective composition (internal/ui resolveChrome) so
// renderers read final values.
type AppChrome struct {
	// Brand bar — App title + logo.
	// @schema {example: "auto", enum: ["auto", "show", "hide"]}
	Brand string `yaml:"brand,omitempty" json:"brand,omitempty"`
	// Navigation links derived from the resolved App menu.
	// @schema {example: "auto", enum: ["auto", "menu", "none"]}
	Nav string `yaml:"nav,omitempty" json:"nav,omitempty"`
	// Auth controls. "links": Sign in link + Sign up button (anon) /
	// logout (signed-in). "button": single Sign in button. "none": no auth
	// UI at all — private Apps still guard via the surface boot redirect.
	// @schema {example: "auto", enum: ["auto", "links", "button", "none"]}
	Auth string `yaml:"auth,omitempty" json:"auth,omitempty"`
	// Page footer.
	// @schema {example: "auto", enum: ["auto", "show", "hide"]}
	Footer string `yaml:"footer,omitempty" json:"footer,omitempty"`
	// Breadcrumb row (sidebar-nav/topnav).
	// @schema {example: "auto", enum: ["auto", "show", "hide"]}
	Breadcrumbs string `yaml:"breadcrumbs,omitempty" json:"breadcrumbs,omitempty"`
	// Theme switcher control (sidebar-nav/topnav).
	// @schema {example: "auto", enum: ["auto", "show", "hide"]}
	ThemeSwitcher string `yaml:"theme_switcher,omitempty" json:"theme_switcher,omitempty"`
}

// AppInterface is one cross-app service interface offered by an App
// (02-workspace-app-module.md §3).
type AppInterface struct {
	Service string   `yaml:"service" json:"service"`
	Actions []string `yaml:"actions,omitempty" json:"actions,omitempty"`
}

// AppConsume is one cross-app interface an App depends on; it triggers a
// grant request to the owning App (02-workspace-app-module.md §3).
type AppConsume struct {
	App     string   `yaml:"app" json:"app"`
	Service string   `yaml:"service" json:"service"`
	Actions []string `yaml:"actions,omitempty" json:"actions,omitempty"`
}

// ─── 1.2 Meta-Kind Structs (platform/03-kind-system.md §2) ───

// VisualSpecKindSpec declares a new view type — a meta-kind that defines the
// schema and renderer contract for a family of page/component instances
// (frontend/02-visual-spec-kind.md).
type VisualSpecKindSpec struct {
	Tier             string     `yaml:"tier" json:"tier"`                         // app | page | component
	Schema           any        `yaml:"schema,omitempty" json:"schema,omitempty"` // JSON Schema for instance spec
	RendererContract any        `yaml:"renderer_contract,omitempty" json:"renderer_contract,omitempty"`
	AcceptsSlots     []SlotDecl `yaml:"accepts_slots,omitempty" json:"accepts_slots,omitempty"`     // tier: page/app only
	ImplementsSlot   string     `yaml:"implements_slot,omitempty" json:"implements_slot,omitempty"` // tier: component only
}

// SlotDecl declares a named slot with its data contract (02-visual-spec-kind.md §4).
type SlotDecl struct {
	Name     string        `yaml:"name" json:"name"`
	Contract *SlotContract `yaml:"contract,omitempty" json:"contract,omitempty"`
}

// SlotContract defines the props contract for a slot.
type SlotContract struct {
	RequiredProps []string `yaml:"required_props,omitempty" json:"required_props,omitempty"`
	OptionalProps []string `yaml:"optional_props,omitempty" json:"optional_props,omitempty"`
}

// RendererSpec declares a concrete implementation of a VisualSpecKind
// (frontend/03-renderer-kind.md).
type RendererSpec struct {
	// @schema {example: "formspec/visual.form-page"}
	Implements string `yaml:"implements" json:"implements"`
	// @schema {example: "react-shadcn"}
	StackFamily string `yaml:"stack_family" json:"stack_family"` // e.g. react-shadcn, vue, flutter
	// @schema {example: "official", enum: ["official", "verified", "community"]}
	TrustTier string `yaml:"trust_tier" json:"trust_tier"` // official | verified | community
}

// PersistBackendSpec declares a storage seam implementation
// (backend/04-persist-backend.md).
type PersistBackendSpec struct {
	// @schema {example: "formspec/storage.entity-persist"}
	Implements string `yaml:"implements" json:"implements"` // storage backend name
	// @schema {example: "official", enum: ["official", "verified", "community"]}
	TrustTier string `yaml:"trust_tier" json:"trust_tier"` // official | verified | community
}

// ─── MenuItem ───

// MenuItem is one navigation entry, embedded in App.spec.menu or
// Module.spec.menu (Core §4.4/§4.5) — there is no standalone kind: Menu.
// Menu order follows array position — no separate `order` field.
//
// Every item is exactly one of three shapes, enforced by the loader/resolver
// (see internal/manifest), not by separate Go types:
//
//   - Adopt node (Type == "module"): only Module set.
//     Splices that Module's own Menu wholesale at this position. Level 1 only.
//   - Group node (len(Children) > 0): Label + Children. Module/View/Route
//     forbidden on the group itself — only its descendants carry Module.
//   - Leaf node (no Children, Type != "module"): Label + Module + exactly one
//     of View/Route. Level 3 leaves cannot have Children (3-level cap).
type MenuItem struct {
	Type     string     `yaml:"type,omitempty" json:"type,omitempty"` // "module" = adopt-shorthand node
	Label    string     `yaml:"label,omitempty" json:"label,omitempty"`
	Icon     string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Module   string     `yaml:"module,omitempty" json:"module,omitempty"`
	View     string     `yaml:"view,omitempty" json:"view,omitempty"`   // name of a registered View resource
	Route    string     `yaml:"route,omitempty" json:"route,omitempty"` // raw URL escape hatch (no registered View)
	When     string     `yaml:"when,omitempty" json:"when,omitempty"`   // FormSpecExpr business condition
	Children []MenuItem `yaml:"children,omitempty" json:"children,omitempty"`
}

// ─── 1.1.9 ConfigSpec — structured ConfigKey ───

// ConfigSpec defines runtime configuration values (Core §4.5, §10).
// Keys are structured — each key declares its type, default value, and
// whether it is a secret (never inlined in YAML, resolved per environment).
// Scripts read values via ctx.config.get("key").
// ConfigSpec declares module-level configuration keys (01-core-basic.md §10).
//
// `settings` is the typed, global presentation/config namespace (spec §10 —
// "jangan pernah menebak"). It lives in ONE place (workspace/App Config) and
// drives cross-component interpretation/display: currency, locale, timezone,
// date format, decimal scale, rounding. Renderers read it instead of guessing.
type ConfigSpec struct {
	// @schema {example: "{invoice_due_days: {type: int, default: 30}, smtp_host: {type: string, secret: true}}"}
	Keys map[string]ConfigKey `yaml:"keys,omitempty" json:"keys,omitempty"`
	// Settings is the typed global presentation/config namespace (spec §10).
	// Declared as `settings:` on a workspace/App Config manifest.
	Settings *Settings `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// ConfigKey declares one configuration entry (01-core-basic.md §10).
type ConfigKey struct {
	Type    string `yaml:"type" json:"type"` // int | string | bool | decimal | json
	Default any    `yaml:"default,omitempty" json:"default,omitempty"`
	Secret  bool   `yaml:"secret,omitempty" json:"secret,omitempty"`
}

// ─── Global Settings (spec §10 — "jangan pernah menebak") ───

// Settings is the typed global presentation/config namespace. It is declared
// once on a workspace/App `kind: Config` manifest under `settings:` and
// resolved with standard defaults so behavior is consistent across every
// component even when unset. Renderers/backends read these values instead of
// guessing per component (01-core-basic.md §10, 05-field-types.md §2).
type Settings struct {
	// Currency is the default money currency for the workspace (ISO-4217 code,
	// minor-unit scale, and optional display symbol). Money fields inherit it
	// unless they override `currency` explicitly (05-field-types.md §2).
	Currency *CurrencySettings `yaml:"currency,omitempty" json:"currency,omitempty"`
	// Locale is the IETF BCP-47 locale used for number/date formatting
	// (e.g. "en-US", "id-ID"). Default "en-US".
	Locale string `yaml:"locale,omitempty" json:"locale,omitempty"`
	// Timezone is the IANA timezone name (e.g. "UTC", "Asia/Jakarta").
	// Default "UTC".
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`
	// DateFormat is the display date format (e.g. "YYYY-MM-DD", "DD/MM/YYYY").
	// Default "YYYY-MM-DD" (ISO-8601).
	DateFormat string `yaml:"date_format,omitempty" json:"date_format,omitempty"`
	// DecimalScale is the default number of digits after the decimal point for
	// `decimal` fields that don't declare their own `scale`. Default 2.
	DecimalScale int `yaml:"decimal_scale,omitempty" json:"decimal_scale,omitempty"`
	// Rounding is the default rounding mode for money/decimal arithmetic:
	// "half_even" (banker's, default) | "half_up" | "half_down" | "up" | "down".
	Rounding string `yaml:"rounding,omitempty" json:"rounding,omitempty"`
}

// CurrencySettings configures the default money currency (05-field-types.md §2).
type CurrencySettings struct {
	// Code is the ISO-4217 currency code (e.g. "IDR", "USD").
	Code string `yaml:"code,omitempty" json:"code,omitempty"`
	// DecimalPlaces is the minor-unit scale of this currency (e.g. IDR=0, USD=2).
	// Pointer so `0` (no minor units) is distinguishable from "unset".
	DecimalPlaces *int `yaml:"decimal_places,omitempty" json:"decimal_places,omitempty"`
	// Symbol is the optional display symbol (e.g. "Rp", "$"). When empty, the
	// renderer derives it from the locale/currency via Intl.
	Symbol string `yaml:"symbol,omitempty" json:"symbol,omitempty"`
}

// DefaultSettings returns the standard defaults for the global settings
// namespace (spec §10 — every setting has a widely-accepted default so
// behavior is consistent even when unset).
func DefaultSettings() *Settings {
	two := 2
	return &Settings{
		Currency: &CurrencySettings{
			Code:          "USD",
			DecimalPlaces: &two,
		},
		Locale:       "en-US",
		Timezone:     "UTC",
		DateFormat:   "YYYY-MM-DD",
		DecimalScale: 2,
		Rounding:     "half_even",
	}
}

// ResolveSettings overlays the declared settings onto the standard defaults,
// returning a fully-populated Settings with no empty fields.
func ResolveSettings(declared *Settings) *Settings {
	d := DefaultSettings()
	if declared == nil {
		return d
	}
	if declared.Currency != nil {
		if declared.Currency.Code != "" {
			d.Currency.Code = declared.Currency.Code
		}
		if declared.Currency.DecimalPlaces != nil {
			d.Currency.DecimalPlaces = declared.Currency.DecimalPlaces
		}
		if declared.Currency.Symbol != "" {
			d.Currency.Symbol = declared.Currency.Symbol
		}
	}
	if declared.Locale != "" {
		d.Locale = declared.Locale
	}
	if declared.Timezone != "" {
		d.Timezone = declared.Timezone
	}
	if declared.DateFormat != "" {
		d.DateFormat = declared.DateFormat
	}
	if declared.DecimalScale != 0 {
		d.DecimalScale = declared.DecimalScale
	}
	if declared.Rounding != "" {
		d.Rounding = declared.Rounding
	}
	return d
}

// ─── 1.1.8 SubscriptionSpec — Tier 2 fields ───

// SubscriptionSpec subscribes to another resource's events (Core §4.6, Ref D35,
// 02-core-extended.md §3).
//
// Tier 1 (Core, outbox): Events + Handler only.
// Tier 2 (Streaming): adds store, retention, position, max_retry, dead_letter,
// filter, transform, and explicit delivery channel.
type SubscriptionSpec struct {
	// @schema {example: "billing.invoice.on_submit"}
	Events  []string `yaml:"events" json:"events"`
	Handler ImplDecl `yaml:"handler" json:"handler"`
	// @schema {example: "redis"}
	Store   string     `yaml:"store,omitempty" json:"store,omitempty"`           // Tier 2: stream backend (redis, kafka)
	Durable string     `yaml:"durability,omitempty" json:"durability,omitempty"` // Tier 2: durability mode ("durable" = streaming)
	Retry   *RetryDecl `yaml:"retry,omitempty" json:"retry,omitempty"`           // Tier 2
	// @schema {example: "latest", enum: ["latest", "earliest"]}
	Position   string           `yaml:"position,omitempty" json:"position,omitempty"`       // Tier 2: latest | earliest | <id>
	Filter     string           `yaml:"filter,omitempty" json:"filter,omitempty"`           // Tier 2: Starlark filter over event payload
	Transform  string           `yaml:"transform,omitempty" json:"transform,omitempty"`     // Tier 2: Starlark transform over event payload
	DeadLetter *DeliveryTarget  `yaml:"dead_letter,omitempty" json:"dead_letter,omitempty"` // Tier 2
	MaxRetry   int              `yaml:"max_retry,omitempty" json:"max_retry,omitempty"`     // Tier 2
	Retention  string           `yaml:"retention,omitempty" json:"retention,omitempty"`     // Tier 2: stream retention duration
	Delivery   *SubDeliveryDecl `yaml:"delivery,omitempty" json:"delivery,omitempty"`       // Tier 2: delivery channel
}

// SubDeliveryDecl configures the delivery channel for a subscription (02-core-extended.md §3).
type SubDeliveryDecl struct {
	Channel string `yaml:"channel" json:"channel"`                       // webhook | notification | pubsub | queue
	URLFrom string `yaml:"url_from,omitempty" json:"url_from,omitempty"` // header | config
	Header  string `yaml:"header,omitempty" json:"header,omitempty"`     // header name for url_from: header
	Sign    bool   `yaml:"sign,omitempty" json:"sign,omitempty"`
}

// ─── 1.1.1 MigrationSpec ───

// MigrationSpec defines a custom DDL migration (01-core-basic.md §4).
// Only DDL statements are allowed — DML is rejected at runtime.
type MigrationSpec struct {
	// @schema {example: "CREATE INDEX idx_invoice_date ON invoice(transaction_date)"}
	DDL string `yaml:"ddl" json:"ddl"`
	// @schema {example: "billing"}
	Module string `yaml:"module,omitempty" json:"module,omitempty"` // owning module for table-level DDL
}

// ─── 4.2.5 DataMigrationSpec ───

// DataMigrationSpec defines a versioned data migration (backfill) with a run
// script and an optional rollback script (01-core-basic.md §4, type 3).
// Distinct from structural diff (automatic) and custom DDL (kind: Migration).
type DataMigrationSpec struct {
	// Version is the migration version (increments per migration).
	Version int `yaml:"version" json:"version"`
	// Run is the Starlark script ref that performs the backfill.
	Run string `yaml:"run" json:"run"`
	// Rollback is an optional Starlark script ref that reverses the backfill.
	Rollback string `yaml:"rollback,omitempty" json:"rollback,omitempty"`
	// Module is the owning module.
	Module string `yaml:"module,omitempty" json:"module,omitempty"`
}

// ─── 1.1.2 WorkflowSpec ───

// WorkflowSpec defines an approval workflow attached to a state machine
// transition (02-core-extended.md §2).
type WorkflowSpec struct {
	// @schema {example: "gl.journal-entry"}
	Entity     string              `yaml:"entity" json:"entity"`
	On         *WorkflowTrigger    `yaml:"on" json:"on"`
	Steps      []WorkflowStep      `yaml:"steps" json:"steps"`
	OnReject   *WorkflowReject     `yaml:"on_reject,omitempty" json:"on_reject,omitempty"`
	Escalation *WorkflowEscalation `yaml:"escalation,omitempty" json:"escalation,omitempty"`
}

// WorkflowTrigger declares which state machine transition this workflow
// intercepts.
type WorkflowTrigger struct {
	Transition *WorkflowTransitionRef `yaml:"transition,omitempty" json:"transition,omitempty"`
}

// WorkflowTransitionRef identifies the intercepted transition by its
// from/to states.
type WorkflowTransitionRef struct {
	// @schema {example: "draft"}
	From string `yaml:"from" json:"from"`
	// @schema {example: "posted"}
	To string `yaml:"to" json:"to"`
}

// WorkflowStep is one approval step in the workflow chain.
// Steps are evaluated sequentially; each must reach quorum before the next begins.
type WorkflowStep struct {
	// @schema {example: "[gl.supervisor]"}
	Roles      []string        `yaml:"roles" json:"roles"`
	Approvers  int             `yaml:"approvers,omitempty" json:"approvers,omitempty"` // quorum, default 1
	Mode       string          `yaml:"mode,omitempty" json:"mode,omitempty"`           // all | any | sequential
	When       string          `yaml:"when,omitempty" json:"when,omitempty"`           // FormSpecExpr — skip step if false
	Escalation *StepEscalation `yaml:"escalation,omitempty" json:"escalation,omitempty"`
}

// StepEscalation configures timeout and reassignment for one step.
type StepEscalation struct {
	After         string   `yaml:"after" json:"after"` // duration e.g. "48h"
	NotifyRoles   []string `yaml:"notify_roles,omitempty" json:"notify_roles,omitempty"`
	ReassignRoles []string `yaml:"reassign_roles,omitempty" json:"reassign_roles,omitempty"`
}

// WorkflowReject declares the target state when the workflow is rejected.
type WorkflowReject struct {
	To string `yaml:"to" json:"to"`
}

// WorkflowEscalation defines global escalation for the entire workflow.
type WorkflowEscalation struct {
	After       string   `yaml:"after" json:"after"`
	NotifyRoles []string `yaml:"notify_roles,omitempty" json:"notify_roles,omitempty"`
}

// ─── 1.1.3 ApiSpec ───

// ApiSpec overrides the external API surface (/api/v1/) for a module's entities
// (02-core-extended.md §12). It does not affect the UI surface (/_ui/entity/).
type ApiSpec struct {
	REST *ApiRESTConfig `yaml:"rest,omitempty" json:"rest,omitempty"`
	GRPC *ApiGRPCConfig `yaml:"grpc,omitempty" json:"grpc,omitempty"`
}

// ApiRESTConfig configures the REST external surface.
type ApiRESTConfig struct {
	// @schema {example: "/invoices"}
	BasePath string `yaml:"base_path,omitempty" json:"base_path,omitempty"` // replaces {module} in route
	// @schema {example: "v2"}
	Version string   `yaml:"version,omitempty" json:"version,omitempty"` // override {version} route
	Disable []string `yaml:"disable,omitempty" json:"disable,omitempty"` // entities to opt-out
}

// ApiGRPCConfig configures the gRPC external surface.
type ApiGRPCConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Package string `yaml:"package,omitempty" json:"package,omitempty"`
}

// ─── 1.1.4 WebhookSpec ───

// WebhookSpec defines a verified inbound webhook endpoint
// (02-core-extended.md §4).
type WebhookSpec struct {
	// @schema {example: "billing.invoice"}
	For string `yaml:"for" json:"for"`
	// @schema {example: "POST", enum: ["GET", "POST", "PUT", "PATCH", "DELETE"]}
	Method string `yaml:"method" json:"method"`
	// @schema {example: "/webhooks/midtrans"}
	Path           string           `yaml:"path,omitempty" json:"path,omitempty"`
	Auth           *WebhookAuth     `yaml:"auth" json:"auth"`
	Idempotent     bool             `yaml:"idempotent,omitempty" json:"idempotent,omitempty"`
	IdempotencyKey *IdempotencyDecl `yaml:"idempotency_key,omitempty" json:"idempotency_key,omitempty"`
}

// WebhookAuth declares how inbound webhook payloads are verified.
// strategy: signature | token
type WebhookAuth struct {
	Strategy  string            `yaml:"strategy" json:"strategy"` // signature | token
	Signature *WebhookSigConfig `yaml:"signature,omitempty" json:"signature,omitempty"`
	// Token configures static-token verification (strategy: token). The token
	// value is referenced via config (never inline in the manifest).
	Token *WebhookTokenConfig `yaml:"token,omitempty" json:"token,omitempty"`
}

// WebhookTokenConfig configures static-token verification for simple internal
// webhooks (02-core-extended.md §4). The token is referenced via config so it
// never appears inline in the manifest.
type WebhookTokenConfig struct {
	// Header is the request header carrying the token (e.g. "Authorization").
	Header string `yaml:"header" json:"header"`
	// Key references the config key holding the expected token value.
	Key *WebhookKeyRef `yaml:"key" json:"key"`
}

// WebhookSigConfig configures HMAC signature verification.
type WebhookSigConfig struct {
	Algorithm string         `yaml:"algorithm" json:"algorithm"`                 // e.g. hmac-sha512
	Header    string         `yaml:"header" json:"header"`                       // signature header name
	Key       *WebhookKeyRef `yaml:"key" json:"key"`                             // config reference for the secret key
	Payload   string         `yaml:"payload,omitempty" json:"payload,omitempty"` // raw_body | parsed
}

// WebhookKeyRef references the secret key via config.
type WebhookKeyRef struct {
	Config string `yaml:"config" json:"config"`
	Secret bool   `yaml:"secret,omitempty" json:"secret,omitempty"`
}

// ─── 1.1.5 IntegratorSpec ───

// IntegratorSpec bridges two entities/modules that do not know each other
// directly (02-core-extended.md §5).
type IntegratorSpec struct {
	Listen *IntegratorListen `yaml:"listen" json:"listen"`
	Call   *IntegratorCall   `yaml:"call" json:"call"`
	// @schema {example: "gl.journal-entry.cancel"}
	Compensate string `yaml:"compensate,omitempty" json:"compensate,omitempty"`
}

// IntegratorListen declares the source event to react to.
type IntegratorListen struct {
	// @schema {example: "sales.order"}
	Resource string `yaml:"resource" json:"resource"`
	// @schema {example: "on_submit"}
	Event string `yaml:"event" json:"event"`
}

// IntegratorCall declares the target action to invoke.
type IntegratorCall struct {
	// @schema {example: "gl.journal-entry"}
	Resource string `yaml:"resource" json:"resource"`
	// @schema {example: "create"}
	Action string `yaml:"action" json:"action"`
}

// ─── 1.1.6 MockupSpec ───

// MockupSpec defines a simulated connector that implements the same contract
// as the real integration (02-core-extended.md §8).
type MockupSpec struct {
	// @schema {example: "billing.payment-gateway"}
	For       string `yaml:"for" json:"for"` // module.service or module.entity
	ConfigRef string `yaml:"config_ref,omitempty" json:"config_ref,omitempty"`
}

// ─── 1.1.7 KindDefinitionSpec ───

// KindDefinitionSpec declares a new resource kind (CRD-like extension),
// including its schema and handler (platform/03-kind-system.md §2).
type KindDefinitionSpec struct {
	// @schema {example: "seed.formspec.dev"}
	Group string `yaml:"group" json:"group"`
	// @schema {example: "v1"}
	Version string    `yaml:"version" json:"version"`
	Schema  any       `yaml:"schema,omitempty" json:"schema,omitempty"`
	Handler *ImplDecl `yaml:"handler" json:"handler"`
	// @schema {example: "module", enum: ["module", "app"]}
	Scope string `yaml:"scope,omitempty" json:"scope,omitempty"` // module | app
}

// AppRendererNames is the closed set of App renderer archetypes
// (frontend/05-app-kinds.md §1). `sidebar-nav` is the default when
// App.spec.app_renderer is omitted. An archetype describes the chrome shape
// only — public/private auth is a separate axis (AppSpec.Access).
var AppRendererNames = map[string]bool{
	"sidebar-nav": true,
	"topnav":      true,
	"no-nav":      true,
}

// DefaultAppRenderer is applied when App.spec.app_renderer is empty.
const DefaultAppRenderer = "sidebar-nav"

// AppAccess controls whether an App's surface is publicly reachable without
// authentication (frontend/05-app-kinds.md §1). `private` is the
// secure-by-default. Orthogonal to the App renderer archetype: any of
// sidebar-nav/topnav/no-nav may be public or private.
type AppAccess string

const (
	AppAccessPrivate AppAccess = "private"
	AppAccessPublic  AppAccess = "public"
)

// DefaultStackFamily is the only installed shell implementation today
// (frontend/03-renderer-kind.md). More shells (flutter, react-mui) register
// later; full renderer resolution is tracked in todo 5.16.
const DefaultStackFamily = "react-shadcn"

// DefaultPersistBackend is the official entity persist backend
// (backend/04-persist-backend.md). It implements EntityPersistContract.
const DefaultPersistBackend = "jsonb-persist"

// EntityPersistContract is the storage contract every entity persist backend
// must implement (backend/04-persist-backend.md).
const EntityPersistContract = "formspec/storage.entity-persist"

// InstalledPersistBackends is the set of entity persist backends wired into
// this engine build (backend/04-persist-backend.md). Adding a backend means
// registering it here AND wiring its driver; ValidateAppSpec rejects any
// name outside this set — swapping to a backend that isn't installed or
// doesn't implement the storage contract is a hard error.
var InstalledPersistBackends = map[string]bool{
	DefaultPersistBackend: true,
}

// ValidateAppSpec validates an AppSpec, returning an error if any constraint
// is violated. Enforces:
//   - app_renderer, when set, must be a known App renderer (closed set).
//   - access, when set, must be private|public.
//   - persist_backend, when set, must be an installed backend — swapping to a
//     backend that doesn't exist / isn't compatible with the storage contract
//     is a hard error (not a warning).
//   - chrome values, when set, must be from their enum (frontend/
//     05-app-kinds.md §4.1).
func ValidateAppSpec(a *AppSpec) error {
	if a.AppRenderer != "" && !AppRendererNames[a.AppRenderer] {
		return fmt.Errorf("app_renderer %q is not a known App renderer (closed set: sidebar-nav, topnav, no-nav)", a.AppRenderer)
	}
	if a.Access != "" && a.Access != AppAccessPrivate && a.Access != AppAccessPublic {
		return fmt.Errorf("access %q is invalid (enum: private, public)", a.Access)
	}
	if a.PersistBackend != "" && !InstalledPersistBackends[a.PersistBackend] {
		return fmt.Errorf("persist_backend %q is not installed (installed: %s — implements %s)", a.PersistBackend, DefaultPersistBackend, EntityPersistContract)
	}
	if c := a.Chrome; c != nil {
		if err := validateChromeValue("chrome.brand", c.Brand, ChromeAuto, ChromeShow, ChromeHide); err != nil {
			return err
		}
		if err := validateChromeValue("chrome.nav", c.Nav, ChromeAuto, ChromeMenu, ChromeNone); err != nil {
			return err
		}
		if err := validateChromeValue("chrome.auth", c.Auth, ChromeAuto, ChromeLinks, ChromeButton, ChromeNone); err != nil {
			return err
		}
		if err := validateChromeValue("chrome.footer", c.Footer, ChromeAuto, ChromeShow, ChromeHide); err != nil {
			return err
		}
		if err := validateChromeValue("chrome.breadcrumbs", c.Breadcrumbs, ChromeAuto, ChromeShow, ChromeHide); err != nil {
			return err
		}
		if err := validateChromeValue("chrome.theme_switcher", c.ThemeSwitcher, ChromeAuto, ChromeShow, ChromeHide); err != nil {
			return err
		}
	}
	return nil
}

// validateChromeValue reports an error when val is non-empty and not one of
// the allowed enum values.
func validateChromeValue(field, val string, allowed ...string) error {
	if val == "" {
		return nil
	}
	for _, a := range allowed {
		if val == a {
			return nil
		}
	}
	return fmt.Errorf("%s %q is invalid (enum: %s)", field, val, strings.Join(allowed, ", "))
}
