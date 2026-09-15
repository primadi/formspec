package api

import (
	"fmt"
	"net/http"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// applyRowScope merges an entity's declared `scope` into a list/aggregate query
// (S2, gaps #6/#9).
//
// Unlike a kind's `fixed_filters` — merged in the browser, and therefore
// omittable by any client that edits the request — Scope is authoritative and
// resolved here, server-side:
//
//   - `from: session` reads an attribute of the authenticated identity. The value
//     is never taken from the client, and it OVERRIDES any client-supplied filter
//     on the same field, so a branch cashier cannot widen the view by editing the
//     query string. An attribute that cannot be resolved fails closed: an empty
//     value must never degrade into "no filter".
//   - `from: route` reads a request query parameter (e.g. an unguessable guest
//     token). The token IS the credential, so a client-supplied value is expected
//     there; a missing parameter fails closed rather than listing everything.
//
// Returns the merged filter map (never nil when a scope is declared).
func (f *HandlerFactory) applyRowScope(r *http.Request, es *spec.EntitySpec, filters map[string]db.FilterOp) (map[string]db.FilterOp, error) {
	if es == nil || len(es.Scope) == 0 {
		return filters, nil
	}
	if filters == nil {
		filters = make(map[string]db.FilterOp, len(es.Scope))
	}
	identity := IdentityFromContext(r.Context())

	for i := range es.Scope {
		sc := &es.Scope[i]
		if sc.Field == "" {
			continue
		}
		op := sc.Op
		if op == "" {
			op = "eq"
		}

		switch sc.From {
		case "session":
			value := sessionAttr(identity, sc.Attr)
			if value == "" {
				return nil, fmt.Errorf(
					"row scope on %s: caller has no %q session attribute — refusing to list unscoped",
					sc.Field, scopeAttrLabel(sc.Attr))
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
