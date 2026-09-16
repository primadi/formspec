package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// AssignmentReader exposes the entities that declare a principal→dimension
// `assignments` mapping (S5) and resolves a principal's value for one of them.
// *entity.Registry implements it; the assertion in resolveAssignedAttr is
// optional so test fakes that only satisfy EntityStoreProvider keep working.
type AssignmentReader interface {
	// AssignmentSources lists every declared mapping, in a stable order.
	AssignmentSources() []spec.AssignmentSource
	// FindAssignmentValue returns the dimension value assigned to principal
	// (matched against the source's principal field), or "" when there is none.
	FindAssignmentValue(ctx context.Context, src spec.AssignmentSource, workspaceID, principal string) (string, error)
}

// ReadAllPermission is the explicit "see every row" grant for one entity:
// `{module}.{plural}.read_all`.
//
// Row scoping exists to keep a caller inside its own dimension (kafe: a cashier
// sees their branch). Some callers legitimately have no dimension at all — a
// workspace owner, or a super-admin identity in dev — and for them scoping would
// fail closed on every read, making the app unusable for the one person who is
// meant to see everything. The exemption is therefore an **explicit permission**
// rather than an implicit wildcard rule: it can be granted deliberately and shows
// up in an audit of who can read across branches.
//
// Note that `*` also satisfies this check (HasPermission treats it as a
// super-wildcard), so a dev super-admin keeps working; a cashier holding only
// `{module}.{plural}.list` is still scoped.
func ReadAllPermission(module, entity string, es *spec.EntitySpec) string {
	plural := entity + "s"
	if es != nil && es.Plural != "" {
		plural = es.Plural
	}
	return module + "." + plural + ".read_all"
}

// applyRowScope merges an entity's declared `row_scope` into a list/aggregate
// query (S2, gaps #6/#9).
//
// Unlike a kind's `fixed_filters` — merged in the browser, and therefore
// omittable by any client that edits the request — RowScope is authoritative and
// resolved here, server-side:
//
//   - `from: session` reads an attribute of the authenticated identity. The value
//     is never taken from the client, and it OVERRIDES any client-supplied filter
//     on the same field, so a branch cashier cannot widen the view by editing the
//     query string. When the token carries no such attribute, the value is looked
//     up from the entity that declares an `assignments` mapping for it (S5) —
//     e.g. employee.username → employee.branch_id. An attribute that cannot be
//     resolved either way fails closed: an empty value must never degrade into
//     "no filter".
//   - `from: route` reads a request query parameter (e.g. an unguessable guest
//     token). The token IS the credential, so a client-supplied value is expected
//     there; a missing parameter fails closed rather than listing everything.
//
// A caller with NO identity whose request arrived through a public grant that
// declares its own `scope` is skipped: that grant already constrains the rows
// (it is applied by applyPublicScope), and a session-sourced scope could never
// resolve for an anonymous caller — so resolving it here would deny every read
// on a public surface instead of filtering it. An entity with `row_scope` but no
// public grant still fails closed for anonymous callers, which is the correct
// answer for an entity nobody meant to expose.
//
// Returns the merged filter map (never nil when a scope is declared).
func (f *HandlerFactory) applyRowScope(r *http.Request, es *spec.EntitySpec, module, entity string, filters map[string]db.FilterOp) (map[string]db.FilterOp, error) {
	if es == nil || len(es.RowScope) == 0 {
		return filters, nil
	}
	// Explicit exemption: a caller holding `{module}.{plural}.read_all` reads
	// across rows by design (owner, cross-branch auditor). Checked before the
	// scope is resolved, because for such a caller an unresolvable attribute is
	// the normal case, not an error.
	if identity := IdentityFromContext(r.Context()); identity != nil &&
		identity.HasPermission(ReadAllPermission(module, entity, es)) {
		return filters, nil
	}
	// The public surface's own scope takes over for anonymous callers (see the
	// doc comment above).
	if IdentityFromContext(r.Context()) == nil && len(publicScopeFromContext(r.Context())) > 0 {
		return filters, nil
	}
	if filters == nil {
		filters = make(map[string]db.FilterOp, len(es.RowScope))
	}
	identity := IdentityFromContext(r.Context())

	for i := range es.RowScope {
		sc := &es.RowScope[i]
		if sc.Field == "" {
			continue
		}
		op := sc.Op
		if op == "" {
			op = "eq"
		}

		switch sc.From {
		case "session":
			// Attribute name: explicit `attr`, else the entity's declared scope
			// field (S5) — so `row_scope: [{field: branch_id, op: eq, from:
			// session}]` on an entity that declares `scope: {dimension: branch,
			// field: branch_id}` needs no third name for the same value.
			attr := sc.Attr
			if attr == "" && es.Scope != nil {
				attr = es.Scope.Field
			}
			value := sessionAttr(identity, attr)
			if value == "" {
				value = f.resolveAssignedAttr(r, attr)
			}
			if value == "" {
				return nil, fmt.Errorf(
					"row scope on %s: caller has no %q session attribute — refusing to list unscoped (issue it in the token's `attrs` claim, or declare `assignments` on the entity that maps the principal to this dimension)",
					sc.Field, scopeAttrLabel(attr))
			}
			filters[sc.Field] = db.FilterOp{Op: op, Value: value}

		case "route":
			param := sc.Param
			if param == "" {
				param = sc.Field
			}
			value := r.URL.Query().Get(param)
			if value == "" {
				return nil, fmt.Errorf(
					"row scope on %s: missing %q request parameter",
					sc.Field, param)
			}
			filters[sc.Field] = db.FilterOp{Op: op, Value: value}
		}
	}
	return filters, nil
}

