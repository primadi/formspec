package spec

// ─── Backend Kind Specs ───

// ServiceSpec defines a stateless, pure computation resource (Core §4.2).
type ServiceSpec struct {
	Version string      `yaml:"version" json:"version"`
	Actions []Action    `yaml:"actions" json:"actions"`
	Auth    *EntityAuth `yaml:"auth,omitempty" json:"auth,omitempty"`
}

// ModuleSpec defines a package of manifests (Core §4.3, Ref D19).
type ModuleSpec struct {
	Version string       `yaml:"version" json:"version"`
	Vendor  string       `yaml:"vendor,omitempty" json:"vendor,omitempty"`     // publishing vendor (platform/02-workspace-app-module.md §2)
	Depends []Dependency `yaml:"depends,omitempty" json:"depends,omitempty"`
	// Datastore binds the module to a named kind: Datastore for ctx.db()
	// (platform/06-datastore.md §1.1). Empty = resolve to Datastore 'default'.
	Datastore string         `yaml:"datastore,omitempty" json:"datastore,omitempty"`
	Config    map[string]any `yaml:"config,omitempty" json:"config,omitempty"`     // module-specific configuration (02-workspace-app-module.md §2)
	AiIndex   *AiIndexDecl    `yaml:"ai_index,omitempty" json:"ai_index,omitempty"` // AI discovery metadata (ai/04-forma-remote-mcp.md §3)
	// Menu is a default navigation suggestion, module-relative (no `Module`
	// field on its items — it's implicitly this module). An App adopts it
	// wholesale via a `type: module` MenuItem (platform/02-workspace-app-module.md §4).
	Menu []MenuItem `yaml:"menu,omitempty" json:"menu,omitempty"`
}

// AiIndexDecl is the optional AI discovery index on a Module
// (ai/04-forma-remote-mcp.md §3). skills_for_ai is untrusted third-party input.
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
	Version        string         `yaml:"version" json:"version"`
	Vendor         string         `yaml:"vendor" json:"vendor"`
	RootURL        string         `yaml:"root_url" json:"root_url"`
	Modules        []string       `yaml:"modules" json:"modules"`
	AppRenderer    string         `yaml:"app_renderer,omitempty" json:"app_renderer,omitempty"` // named Renderer tier app (frontend/05-app-kinds.md)
	ThemeRef       string         `yaml:"theme_ref,omitempty" json:"theme_ref,omitempty"`       // per-App Theme resolution (platform/02 §3)
	AuthConfigRef  string         `yaml:"auth_config_ref,omitempty" json:"auth_config_ref,omitempty"` // per-App auth strategy config
	Menu           []MenuItem     `yaml:"menu,omitempty" json:"menu,omitempty"`
	Publishes      []AppInterface `yaml:"publishes,omitempty" json:"publishes,omitempty"`       // cross-app interfaces offered
	Consumes       []AppConsume   `yaml:"consumes,omitempty" json:"consumes,omitempty"`         // cross-app interfaces needed → grant request
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
	Implements  string `yaml:"implements" json:"implements"`
	StackFamily string `yaml:"stack_family" json:"stack_family"` // e.g. react-shadcn, vue, flutter
	TrustTier   string `yaml:"trust_tier" json:"trust_tier"`     // official | verified | community
}

// PersistBackendSpec declares a storage seam implementation
// (backend/04-persist-backend.md).
type PersistBackendSpec struct {
	Implements string `yaml:"implements" json:"implements"` // storage backend name
	TrustTier  string `yaml:"trust_tier" json:"trust_tier"` // official | verified | community
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
	When     string     `yaml:"when,omitempty" json:"when,omitempty"`   // FormaExpr business condition
	Children []MenuItem `yaml:"children,omitempty" json:"children,omitempty"`
}

// ─── 1.1.9 ConfigSpec — structured ConfigKey ───

// ConfigSpec defines runtime configuration values (Core §4.5, §10).
// Keys are structured — each key declares its type, default value, and
// whether it is a secret (never inlined in YAML, resolved per environment).
// Scripts read values via ctx.config.get("key").
type ConfigSpec struct {
	Keys map[string]ConfigKey `yaml:"keys" json:"keys"`
}

