package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// surfaceFromPath returns the field-exclusion surface for a request path:
// "ui" for /_ui/, "public_api" for /api/v1/ (05-field-types.md §5.3).
func surfaceFromPath(path string) string {
	if isUISurface(path) {
		return "ui"
	}
	return "public_api"
}

// sanitizeData applies field-level security to a record's data map (todo 6.7):
//   - masked fields → replaced with a mask (6.7.5)
//   - fields with required_permission the caller lacks → removed (6.7.2)
//   - fields excluded from the current surface → removed (6.7.3)
//
// It returns a NEW map and never mutates the stored record.
func sanitizeData(entitySpec *spec.EntitySpec, identity *auth.Identity, surface string, data map[string]any) map[string]any {
	if entitySpec == nil || data == nil {
		return data
	}
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[k] = v
	}
	for _, f := range entitySpec.Fields {
		if _, ok := out[f.Name]; !ok {
			continue
		}
		// Surface exclusion (6.7.3): e.g. exclude: [public_api] hides the
		// field from the external surface but keeps it on the UI surface.
		if containsString(f.Exclude, surface) {
			delete(out, f.Name)
			continue
		}
		// Field-level required_permission (6.7.2): caller without the
		// permission does not see the field at all.
		if f.RequiredPermission != "" && (identity == nil || !identity.HasPermission(f.RequiredPermission)) {
			delete(out, f.Name)
			continue
		}
		// Masked (6.7.5): auto-mask in the response.
		if f.Masked {
			out[f.Name] = maskValue(out[f.Name])
		}
	}
	return out
}

// maskValue masks a scalar value, keeping a short prefix/suffix for
// recognizability (e.g. "ab****cd"). Non-strings are fully masked.
func maskValue(v any) any {
	s, ok := v.(string)
	if !ok {
		return "****"
	}
	if s == "" {
		return s
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}

// containsString reports whether list contains s.
func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// forbiddenFieldWrites returns the fields in a request payload that the caller
// is not allowed to SET, because they lack the field's `required_permission`
// (05-field-types.md §5.3).
//
// §5.3 guards BOTH directions — "tidak boleh melihat **atau menyetel** field
// sensitif ini tanpa permission tambahan … dan penyetelannya di payload
// ditolak". Only the read half used to exist (sanitizeData strips the field from
// responses), which is the worse half to leave open: a caller who cannot READ a
// field could still WRITE it, so `salary: 999999` was accepted from a role that
// could not afterwards see what it had set.
//
// Rejection (not stripping) is what §5.3 specifies: silently dropping the value
// would answer 200 for a request the caller believes succeeded, and they would
// have no way to learn their input was ignored.
//
// Sorted for a deterministic message — the payload is a map, and a random order
// would make the same request report a different field each time.
func forbiddenFieldWrites(entitySpec *spec.EntitySpec, identity *auth.Identity, body map[string]any) []string {
	if entitySpec == nil || len(body) == 0 {
		return nil
	}
	var out []string
	for _, f := range entitySpec.Fields {
		if f.RequiredPermission == "" {
			continue
		}
		// Only the caller's INTENT matters: a field present in the payload is a
		// field they are trying to set. (An update merges onto the stored record,
		// so a stored value the caller never sent must not be flagged here.)
		if _, sent := body[f.Name]; !sent {
			continue
		}
		if identity != nil && identity.HasPermission(f.RequiredPermission) {
			continue
		}
		out = append(out, f.Name)
	}
	sort.Strings(out)
	return out
}

// denyForbiddenFieldWrites writes a 403 and returns false when the payload sets
// a field the caller may not set; returns true when the request may proceed.
func (f *HandlerFactory) denyForbiddenFieldWrites(w http.ResponseWriter, entitySpec *spec.EntitySpec, identity *auth.Identity, body map[string]any) bool {
	fields := forbiddenFieldWrites(entitySpec, identity, body)
	if len(fields) == 0 {
		return true
	}
	details := make([]ErrorDetailItem, 0, len(fields))
	for _, name := range fields {
		details = append(details, ErrorDetailItem{
			Level:   "field",
			Field:   name,
			Message: "setting field " + name + " requires its required_permission",
		})
	}
	writeErrorWithDetails(w, http.StatusForbidden, "FORBIDDEN",
		"missing permission to set field(s): "+strings.Join(fields, ", "), details)
	return false
}

// sanitize applies field-level security for a request against one entity's
// record data, resolving the entity spec + caller identity + surface.
func (f *HandlerFactory) sanitize(r *http.Request, module, entity string, data map[string]any) map[string]any {
	var entitySpec *spec.EntitySpec
	if f.specLookup != nil {
		entitySpec, _ = f.specLookup(module, entity)
	}
	identity := IdentityFromContext(r.Context())
	return sanitizeData(entitySpec, identity, surfaceFromPath(r.URL.Path), data)
}

// sanitizeList applies field-level security to every record in a list result.
func (f *HandlerFactory) sanitizeList(r *http.Request, module, entity string, records []db.EntityRecord) []db.EntityRecord {
	out := make([]db.EntityRecord, len(records))
	for i, rec := range records {
		rec.Data = f.sanitize(r, module, entity, rec.Data)
		out[i] = rec
	}
	return out
}
