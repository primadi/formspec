// Package permission provides the permission model and registry for Forma.
//
// It implements D20 — the explicit permission model:
//   - Every action declares `required_permission` (caller guard)
//   - Every action declares `uses` (code's own access: db, resources, config, primitives)
//   - Module footprint = aggregate of all declarations, used for consent at install time
//
// Core concepts:
//   - PermissionEntry: a single required_permission with its source
//   - UsesEntry: a parsed uses declaration with all resource/config/primitive accesses
//   - ModuleFootprint: aggregated footprint for one module
//   - Registry: indexes all permissions across loaded modules
package permission

import (
	"fmt"
	"strings"
)

// PermissionType classifies a permission entry.
type PermissionType string

const (
	// PermAction is a "required_permission" — guards caller access to an action.
	PermAction PermissionType = "action"
	// PermUse is a "uses" declaration — declares code's own access requirements.
	PermUse PermissionType = "use"
)

// AccessMode is the type of access declared in uses.
type AccessMode string

const (
	AccessRead  AccessMode = "read"
	AccessWrite AccessMode = "write"
)

// PermissionEntry records one required_permission declaration from a manifest.
type PermissionEntry struct {
	Module string         `json:"module"`
	Entity string         `json:"entity"`
	Action string         `json:"action"`
	Type   PermissionType `json:"type"` // "action" or "use"
	Key    string         `json:"key"`  // fully-qualified permission key
	Source string         `json:"source"`
	Audit  bool           `json:"audit,omitempty"`
}

// UsesEntry records a parsed "uses" declaration from an action.
type UsesEntry struct {
	Module     string        `json:"module"`
	Entity     string        `json:"entity"`
	Action     string        `json:"action"`
	Resources  []ResourceUse `json:"resources,omitempty"`
	Config     *ConfigUse    `json:"config,omitempty"`
	Primitives []string      `json:"primitives,omitempty"`
	Db         *DbUse        `json:"db,omitempty"` // only when UsesDecl has db field
}

// ResourceUse records access to another resource.
type ResourceUse struct {
	Target string     `json:"target"` // "module.entity" or "module.entity.action"
	Mode   AccessMode `json:"mode"`   // "read" or "write"
}

// ConfigUse records config key access.
type ConfigUse struct {
	Read  []string `json:"read,omitempty"`
	Write []string `json:"write,omitempty"`
}

// DbUse records database category access.
type DbUse struct {
	Read  []string `json:"read,omitempty"`
	Write []string `json:"write,omitempty"`
}

// ModuleFootprint is the aggregate permission footprint of one module.
//
// Per D20, this is presented to the Data Owner at install time for consent.
// Cross-module writes are flagged as "high-risk consent" (D46).
type ModuleFootprint struct {
	Module            string            `json:"module"`
	Description       string            `json:"description,omitempty"`
	Permissions       []PermissionEntry `json:"permissions"`
	Uses              []UsesEntry       `json:"uses"`
	CrossModuleWrites []string          `json:"cross_module_writes,omitempty"` // resources written outside own module
}

// String returns a human-readable summary of the footprint.
func (f *ModuleFootprint) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Module: %s\n", f.Module))
	b.WriteString(fmt.Sprintf("  Required permissions: %d\n", len(f.Permissions)))
	b.WriteString(fmt.Sprintf("  Uses declarations:    %d\n", len(f.Uses)))
	for _, p := range f.Permissions {
		b.WriteString(fmt.Sprintf("    - %s (on %s.%s)\n", p.Key, p.Entity, p.Action))
	}
	if len(f.CrossModuleWrites) > 0 {
		b.WriteString("  ⚠ Cross-module writes (high-risk):\n")
		for _, cm := range f.CrossModuleWrites {
			b.WriteString(fmt.Sprintf("    - %s\n", cm))
		}
	}
	return b.String()
}

// ValidatePermissionFormat checks that a permission string is valid.
//
// Valid formats:
//   - "{module}.{key}" — fully qualified (e.g., "billing.invoices.list")
//   - "{key}" — auto-prefixed by own module at registration time
//   - "public" — anonymous access (always allowed)
func ValidatePermissionFormat(perm string) error {
	if perm == "" || perm == "public" {
		return nil
	}
	parts := strings.Split(perm, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid permission %q: must be %q or \"{module}.{key}\" format", perm, "public")
	}
	for _, p := range parts {
		if p == "" {
			return fmt.Errorf("invalid permission %q: empty segment", perm)
		}
	}
	return nil
}

// AutoPrefixPermission adds the module prefix if the permission is not qualified.
//
// Spec §4.7: "Every permission string is fully qualified as {module}.{key}.
// Inside a manifest, own-module prefix MAY be omitted and MUST be auto-prefixed."
//
// Rules:
//   - "invoices.list" (2 segments) → "billing.invoices.list"  (unqualified → auto-prefix)
//   - "billing.invoices.list" (3+ segments) → "billing.invoices.list" (already qualified)
//   - "public" → "public" (reserved keyword, unchanged)
func AutoPrefixPermission(perm, module string) string {
	if perm == "" || perm == "public" {
		return perm
	}
	// Count segments to determine if it's already qualified
	// Qualified = {module}.{key} = 2+ dots = 3+ segments
	// Unqualified = {key} = 0-1 dots = 1-2 segments
	parts := strings.Split(perm, ".")
	if len(parts) >= 3 {
		return perm // already qualified (module.entity.action)
	}
	if len(parts) == 2 {
		// 2 segments — treat as unqualified {entity}.{action}
		// Unless the first segment is our own module name (redundant)
		if parts[0] == module {
			return perm // "billing.list" with module billing → keep
		}
		return module + "." + perm // "invoices.list" → "billing.invoices.list"
	}
	// 1 segment — bare action name
	return module + "." + perm
}

// ParseResourceTarget parses a resource use string into module/entity/action.
// Formats:
//   - "billing.invoice.read" → module="billing", entity="invoice", action="read"
//   - "billing.invoice"     → module="billing", entity="invoice", action="" (full entity access)
//   - "invoice.read"        → module default, entity="invoice", action="read"
func ParseResourceTarget(target, defaultModule string) (module, entity, action string, err error) {
	parts := strings.Split(target, ".")
	switch len(parts) {
	case 1:
		return "", "", "", fmt.Errorf("invalid resource target %q: too few segments", target)
	case 2:
		return defaultModule, parts[0], parts[1], nil
	case 3:
		return parts[0], parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("invalid resource target %q: too many segments", target)
	}
}

// AuthChecker implements auth.PermissionChecker backed by a permission.Registry.
// It validates that a permission exists in the registry before checking the identity.
type AuthChecker struct {
	registry *Registry
}

// NewAuthChecker creates a new AuthChecker.
func NewAuthChecker(registry *Registry) *AuthChecker {
	return &AuthChecker{registry: registry}
}

// PermissionExists checks if a permission key is registered.
func (c *AuthChecker) PermissionExists(key string) bool {
	return c.registry.PermissionExists(key)
}

// HasPermission checks if the identity has the given permission.
// Returns nil if allowed, or an error describing why not.
func (c *AuthChecker) HasPermission(id interface {
	HasPermission(string) bool
}, permission string) error {
	if !id.HasPermission(permission) {
		return fmt.Errorf("missing permission: %s", permission)
	}
	return nil
}
