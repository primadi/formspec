package auth

import (
	"context"
	"strings"
	"sync"
)

// PermissionResolver resolves a user's effective permissions — their direct
// grants plus the materialized grants of every role they hold — with a
// per-session cache (todo 6.2.4). The cache avoids re-materializing role
// grants on every token issuance; it is invalidated when roles change.
type PermissionResolver struct {
	users       *EntityUserStore
	roleStore   *RoleStore
	materialize *Materializer
	// logf, when set, receives diagnostics that would otherwise vanish. A role
	// with malformed grants is the important case: without this it looks like a
	// plain authorization failure.
	logf func(format string, args ...any)

	mu    sync.Mutex
	cache map[string][]string // key: workspaceID + "/" + userID
}

// SetLogger installs a diagnostic logger. Optional — a nil logger keeps the
// resolver silent, which is right for tests.
func (r *PermissionResolver) SetLogger(logf func(format string, args ...any)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logf = logf
}

// NewPermissionResolver creates a permission resolver.
func NewPermissionResolver(users *EntityUserStore, roleStore *RoleStore, materialize *Materializer) *PermissionResolver {
	return &PermissionResolver{
		users:       users,
		roleStore:   roleStore,
		materialize: materialize,
		cache:       map[string][]string{},
	}
}

// Resolve returns the user's effective permissions, using the per-session
// cache. The returned slice must not be mutated by the caller.
//
// app scopes the resolution: roles with a non-empty App only contribute when
// they match the current app (empty app = workspace-global role). The cache
// key includes the app so a user logged into different Apps gets distinct
// permission sets.
func (r *PermissionResolver) Resolve(ctx context.Context, workspaceID, app string, user *User) ([]string, error) {
	key := workspaceID + "/" + app + "/" + user.ID

	r.mu.Lock()
	if perms, ok := r.cache[key]; ok {
		r.mu.Unlock()
		return perms, nil
	}
	r.mu.Unlock()

	perms, err := r.resolveUncached(ctx, workspaceID, app, user)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.cache[key] = perms
	r.mu.Unlock()
	return perms, nil
}

// Invalidate clears the cached permissions for a user. Call this whenever a
// user's roles change so the next resolution is fresh.
func (r *PermissionResolver) Invalidate(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	suffix := "/" + userID
	for k := range r.cache {
		if strings.HasSuffix(k, suffix) {
			delete(r.cache, k)
		}
	}
}

// InvalidateAll clears the entire cache (e.g. on spec hot-reload).
func (r *PermissionResolver) InvalidateAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = map[string][]string{}
}

// resolveUncached computes a user's effective permissions without the cache.
func (r *PermissionResolver) resolveUncached(ctx context.Context, workspaceID, app string, user *User) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range user.Permissions {
		add(p)
	}
	if r.roleStore == nil || r.materialize == nil {
		return out, nil
	}
	for _, roleName := range user.Roles {
		role, err := r.roleStore.GetByName(ctx, workspaceID, roleName)
		if err != nil {
			continue // unknown role — skip
		}
		// App-scoped roles only contribute when the current app matches.
		// Empty app = workspace-global role (e.g. seeded owner roles).
		if role.App != "" && role.App != app {
			continue
		}
		// Owner roles grant broad wildcard access within their scope
		// (todo 6.3.4) — no page-grant materialization needed.
		if p := ownerRolePermission(role); p != "" {
			add(p)
			continue
		}
		perms, err := r.materialize.Materialize(role.Grants)
		if err != nil {
			// A role whose grants cannot be materialized contributes NOTHING.
			// Swallowing this made a typo in a grant page name indistinguishable
			// from a role that legitimately has no permissions: the symptom is a
			// 404 on every request (permission check fails), with no error
			// anywhere and nothing in the logs. Report it and carry on — the
			// remaining roles may still grant access, so failing the whole login
			// would be worse.
			if r.logf != nil {
				r.logf("auth: role %q has unmaterializable grants (contributing no permissions): %v", role.Name, err)
			}
			continue
		}
		for _, p := range perms {
			add(p)
		}
	}
	return out, nil
}
