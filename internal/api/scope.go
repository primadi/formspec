package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
// A caller authorized by a public grant that declares its own `scope` is
// skipped: that grant already constrains the rows (it is applied by
// applyPublicScope), and a session-sourced scope could never resolve for such a
// caller — so resolving it here would deny every read on a public surface
// instead of filtering it. That covers both anonymous callers and signed-in
// ones running on the grant (see RequirePermissionOrAnonymous). An entity with
// `row_scope` but no public grant still fails closed for anonymous callers,
// which is the correct answer for an entity nobody meant to expose.
//
// Returns the merged filter map (never nil when a scope is declared).
func (f *HandlerFactory) applyRowScope(r *http.Request, es *spec.EntitySpec, module, entity string, filters map[string]db.FilterOp) (map[string]db.FilterOp, error) {
	preds, err := f.entityRowScopePredicates(r, es, module, entity)
	if err != nil {
		return nil, err
	}
	if len(preds) == 0 {
		return filters, nil
	}
	if filters == nil {
		filters = make(map[string]db.FilterOp, len(preds))
	}
	// Assigning into the map keeps the long-standing override semantics: a scope
	// on a field the client also filtered replaces the client's value, so the
	// client cannot widen the view by editing the query string. A second
	// predicate on the SAME field would overwrite the first here, which is why
	// grant row scopes do not go through this map — they are ANDed in SQL
	// (RowPredicates), where two restrictions can only narrow the result.
	for _, p := range preds {
		filters[p.Field] = p.FilterOpFrom()
	}
	return filters, nil
}

// entityRowScopePredicates resolves an entity's declared `row_scope` into row
// predicates, applying the two exemptions that make row scoping usable rather
// than merely strict.
//
//   - A caller holding `{module}.{plural}.read_all` reads across rows by design
//     (owner, cross-branch auditor). For them an unresolvable attribute is the
//     normal case, not an error.
//   - A caller authorized by a public grant that declares its own `scope` is
//     skipped: that grant already constrains the rows (applyPublicScope does
//     it), and a session-sourced scope could never resolve for such a caller —
//     resolving it here would deny every read on a public surface instead of
//     filtering it.
//
// This is the shared core behind BOTH the list path (via applyRowScope, which
// merges into the filter map) and the id-addressed paths (get/update/delete,
// which pass the predicates into the store). Splitting them earlier is the
// mistake this repo keeps re-learning: two copies of the same resolution drift,
// and the copy that drifts is the one that stops enforcing (10.46).
func (f *HandlerFactory) entityRowScopePredicates(r *http.Request, es *spec.EntitySpec, module, entity string) ([]db.RowPredicate, error) {
	if es == nil || len(es.RowScope) == 0 {
		return nil, nil
	}
	if identity := IdentityFromContext(r.Context()); identity != nil &&
		identity.HasPermission(ReadAllPermission(module, entity, es)) {
		return nil, nil
	}
	if len(publicScopeFromContext(r.Context())) > 0 &&
		(IdentityFromContext(r.Context()) == nil || isPublicGrantAuth(r.Context())) {
		return nil, nil
	}
	return f.filterSpecsToPredicates(r, es, es.RowScope, "row scope")
}

