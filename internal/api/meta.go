package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/primadi/forma/internal/ui"
)

// ─── Meta API (Frontend Spec §1.1, design doc §4.2) ───
//
// Read-only, same-origin endpoints the manifest-driven renderer boots from:
//
//	GET /{ws}/api/v1/_meta/ui                      → UI bundle (permission-filtered, ETag)
//	GET /{ws}/api/v1/_meta/me                      → caller identity + effective permissions
//	GET /{ws}/api/v1/_meta/entities/{module}/{name} → one full entity schema

// metaIdentity is the /_meta/me payload.
type metaIdentity struct {
	UserID      string   `json:"user_id"`
	Workspace   string   `json:"workspace"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// callerChecker returns a PermissionChecker for the request's identity.
// Anonymous callers hold no permissions.
func callerChecker(r *http.Request) ui.PermissionChecker {
	id := IdentityFromContext(r.Context())
	if id == nil {
		return func(string) bool { return false }
	}
	return id.HasPermission
}

// HandleMetaUI serves the full UI bundle with ETag/304 support. The bundle
// is permission-filtered per caller, so the ETag is computed per response.
func (b *RouterBuilder) HandleMetaUI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if b.uiRegistry == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "UI registry not configured")
			return
		}

		bundle := b.uiRegistry.BuildBundle(b.listEntityDescriptors, callerChecker(r))

		payload, err := json.Marshal(SingleResponse{
			Data: bundle,
			Meta: MetaSingle{RequestID: requestIDFromContext(r.Context()), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}

		// ETag over the data portion only (Meta carries request-varying fields).
		data, _ := json.Marshal(bundle)
		sum := sha256.Sum256(data)
		etag := `"` + hex.EncodeToString(sum[:16]) + `"`

		if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}
}

// HandleMetaMe serves the caller's identity and effective permissions —
// the source for client-side permission gating (Frontend §1.4, UX only;
// the server re-checks every call).
func (b *RouterBuilder) HandleMetaMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := IdentityFromContext(r.Context())
		me := metaIdentity{UserID: "anonymous", Roles: []string{}, Permissions: []string{}}
		if id != nil {
			me = metaIdentity{
				UserID:      id.UserID,
				Workspace:   id.WorkspaceID,
				Roles:       id.Roles,
				Permissions: id.Permissions,
			}
		}
		if me.Workspace == "" {
			me.Workspace = tenantFromContext(r.Context())
		}
		if me.Roles == nil {
			me.Roles = []string{}
		}
		if me.Permissions == nil {
			me.Permissions = []string{}
		}
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: me,
			Meta: MetaSingle{RequestID: requestIDFromContext(r.Context()), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
	}
}

// HandleMetaEntity serves one full entity schema (lazy-loaded by the
// renderer for heavy forms).
func (b *RouterBuilder) HandleMetaEntity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		module := chi.URLParam(r, "module")
		name := chi.URLParam(r, "name")

		info, ok := b.registry.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "entity not found: "+module+"/"+name)
			return
		}

		schema := ui.BuildEntitySchema(ui.EntityDescriptor{
			Module:      module,
			Name:        name,
			Description: info.Metadata.Description,
			Spec:        info.EntitySpec,
		})

		// Same visibility rule as the bundle: caller needs list or view.
		can := callerChecker(r)
		if !can(module+"."+schema.Plural+".list") && !can(module+"."+schema.Plural+".view") {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "entity not found: "+module+"/"+name)
			return
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: schema,
			Meta: MetaSingle{RequestID: requestIDFromContext(r.Context()), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
	}
}

// listEntityDescriptors adapts the entity registry to ui.EntityLister.
func (b *RouterBuilder) listEntityDescriptors() []ui.EntityDescriptor {
	infos := b.registry.ListEntities()
	out := make([]ui.EntityDescriptor, 0, len(infos))
	for _, info := range infos {
		specInfo, ok := b.registry.GetEntity(info.Module, info.Name)
		if !ok || specInfo.EntitySpec == nil {
			continue
		}
		out = append(out, ui.EntityDescriptor{
			Module:      info.Module,
			Name:        info.Name,
			Description: specInfo.Metadata.Description,
			Spec:        specInfo.EntitySpec,
		})
	}
	return out
}
