// Package spec defines the Go types that mirror the FormSpec specification.
// These are the canonical structs used throughout the runtime to represent
// all FormSpec resource kinds.
//
// The types in this package correspond 1:1 with the YAML manifest format
// defined in docs/spec/backend/{01-core-basic,02-core-extended,03-entity-extension}.md,
// docs/spec/frontend/{01-04} (docs_old/spec/05-frontend.md §3-13 pending migration),
// and docs_old/spec/04-control-plane.md (pending migration).
package spec

// APIVersion is the current FormSpec API version string.
const APIVersion = "formspec.dev/v1alpha1"

// Manifest is the top-level structure of every FormSpec YAML document.
// It contains exactly four top-level keys: apiVersion, kind, metadata, spec.
type Manifest struct {
	APIVersion string         `yaml:"apiVersion" json:"apiVersion"`
	Kind       Kind           `yaml:"kind" json:"kind"`
	Metadata   Metadata       `yaml:"metadata" json:"metadata"`
	Spec       any            `yaml:"spec" json:"spec"` // kind-specific, parsed by kind
	RawSpec    map[string]any `yaml:"-" json:"-"`       // raw spec for deferred parsing
}

// Metadata is common to all FormSpec resources.
type Metadata struct {
	Name        string            `yaml:"name" json:"name"`
	Module      string            `yaml:"module,omitempty" json:"module,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// Kind represents the resource kind as a PascalCase string.
// @schema {title: "Resource Kind"}
type Kind string

// Core Basic built-in kinds (Core §4).
const (
	KindApp          Kind = "App"
	KindModule       Kind = "Module"
	KindEntity       Kind = "Entity"   // stateful persisted resource (Core §4.1)
	KindDocument     Kind = "Document" // deprecated alias for KindEntity (kept for backward compatibility)
	KindService      Kind = "Service"
	KindConfig       Kind = "Config"
	KindMigration    Kind = "Migration"
	KindSubscription Kind = "Subscription"
)

// Core Extended kinds (D48).
const (
	KindWorkflow       Kind = "Workflow"
	KindApi            Kind = "Api"
	KindKindDefinition Kind = "KindDefinition"
	KindWebhook        Kind = "Webhook"
	KindMockup         Kind = "Mockup"
	KindIntegrator     Kind = "Integrator" // cross-module bridge (Core Extended §5)
)

// Control Plane kinds (Control §2, Datastore Spec).
const (
	KindEnvironment Kind = "Environment"
	KindPolicy      Kind = "Policy"
	KindDatastore   Kind = "Datastore"
)

// Renderer / meta-kind kinds (frontend/02-visual-spec-kind.md,
// frontend/03-renderer-kind.md, backend/04-persist-backend.md).
const (
	KindRenderer       Kind = "Renderer"
	KindVisualSpecKind Kind = "VisualSpecKind"
	KindPersistBackend Kind = "PersistBackend"
)

// Frontend kinds (Frontend §2).
//
// There is no KindMenu — navigation is not a standalone kind. It lives as
// App.spec.menu and Module.spec.menu, both typed as []MenuItem
// (Core §4.4/§4.5).
const (
	KindPage      Kind = "Page"
	KindForm      Kind = "Form"
	KindTable     Kind = "Table"
	KindDashboard Kind = "Dashboard"
	KindWidget    Kind = "Widget"
	KindReport    Kind = "Report"
	KindWizard    Kind = "Wizard"
	KindKanban    Kind = "Kanban"
	KindTimeline  Kind = "Timeline"
	KindPrint     Kind = "Print"
	KindTheme     Kind = "Theme"
	KindListing   Kind = "Listing"
	KindCalendar  Kind = "Calendar"
	// ApprovalInbox and NotificationCenter are zero-config pages (no entity
	// binding) — their data sources are the caller's pending approvals /
	// notifications (06-page-kinds.md §11–§12).
	KindApprovalInbox      Kind = "ApprovalInbox"
	KindNotificationCenter Kind = "NotificationCenter"
)

// Characteristic classifies the data nature of an Entity (Core §4.1).
// @schema {title: "Characteristic", description: "Data nature classification — determines storage strategy, partitioning, and API behavior", example: "transaction"}
type Characteristic string

const (
	CharMaster      Characteristic = "master"
	CharTransaction Characteristic = "transaction"
	CharReference   Characteristic = "reference"
	CharSummary     Characteristic = "summary"
)

// ImplType is the implementation strategy for an action (Core §4.1).
// @schema {title: "Implementation Type", description: "How an action is implemented: script_ref (Starlark file), script (inline Starlark), native (Go), compiled (WASM), sidecar (external process)", example: "native"}
type ImplType string

const (
	ImplScriptRef ImplType = "script_ref" // Starlark script
	ImplScript    ImplType = "script"     // Inline Starlark
	ImplNative    ImplType = "native"     // Go compiled code
	ImplCompiled  ImplType = "compiled"   // Pre-compiled WASM
	ImplSidecar   ImplType = "sidecar"    // External process
)

// Environment mode (Control §2).
// @schema {description: "Deployment environment mode", enum: ["dev", "prod"]}
type EnvironmentMode string

const (
	EnvModeDev  EnvironmentMode = "dev"
	EnvModeProd EnvironmentMode = "prod"
)

// IsValidKind returns true if k is a known FormSpec kind.
func IsValidKind(k Kind) bool {
	switch k {
	case KindApp, KindModule, KindDocument, KindEntity, KindService, KindConfig, KindMigration, KindSubscription,
		KindWorkflow, KindApi, KindKindDefinition, KindWebhook, KindMockup, KindIntegrator,
		KindEnvironment, KindPolicy, KindDatastore,
		KindRenderer, KindVisualSpecKind, KindPersistBackend,
		KindPage, KindForm, KindTable, KindDashboard, KindWidget, KindReport,
		KindWizard, KindKanban, KindTimeline, KindPrint, KindTheme,
		KindListing, KindCalendar, KindApprovalInbox, KindNotificationCenter:
		return true
	default:
		return false
	}
}

// IsEntityKind returns true if k is Entity (or the deprecated Document alias).
func IsEntityKind(k Kind) bool {
	return k == KindEntity || k == KindDocument
}

// IsDocumentKind returns true if k is Document (or deprecated Entity alias).
// Deprecated: renamed to IsEntityKind; kept for backward compatibility.
func IsDocumentKind(k Kind) bool {
	return IsEntityKind(k)
}