// filterSpecsToPredicates resolves a list of FilterSpec into row predicates.
//
// `origin` names the declaration for the error message ("row scope" for an
// entity, "grant row scope" for a role grant) so an operator can tell WHICH
// declaration refused the request.
func (f *HandlerFactory) filterSpecsToPredicates(r *http.Request, es *spec.EntitySpec, specs []spec.FilterSpec, origin string) ([]db.RowPredicate, error) {
	identity := IdentityFromContext(r.Context())
	var out []db.RowPredicate

	for i := range specs {
		sc := &specs[i]
		if sc.Field == "" {
			continue
		}
		op := sc.Op
		if op == "" {
			op = "eq"
		}

		switch sc.From {
		case "":
			// Literal value: the manifest itself states the predicate, so the
			// filter is a server-side constant no client can widen or drop
			// (kafe 10.67 / GAP-08). An empty literal is refused rather than
			// skipped — a scope entry that silently filters nothing looks like
			// protection while providing none.
			if sc.Value == "" {
				return nil, fmt.Errorf(
					"%s on %s: entry declares neither a value source (from: session|route) nor a literal value — refusing to read unscoped",
					origin, sc.Field)
			}
			value, err := scopeLiteralValue(op, sc.Value)
			if err != nil {
				return nil, fmt.Errorf("%s on %s: %w", origin, sc.Field, err)
			}
			out = append(out, db.RowPredicate{Field: sc.Field, Op: op, Value: value})

		case "session":
			// Attribute name: explicit `attr`, else the entity's declared scope
			// field (S5) — so `row_scope: [{field: branch_id, op: eq, from:
			// session}]` on an entity that declares `scope: {dimension: branch,
			// field: branch_id}` needs no third name for the same value.
			attr := sc.Attr
			if attr == "" && es != nil && es.Scope != nil {
				attr = es.Scope.Field
			}
			value := sessionAttr(identity, attr)
			if value == "" {
				value = f.resolveAssignedAttr(r, attr)
			}
			if value == "" {
				return nil, fmt.Errorf(
					"%s on %s: caller has no %q session attribute — refusing to proceed unscoped (issue it in the token's `attrs` claim, or declare `assignments` on the entity that maps the principal to this dimension)",
					origin, sc.Field, scopeAttrLabel(attr))
			}
			out = append(out, db.RowPredicate{Field: sc.Field, Op: op, Value: value})

		case "route":
			value, err := f.resolveRouteScopeValue(r, sc)
			if err != nil {
				return nil, fmt.Errorf("%s on %s: %w", origin, sc.Field, err)
			}
			out = append(out, db.RowPredicate{Field: sc.Field, Op: op, Value: value})
		}
	}
	return out, nil
}

// enforceCreateScope applies an entity's `create_scope` declarations to a
// create payload (kafe 10.76 family; plan public-scope-enforcement.md).
//
// Why this exists at all: every other scope in the system restricts a READ, and
// a read can be restricted because the row already exists. A create has no row
// to filter yet — `db.InsertParams` carries no predicates — so a caller could
// simply write the dimension value of their choice. Measured on kafe: an
// anonymous QR guest could create an order with another branch's `branch_id`,
// because `order.branch_id` declares no `required_permission` and
// `denyForbiddenFieldWrites` only guards fields that do.
//
// The rule is CONDITIONAL on the reference: the check fires only when the
// payload carries a value for `ref_field`. That is deliberate — a cashier
// creating a walk-in order passes no `table_session_id`, so there is no record
// to derive the branch from, and the rule must not turn into "every create needs
// a table session".
//
// A MISMATCH IS REFUSED, never overwritten: silently replacing the value would
// make the request and the stored row disagree, and the caller would have no way
// to notice that their input was discarded.
//
// It returns the HTTP status alongside the error because the two failure modes
// are different CLASSES, and collapsing them was a real defect (measured: a
// broken `dining_table_id` started answering 403 "cannot verify" instead of the
// 422 the relation check had always produced):
//
//   - a mismatch is an AUTHORIZATION refusal (403): the caller is asking to
//     write outside the dimension they are confined to;
//   - an unresolvable reference is bad INPUT (422), the same class as any other
//     dangling reference — the caller is allowed, the payload is wrong. Answering
//     403 there would mislabel a client-fault as a permission problem and break
//     each surface's error handling.
//
// The status is 0 when err is nil.
func (f *HandlerFactory) enforceCreateScope(ctx context.Context, module, entity string, es *spec.EntitySpec, body map[string]any) (int, error) {
	if es == nil || len(es.CreateScope) == 0 {
		return 0, nil
	}

	for i := range es.CreateScope {
		cs := &es.CreateScope[i]

		refValue := createScopeString(body[cs.RefField])
		if refValue == "" {
			// No reference in the payload: nothing to derive the dimension from,
			// so the declaration does not apply to this request.
			continue
		}

		want, err := f.resolveViaValue(ctx, cs.Via, cs.ViaField, refValue)
		if err != nil {
			// Refused here rather than left to the foreign-key check, so the rule
			// cannot be skipped by referencing something that does not exist
			// (fail closed). Classified as bad input, because that is what it is.
			return http.StatusUnprocessableEntity, fmt.Errorf("create scope on %s: %w", cs.Field, err)
		}
		got := createScopeString(body[cs.Field])
		if got != "" && got != want {
			return http.StatusForbidden, fmt.Errorf("create scope on %s: payload says %q but %s (%s) says %q — the %s of a new record must match the record it references",
				cs.Field, got, cs.Via, refValue, want, cs.Field)
		}
	}
	return 0, nil
}

// errViaNotFound marks a `via` reference that resolves to no record, so callers
// can classify it (bad input) instead of treating it as an authorization denial.
var errViaNotFound = errors.New("the referenced record does not exist")

