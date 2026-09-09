package spec

import (
	"fmt"
	"regexp"
)

// Workspace is a platform-level kind: it declares a named workspace — the
// single multi-tenancy unit of FormSpec (platform/02-workspace-app-module.md
// §1). Workspace manifests are seed declarations: the slug becomes (and must
// equal) the workspace ID used in URLs (/{ws}/...) and as the tenant scope of
// every store. Runtime-created workspaces (CLI `formspec workspace create`)
// land in the same registry — the formspec.core/workspace entity — so both
// sources converge on one store.
//
// Workspace is NOT a business Entity: it has no CRUD surface of its own and
// participates in no module. It is a boot-time registry seed, like kind: App.
const KindWorkspace Kind = "Workspace"

// WorkspaceSpec is the kind-specific body of a `kind: Workspace` manifest.
type WorkspaceSpec struct {
	// Slug is the URL-facing workspace identifier — the first path segment
	// (/{ws}/...). Optional: defaults to metadata.name. Must be kebab-case
	// and must not collide with a reserved first segment.
	// @schema {example: "cafe", pattern: "^[a-z0-9](-[a-z0-9])*$", description: "URL-facing workspace slug — defaults to metadata.name; kebab-case, unique across the deployment"}
	Slug string `yaml:"slug,omitempty" json:"slug,omitempty"`
	// DisplayName is the human-readable workspace name shown in admin
	// surfaces. metadata.name stays the machine identifier.
	// @schema {example: "Kafe Demo", description: "Human-readable workspace name — shown in admin surfaces"}
	DisplayName string `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	// Description explains the workspace's purpose (AI readability).
	// @schema {example: "Workspace utama aplikasi kafe demo"}
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// OwnerUserID optionally seeds the workspace owner (user ID). Empty =
	// no seeded owner; the first admin is created via the setup wizard.
	// @schema {description: "Optional user ID seeded as workspace owner — empty = no seeded owner (first admin via setup wizard)"}
	OwnerUserID string `yaml:"owner_user_id,omitempty" json:"owner_user_id,omitempty"`
	// Settings is free-form workspace metadata persisted to the
	// formspec.core/workspace registry row (json). Structured platform
	// settings (locale, currency, auth) belong to kind: Config, not here.
	// @schema {description: "Free-form workspace metadata persisted to the registry row (json) — structured platform settings belong to kind: Config"}
	Settings map[string]any `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// workspaceSlugPattern is the kebab-case constraint for workspace slugs.
var workspaceSlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ReservedWorkspaceSlugs is the closed set of first path segments that
// cannot be used as a workspace slug — they are reserved by the router
// surfaces mounted under the workspace prefix (AppSpec.RootURL docs) and by
// root-level mounts (static assets, health): _ui, api, _admin, assets,
// health, login, register, _ws, print, favicon.svg, icons.svg, manifest.json.
var ReservedWorkspaceSlugs = map[string]bool{
	"_ui":           true,
	"api":           true,
	"_admin":        true,
	"assets":        true,
	"health":        true,
	"login":         true,
	"register":      true,
	"_ws":           true,
	"print":         true,
	"favicon.svg":   true,
	"icons.svg":     true,
	"manifest.json": true,
}

// DefaultWorkspaceSlug is the workspace seeded/assumed when no explicit
// workspace is configured (dev mode, CLI flags, frontend fallback).
const DefaultWorkspaceSlug = "default"

// EffectiveSlug returns the registry slug of the workspace: spec.slug when
// set, otherwise metadata.name.
func (w *WorkspaceSpec) EffectiveSlug(name string) string {
	if w.Slug != "" {
		return w.Slug
	}
	return name
}

// IsValidWorkspaceSlug reports whether slug is a valid, non-reserved
// workspace slug (kebab-case, not a reserved router segment). Used by
// ValidateWorkspaceSpec and by any allowlist that references workspace
// slugs (e.g. AppSpec.Workspaces).
func IsValidWorkspaceSlug(slug string) bool {
	if !workspaceSlugPattern.MatchString(slug) {
		return false
	}
	return !ReservedWorkspaceSlugs[slug]
}

// ValidateWorkspaceSpec validates a WorkspaceSpec, returning an error if any
// constraint is violated. Enforces:
//   - effective slug (spec.slug or metadata.name) is kebab-case.
//   - slug is not a reserved first path segment.
func ValidateWorkspaceSpec(w *WorkspaceSpec, name string) error {
	slug := w.EffectiveSlug(name)
	if !IsValidWorkspaceSlug(slug) {
		if workspaceSlugPattern.MatchString(slug) {
			return fmt.Errorf("workspace slug %q is reserved (reserved segments: _ui, api, _admin, assets, health, login, register, _ws, print, favicon.svg, icons.svg, manifest.json)", slug)
		}
		return fmt.Errorf("workspace slug %q is invalid (kebab-case required: lowercase letters, digits, hyphens)", slug)
	}
	return nil
}

// MountsWithin reports whether the App may mount in workspace ws, per the
// optional Workspaces allowlist (plan docs_internal/plan/named-workspaces.md):
//
//   - nil (field absent): all workspaces — the backward-compatible default.
//   - non-nil empty (explicit `workspaces: []`): NO workspace — the App is
//     staged (uploaded but mounted nowhere; still validated + reloadable).
//   - non-empty: only the listed slugs.
func (a *AppSpec) MountsWithin(ws string) bool {
	if a.Workspaces == nil {
		return true
	}
	for _, w := range *a.Workspaces {
		if w == ws {
			return true
		}
	}
	return false
}
