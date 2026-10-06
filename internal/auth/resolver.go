package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/primadi/formspec/pkg/spec"
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
	// grantScopes caches the row scope attached to a (caller, permission) pair.
	// Separate from `cache` because its key includes the permission, and because
	// it is invalidated on the same events (role edit / spec reload).
	grantScopes map[string]grantScopeEntry
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
		grantScopes: map[string]grantScopeEntry{},
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
	r.grantScopes = map[string]grantScopeEntry{}
}

// grantScopeTTL bounds how long a resolved grant row scope is reused. Same
// trade-off as scopeAttrTTL in the API layer: long enough that a scoped list
// does not re-materialize a role on every request, short enough that editing a
// role takes effect without a restart.
const grantScopeTTL = 30 * time.Second

type grantScopeEntry struct {
	scope   []spec.FilterSpec
	expires time.Time
}

// GrantScope returns the row scope the caller's roles attach to ONE permission,
// or an error when a row restriction was DECLARED and cannot be applied.
//
// This is the second half of authorization: `{module}.{entity}.{action}` says
// whether the caller may act, and this says on which rows. It is resolved from
// the role grants (never from the request), so a client cannot widen or drop it.
//
// Why a permission key rather than a role key: the grants are declared per
// page/action and materialized into permissions, so the permission string is the
// only stable join between "who is asking" and "what rows". Several roles can
// contribute; the predicates are CONCATENATED (AND), which is the fail-closed
// reading — two grants that each restrict a field can only narrow the result,
// never widen it. Returns nil when nothing restricts the permission.
//
// FAIL-CLOSED RULE. A restriction that was declared and cannot be applied is a
// DENIAL, not a note in the log. Two shapes reach this point: a grant whose
// `row_scope` is unreadable (e.g. `row_scopes:`, or an entry with no value
// source — see ValidateGrantListShape) and a grant that carries a row scope but
// does not resolve to a page/action at all. In both cases the permission itself
// survives somewhere (another page, another role) while the restriction does
// not — which is precisely the 10.67 leak: a role that looks confined to the
// paid orders and in fact reads every row. So the caller is refused and the
// reason is logged through SetLogger, attributing it to the role that declared
// it.
func (r *PermissionResolver) GrantScope(ctx context.Context, workspaceID, app string, roles []string, permission string) ([]spec.FilterSpec, error) {
	if permission == "" || len(roles) == 0 || r.roleStore == nil || r.materialize == nil {
		return nil, nil
	}
	key := workspaceID + "/" + app + "/" + strings.Join(roles, ",") + "/" + permission

	r.mu.Lock()
	if e, ok := r.grantScopes[key]; ok && time.Now().Before(e.expires) {
		r.mu.Unlock()
		return e.scope, nil
	}
	r.mu.Unlock()

	var scope []spec.FilterSpec
	seenScope := map[string]bool{}
	for _, roleName := range roles {
		role, err := r.roleStore.GetByName(ctx, workspaceID, roleName)
		if err != nil {
			continue // unknown role — the permission check already ignores it
		}
		if role.App != "" && role.App != app {
			continue
		}
		if ownerRolePermission(role) != "" {
			continue // owner roles carry no row restriction
		}

		// A row restriction that could not even be READ is the loudest case: the
		// JSON says `row_scopes` (or a value source is missing), so nothing in the
		// role confines the permission — yet the permission is still granted.
		if p, ok := firstRowScopeShapeProblem(role); ok {
			if r.logf != nil {
				r.logf("auth: role %q: %s", role.Name, p)
			}
			return nil, fmt.Errorf("role %q declares a row restriction that cannot be read (%s) — refusing to act unscoped", role.Name, p)
		}

		detailed, problems := r.materialize.MaterializeDetailed(role.Grants)
		for _, p := range problems {
			if r.logf != nil {
				r.logf("auth: role %q grant contributed nothing — %s", role.Name, p)
			}
			// Declared but unusable: the action the operator meant to confine is
			// still reachable through this permission, so the request is refused
			// rather than answered with an unbounded read.
			if p.HadRowScope {
				return nil, fmt.Errorf("role %q declares a row restriction that cannot be applied (%s) — refusing to act unscoped", role.Name, p)
			}
		}
		for _, d := range detailed {
			if d.Permission != permission || len(d.RowScope) == 0 {
				continue
			}
			for _, sc := range d.RowScope {
				// De-duplicate identical entries so a role that declares the
				// same restriction on two pages does not AND it with itself.
				fp := sc.Field + "|" + sc.Op + "|" + sc.Value + "|" + sc.From + "|" + sc.Attr + "|" + sc.Param
				if seenScope[fp] {
					continue
				}
				seenScope[fp] = true
				scope = append(scope, sc)
			}
		}
	}

	r.mu.Lock()
	r.grantScopes[key] = grantScopeEntry{scope: scope, expires: time.Now().Add(grantScopeTTL)}
	r.mu.Unlock()
	return scope, nil
}

// firstRowScopeShapeProblem returns the first unreadable-row-restriction problem
// on a role, if any. Shape problems that only lose a grant (a misspelled page,
// say) are left to the materializer, which reports them per grant without
// denying anything — they cannot leave a restriction behind.
func firstRowScopeShapeProblem(role *Role) (GrantShapeProblem, bool) {
	for _, p := range role.GrantShapeProblems {
		if p.RowScope {
			return p, true
		}
	}
	return GrantShapeProblem{}, false
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
		perms, problems := r.materialize.MaterializePartial(role.Grants)
		// Grants that resolved to nothing are skipped INDIVIDUALLY, and named.
		//
		// Swallowing the reason made a typo in a grant page name
		// indistinguishable from a role that legitimately has no permissions:
		// the symptom is a 404 on every request (the permission check fails),
		// with nothing in the logs and no clue which grant caused it. Reporting
		// per grant also means one bad entry can no longer remove the access the
		// OTHER entries granted — before this, Materialize was all-or-nothing
		// and a single unknown page voided the whole role (kafe 10.53).
		for _, p := range problems {
			if r.logf != nil {
				r.logf("auth: role %q grant contributed no permissions — %s", role.Name, p)
			}
		}
		for _, p := range perms {
			add(p)
		}
	}
	return out, nil
}