// scopeAttrTTL bounds how long an assignment-resolved attribute is reused. Short
// enough that moving an employee to another branch takes effect without a
// re-login, long enough that a scoped list does not query the assignment entity
// on every request.
const scopeAttrTTL = 30 * time.Second

type scopeAttrEntry struct {
	value   string
	expires time.Time
}

var (
	scopeAttrMu    sync.RWMutex
	scopeAttrCache = map[string]scopeAttrEntry{}
)

// resetScopeAttrCache drops memoized assignment lookups. Tests use it so a case
// never observes a value resolved for an earlier case.
func resetScopeAttrCache() {
	scopeAttrMu.Lock()
	scopeAttrCache = map[string]scopeAttrEntry{}
	scopeAttrMu.Unlock()
}

// resolveAssignedAttr resolves a session attribute from the entities that declare
// an `assignments` mapping for it (S5): it finds the row whose principal field
// matches the authenticated username and reads the dimension field off it.
//
// Returns "" when nothing resolves — the caller fails closed. Store errors are
// deliberately folded into "": an unavailable assignment must never turn into an
// unfiltered list.
func (f *HandlerFactory) resolveAssignedAttr(r *http.Request, attr string) string {
	if attr == "" {
		return ""
	}
	identity := IdentityFromContext(r.Context())
	if identity == nil || identity.Username == "" {
		return ""
	}
	prov, ok := f.registry.(AssignmentReader)
	if !ok {
		return ""
	}

	cacheKey := identity.WorkspaceID + "\x00" + identity.Username + "\x00" + attr
	scopeAttrMu.RLock()
	entry, hit := scopeAttrCache[cacheKey]
	scopeAttrMu.RUnlock()
	if hit && time.Now().Before(entry.expires) {
		return entry.value
	}

	value := ""
	for _, src := range prov.AssignmentSources() {
		if src.Field != attr || src.Entity == "" {
			continue
		}
		v, err := prov.FindAssignmentValue(r.Context(), src, identity.WorkspaceID, identity.Username)
		if err != nil {
			// An unavailable assignment must never turn into an unfiltered list.
			continue
		}
		if v != "" {
			value = v
			break
		}
	}

	scopeAttrMu.Lock()
	scopeAttrCache[cacheKey] = scopeAttrEntry{value: value, expires: time.Now().Add(scopeAttrTTL)}
	scopeAttrMu.Unlock()

	if value != "" && identity.Attributes == nil {
		// Also record it on the identity so the rest of the request sees the same
		// value without a second lookup.
		identity.Attributes = map[string]string{attr: value}
	} else if value != "" {
		identity.Attributes[attr] = value
	}
	return value
}