// ConfigKey declares one configuration entry (01-core-basic.md §10).
type ConfigKey struct {
	Type    string `yaml:"type" json:"type"` // int | string | bool | decimal | json
	Default any    `yaml:"default,omitempty" json:"default,omitempty"`
	Secret  bool   `yaml:"secret,omitempty" json:"secret,omitempty"`
}

// ─── 1.1.8 SubscriptionSpec — Tier 2 fields ───

// SubscriptionSpec subscribes to another resource's events (Core §4.6, Ref D35,
// 02-core-extended.md §3).
//
// Tier 1 (Core, outbox): Events + Handler only.
// Tier 2 (Streaming): adds store, retention, position, max_retry, dead_letter,
// filter, transform, and explicit delivery channel.
type SubscriptionSpec struct {
	Events     []string         `yaml:"events" json:"events"`
	Handler    ImplDecl         `yaml:"handler" json:"handler"`
	Store      string           `yaml:"store,omitempty" json:"store,omitempty"`             // Tier 2: stream backend (redis, kafka)
	Durable    string           `yaml:"durable,omitempty" json:"durable,omitempty"`         // Tier 2: durability mode
	Retry      *RetryDecl       `yaml:"retry,omitempty" json:"retry,omitempty"`             // Tier 2
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
	DDL    string `yaml:"ddl" json:"ddl"`
	Module string `yaml:"module,omitempty" json:"module,omitempty"` // owning module for table-level DDL
}

// ─── 1.1.2 WorkflowSpec ───

// WorkflowSpec defines an approval workflow attached to a state machine
// transition (02-core-extended.md §2).
type WorkflowSpec struct {
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
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
}

// WorkflowStep is one approval step in the workflow chain.
// Steps are evaluated sequentially; each must reach quorum before the next begins.
type WorkflowStep struct {
	Roles      []string        `yaml:"roles" json:"roles"`
	Approvers  int             `yaml:"approvers,omitempty" json:"approvers,omitempty"` // quorum, default 1
	Mode       string          `yaml:"mode,omitempty" json:"mode,omitempty"`           // all | any | sequential
	When       string          `yaml:"when,omitempty" json:"when,omitempty"`           // FormaExpr — skip step if false
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
	BasePath string   `yaml:"base_path,omitempty" json:"base_path,omitempty"` // replaces {module} in route
	Version  string   `yaml:"version,omitempty" json:"version,omitempty"`     // override {version} route
	Disable  []string `yaml:"disable,omitempty" json:"disable,omitempty"`     // entities to opt-out
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
	For            string           `yaml:"for" json:"for"`
	Method         string           `yaml:"method" json:"method"`
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
	Listen     *IntegratorListen `yaml:"listen" json:"listen"`
	Call       *IntegratorCall   `yaml:"call" json:"call"`
	Compensate string            `yaml:"compensate,omitempty" json:"compensate,omitempty"`
}

// IntegratorListen declares the source event to react to.
type IntegratorListen struct {
	Resource string `yaml:"resource" json:"resource"`
	Event    string `yaml:"event" json:"event"`
}

// IntegratorCall declares the target action to invoke.
type IntegratorCall struct {
	Resource string `yaml:"resource" json:"resource"`
	Action   string `yaml:"action" json:"action"`
}

// ─── 1.1.6 MockupSpec ───

// MockupSpec defines a simulated connector that implements the same contract
// as the real integration (02-core-extended.md §8).
type MockupSpec struct {
	For       string `yaml:"for" json:"for"` // module.service or module.entity
	ConfigRef string `yaml:"config_ref,omitempty" json:"config_ref,omitempty"`
}

// ─── 1.1.7 KindDefinitionSpec ───

// KindDefinitionSpec declares a new resource kind (CRD-like extension),
// including its schema and handler (platform/03-kind-system.md §2).
type KindDefinitionSpec struct {
	Group   string    `yaml:"group" json:"group"`
	Version string    `yaml:"version" json:"version"`
	Schema  any       `yaml:"schema,omitempty" json:"schema,omitempty"`
	Handler *ImplDecl `yaml:"handler" json:"handler"`
	Scope   string    `yaml:"scope,omitempty" json:"scope,omitempty"` // module | app
}
