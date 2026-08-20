package auth

import (
	"fmt"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
)

// CoreModule is the reserved namespace for framework-owned resources
// (platform/02-workspace-app-module.md §9). It is always present in every
// workspace and cannot be declared by any user Module.
const CoreModule = "formspec.core"

// Role names for the logical auth roles resolved by RoleResolver.
const (
	RoleUser    = "user"
	RoleSession = "session"
	RoleRole    = "role"
)

// RegisterCoreEntities registers the framework-owned auth entities
// (formspec.core.user, formspec.core.session, formspec.core.role,
// formspec.core.role-assignment) into the registry. They are marked Internal —
// no API routes, no meta-bundle exposure — and are only reachable by framework
// code via GetEntityStore.
//
// Call this BEFORE SyncSchema so the tables are created. A user-provided
// override in external/ (or via auth_config_ref) replaces these at the
// RoleResolver level; the defaults remain registered as a safe fallback.
// If an entity with the same key is already registered (e.g. loaded from
// external/), the default is skipped — user override wins.
func RegisterCoreEntities(reg *entity.Registry) error {
	for _, e := range []struct {
		name string
		desc string
		spec *spec.EntitySpec
	}{
		{"user", "Framework-owned user account (auth)", coreUserSpec()},
		{"session", "Framework-owned login session (auth)", coreSessionSpec()},
		{"role", "Framework-owned role (auth) — page/tab/action grants", coreRoleSpec()},
		{"role-assignment", "Framework-owned role assignment (auth) — user→role in App", coreRoleAssignmentSpec()},
	} {
		if reg.HasEntity(CoreModule, e.name) {
			continue // user override wins
		}
		if err := reg.RegisterCoreEntity(CoreModule, e.name, e.desc, e.spec); err != nil {
			return fmt.Errorf("register core %s: %w", e.name, err)
		}
	}
	return nil
}

// coreUserSpec returns the EntitySpec for formspec.core.user.
//
// Role contract (minimal): username (unique) + password_hash. Extra fields
// (display_name, email, roles, permissions, active) are the default shape;
// a user-provided override only needs the contract fields.
func coreUserSpec() *spec.EntitySpec {
	return &spec.EntitySpec{
		Version:        "v1",
		Plural:         "users",
		Characteristic: spec.CharMaster,
		DisplayField:   "username",
		Fields: []spec.Field{
			{Name: "username", Type: spec.FieldString, Required: true, Unique: true, Index: true},
			{Name: "password_hash", Type: spec.FieldString, Required: true, Masked: true},
			{Name: "display_name", Type: spec.FieldString},
			{Name: "email", Type: spec.FieldString},
			{Name: "roles", Type: spec.FieldJSON},
			{Name: "permissions", Type: spec.FieldJSON},
			{Name: "active", Type: spec.FieldBoolean, Default: true},
		},
		// No Expose — internal entity, never surfaced via API.
	}
}

// coreSessionSpec returns the EntitySpec for formspec.core.session.
//
// Role contract (minimal): refresh_jti (unique) + user_id + expires_at.
func coreSessionSpec() *spec.EntitySpec {
	return &spec.EntitySpec{
		Version:        "v1",
		Plural:         "sessions",
		Characteristic: spec.CharTransaction,
		Fields: []spec.Field{
			{Name: "transaction_date", Type: spec.FieldDate, Required: true},
			{Name: "refresh_jti", Type: spec.FieldString, Required: true, Unique: true, Index: true},
			{Name: "user_id", Type: spec.FieldString, Required: true, Index: true},
			{Name: "expires_at", Type: spec.FieldDateTime, Required: true},
			{Name: "ip", Type: spec.FieldString},
			{Name: "user_agent", Type: spec.FieldString},
		},
		// No Expose — internal entity, never surfaced via API.
	}
}

// coreRoleSpec returns the EntitySpec for formspec.core.role.
//
// A role is a named collection of grants (page → tab → action + conditions),
// scoped to an App (platform/02 §9: role dimiliki Module, ter-scope per-App).
// The `grants` field holds the admin-facing grant hierarchy (Grant type);
// it is materialized into concrete permission strings at enforcement time.
func coreRoleSpec() *spec.EntitySpec {
	return &spec.EntitySpec{
		Version:        "v1",
		Plural:         "roles",
		Characteristic: spec.CharMaster,
		DisplayField:   "name",
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString, Required: true, Unique: true, Index: true},
			{Name: "app", Type: spec.FieldString, Index: true},
			{Name: "description", Type: spec.FieldString},
			{Name: "grants", Type: spec.FieldJSON},
		},
		// No Expose — internal entity, never surfaced via API.
	}
}

// coreRoleAssignmentSpec returns the EntitySpec for formspec.core.role-assignment.
//
// A role-assignment binds a user to a role within an App context
// (platform/02 §9: penetapan role ke user dalam konteks App tertentu).
func coreRoleAssignmentSpec() *spec.EntitySpec {
	return &spec.EntitySpec{
		Version:        "v1",
		Plural:         "role-assignments",
		Characteristic: spec.CharMaster,
		Fields: []spec.Field{
			{Name: "user_id", Type: spec.FieldString, Required: true, Index: true},
			{Name: "role_id", Type: spec.FieldString, Required: true, Index: true},
			{Name: "app", Type: spec.FieldString, Index: true},
			{Name: "active", Type: spec.FieldBoolean, Default: true},
		},
		// No Expose — internal entity, never surfaced via API.
	}
}