// publicScopeContextKey carries a public grant's declared row scope (#45) from
// route registration to the list handler. It lives in the request context
// because the grant is an App-level declaration, while the handler is shared by
// every surface the entity appears on (a POS surface must not inherit the
// public surface's filter).
type publicScopeContextKey struct{}

// withPublicScope injects the grant's row scope into the request context.
func withPublicScope(h http.HandlerFunc, scope []spec.FilterSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r.WithContext(context.WithValue(r.Context(), publicScopeContextKey{}, scope)))
	}
}

// publicScopeFromContext returns the row scope declared by the public grant this
// request arrived through, or nil for any other route.
func publicScopeFromContext(ctx context.Context) []spec.FilterSpec {
	scope, _ := ctx.Value(publicScopeContextKey{}).([]spec.FilterSpec)
	return scope
}

// applyPublicScope merges a public grant's row scope into a list query (#45).
//
// The grant says WHICH entity an anonymous caller may read; this says WHICH ROWS.
// The value comes from a request parameter (e.g. an unguessable guest token) —
// the token IS the credential — and a missing one fails closed, exactly like an
// entity `row_scope` with `from: route`.
//
// It applies to ANONYMOUS callers only. An authenticated caller is governed by
// its permissions and the entity's own `row_scope`: the grant exists to constrain
// access that has no identity, and applying it to signed-in callers would filter
// the cashier's POS list by a token it does not carry.
func (f *HandlerFactory) applyPublicScope(r *http.Request, filters map[string]db.FilterOp) (map[string]db.FilterOp, error) {
	scope := publicScopeFromContext(r.Context())
	if len(scope) == 0 || IdentityFromContext(r.Context()) != nil {
		return filters, nil
	}
	if filters == nil {
		filters = make(map[string]db.FilterOp, len(scope))
	}
	for i := range scope {
		sc := &scope[i]
		if sc.Field == "" {
			continue
		}
		op := sc.Op
		if op == "" {
			op = "eq"
		}
		param := sc.Param
		if param == "" {
			param = sc.Field
		}
		value := r.URL.Query().Get(param)
		if value == "" {
			return nil, fmt.Errorf(
				"this entity is readable anonymously only together with a %q request parameter — refusing to list every row",
				param)
		}
		filters[sc.Field] = db.FilterOp{Op: op, Value: value}
	}
	return filters, nil
}

// scopeAttrLabel renders an attribute name for error messages.
func scopeAttrLabel(attr string) string {
	if attr == "" {
		return "principal_id"
	}
	return attr
}

// sessionAttr resolves a named attribute of the authenticated identity. Returns
// "" when the attribute is absent — the caller treats that as fail-closed.
//
// Note: an anonymous request has no identity here — dev mode's identity resolver
// returns nil for the `anonymous` permission — so a session-scoped entity fails
// closed for anonymous callers by design. Exercising the positive path therefore
// requires a real identity (unit test: TestApplyRowScope_SessionOverridesClientValue).
func sessionAttr(id *auth.Identity, attr string) string {
	if id == nil {
		return ""
	}
	switch attr {
	case "", "principal_id", "user_id":
		return id.UserID
	case "username":
		return id.Username
	case "workspace", "workspace_id":
		return id.WorkspaceID
	}
	return id.Attributes[attr]
}