// resolveViaValue resolves a scope value THROUGH a record (kafe 10.76).
//
// `ref` is an id or natural key of `via` (`"<module>.<entity>"`); the returned
// value is that record's `viaField`. A caller-supplied reference that does not
// resolve, or a record that carries no value for the field, is an ERROR — never
// an empty scope, which would fail open.
//
// One implementation serves both directions on purpose: the read scope
// (`from: route, via: …`) and the write pin (`create_scope`) ask the same
// question — "what does this reference say the value is?" — and two copies of
// that question would drift, with the drifting copy being the one that stops
// enforcing.
func (f *HandlerFactory) resolveViaValue(ctx context.Context, via, viaField, ref string) (string, error) {
	normalized, ok := spec.NormalizeEntityRef(via)
	if !ok {
		return "", fmt.Errorf("via %q is not a valid entity reference", via)
	}
	viaModule, viaEntity, _ := strings.Cut(normalized, "/")
	store, err := f.registry.GetEntityStore(viaModule, viaEntity)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %s: %w", via, err)
	}
	// GetByID handles both UUID v7 ids and natural keys, so a reference written
	// as a human-readable code resolves the same way as an id.
	rec, err := store.GetByID(ctx, db.GetByIDParams{
		WorkspaceID: workspaceFromContext(ctx),
		ID:          ref,
	})
	if err != nil || rec == nil {
		return "", fmt.Errorf("%s %q: %w", via, ref, errViaNotFound)
	}
	value := createScopeString(rec.Data[viaField])
	if value == "" {
		return "", fmt.Errorf("%s %q carries no %q", via, ref, viaField)
	}
	return value, nil
}

// createScopeString renders a payload/map value as the string used for scope
// comparison. Scopes compare identifiers (ids, codes, natural keys), which are
// strings on the wire; a number is rendered so an integer-coded dimension still
// compares instead of silently reading as absent.
func createScopeString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

// grantRowPredicates resolves the row scope a ROLE GRANT attaches to one action
// into row predicates (kafe 10.67, GAP-08).
//
// This is the half that did not exist: `row_scope` on the entity filters by WHO
// (session/route attributes) and is per-ENTITY, so it cannot say "this role sees
// only paid orders" without blinding the cashier to their own drafts. A grant is
// per (role, action), which is the granularity the rule needs.
//
// `action` is the CRUD action name; the permission it maps to is the same
// `{module}.{plural}.{action}` the materializer produces (and §8.6 makes
// normative), so a grant and its enforcement can never disagree on the name.
func (f *HandlerFactory) grantRowPredicates(r *http.Request, es *spec.EntitySpec, module, entity, action string) ([]db.RowPredicate, error) {
	identity := IdentityFromContext(r.Context())
	if identity == nil || len(identity.Roles) == 0 {
		return nil, nil
	}
	if f.grantScopeLookup == nil {
		return nil, nil
	}
	perm := entityActionPermission(module, entity, es, action)
	scope, err := f.grantScopeLookup(r.Context(), identity.WorkspaceID, identity.App, identity.Roles, perm)
	if err != nil {
		// A declared row restriction that cannot be applied is a denial, never a
		// silently unscoped read (see auth.PermissionResolver.GrantScope).
		return nil, err
	}
	if len(scope) == 0 {
		return nil, nil
	}
	return f.filterSpecsToPredicates(r, es, scope, "grant row scope")
}

// rowPredicatesFor is the single entry point for "which rows may this request
// touch?" — entity `row_scope` AND the caller's grant row scope, combined.
//
// Both are kept as predicates rather than being merged into the filter map so
// that two restrictions on the SAME field are ANDed in SQL instead of one
// silently replacing the other. That is the difference between "narrower than
// declared" (safe, visible) and "wider than declared" (a leak).
func (f *HandlerFactory) rowPredicatesFor(r *http.Request, es *spec.EntitySpec, module, entity, action string) ([]db.RowPredicate, error) {
	entityPreds, err := f.entityRowScopePredicates(r, es, module, entity)
	if err != nil {
		return nil, err
	}
	grantPreds, err := f.grantRowPredicates(r, es, module, entity, action)
	if err != nil {
		return nil, err
	}
	return append(entityPreds, grantPreds...), nil
}

