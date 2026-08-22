package api

import (
	"net/http"

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