// entityActionPermission builds the canonical `{module}.{plural}.{action}`
// permission (01-core-basic.md §8.6). Every layer that needs to name a
// permission goes through here or through spec.QualifyPermission, so the name
// used to enforce is the name the materializer produced.
func entityActionPermission(module, entity string, es *spec.EntitySpec, action string) string {
	plural := entity + "s"
	if es != nil && es.Plural != "" {
		plural = es.Plural
	}
	return module + "." + plural + "." + action
}

// scopeLiteralValue converts a manifest-declared literal into the value the
// storage layer expects.
//
// A literal is written as a single string in YAML, but the operator decides the
// shape: `in`/`nin` need a LIST and `between` needs exactly two bounds. Both are
// written comma-separated (`value: "paid,in_kitchen"`) because a row scope is
// declared in one line and a list of one is still a common case. Other operators
// take the string as-is — the storage layer coerces per field type (numbers,
// booleans, dates), so `value: "true"` works on a boolean column.
func scopeLiteralValue(op, raw string) (any, error) {
	switch op {
	case "in", "nin":
		parts := strings.Split(raw, ",")
		out := make([]any, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("literal %q yields an empty value list", raw)
		}
		return out, nil
	case "between":
		parts := strings.Split(raw, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("literal %q must contain exactly two comma-separated bounds", raw)
		}
		return []any{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])}, nil
	default:
		return raw, nil
	}
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

// resolveRouteScopeValue turns a `from: route` scope's declared parameter into
// the value the caller is filtered by.
//
// It is the ONE place that answers "what value does this route scope produce",
// shared by both readers of a route scope: the predicate path
// (filterSpecsToPredicates, which serves entity `row_scope` and grant scopes) and
// the public-grant path (applyPublicScope). Two copies of this question already
// drifted once — `applyPublicScope` kept taking the raw parameter after the
// predicate path learned about `via`, so a price list scoped by a session
// resolved to "the current branch" on one path and to "the literal session id"
// on the other. The second reading matched nothing, which is at least visible;
// the same drift with a token-derived scope would have matched everything.
//
// A missing parameter is an error, never an empty filter: the failure mode of a
// scope must be "denied", not "unscoped".
func (f *HandlerFactory) resolveRouteScopeValue(r *http.Request, sc *spec.FilterSpec) (string, error) {
	param := sc.Param
	if param == "" {
		param = sc.Field
	}
	value := r.URL.Query().Get(param)
	if value == "" {
		return "", fmt.Errorf("missing %q request parameter", param)
	}
	// `via` turns the parameter into a REFERENCE: the value is read from the
	// named record rather than trusted from the query string (kafe 10.76). The
	// caller states which session it is — already visible in the URL — and never
	// which branch it may see.
	if sc.Via != "" {
		return f.resolveViaValue(r.Context(), sc.Via, sc.ViaField, value)
	}
	return value, nil
}

// applyPublicScope merges a public grant's row scope into a list query (#45).
//
// The grant says WHICH entity an anonymous caller may read; this says WHICH ROWS.
// The value comes from a request parameter (e.g. an unguessable guest token) —
// the token IS the credential — and a missing one fails closed, exactly like an
// entity `row_scope` with `from: route`.
//
// It applies to callers AUTHORIZED BY THE GRANT — anonymous ones, and signed-in
// ones with no permission of their own for this route (see
// RequirePermissionOrAnonymous). A caller that holds the permission is governed
// by it plus the entity's own `row_scope` instead: applying the grant to such a
// caller would filter the cashier's POS list by a token it does not carry.
func (f *HandlerFactory) applyPublicScope(r *http.Request, filters map[string]db.FilterOp) (map[string]db.FilterOp, error) {
	scope := publicScopeFromContext(r.Context())
	if len(scope) == 0 {
		return filters, nil
	}
	if IdentityFromContext(r.Context()) != nil && !isPublicGrantAuth(r.Context()) {
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
		// Same resolution as the predicate path — including a `via` lookup, so a
		// scope derived from a reference is enforced identically whichever path
		// carries it.
		value, err := f.resolveRouteScopeValue(r, sc)
		if err != nil {
			return nil, fmt.Errorf(
				"this entity is readable anonymously only together with a %q request parameter — refusing to list every row",
				routeScopeParam(sc))
		}
		filters[sc.Field] = db.FilterOp{Op: op, Value: value}
	}
	return filters, nil
}

// routeScopeParam names the parameter a route scope reads (default: the field).
func routeScopeParam(sc *spec.FilterSpec) string {
	if sc.Param != "" {
		return sc.Param
	}
	return sc.Field
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
