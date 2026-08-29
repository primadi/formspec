package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// ─── Meta API (Frontend Spec §1.1, design doc §4.2) ───
//
// Read-only, same-origin endpoints the manifest-driven renderer boots from:
//
//	GET /{ws}/_ui/_meta/ui                      → UI bundle (permission-filtered, ETag)
//	GET /{ws}/_ui/_meta/me                      → caller identity + effective permissions
//	GET /{ws}/_ui/_meta/entities/{module}/{name} → one full entity schema

// metaIdentity is the /_meta/me payload.
type metaIdentity struct {
	UserID      string   `json:"user_id"`
	Workspace   string   `json:"workspace"`
	App         string   `json:"app,omitempty"`
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

// appMetaSummary is one entry of the /_meta/apps payload.
type appMetaSummary struct {
	Name    string `json:"name"`
	RootURL string `json:"root_url"`
	// AppRenderer is the resolved App renderer archetype (frontend/05-app-kinds.md).
	AppRenderer string `json:"app_renderer,omitempty"`
	// Access is the resolved auth axis: private | public.
	Access string `json:"access,omitempty"`
	// StackFamily is the shell implementation (e.g. react-shadcn).
	StackFamily string `json:"stack_family,omitempty"`
	// PersistBackend is the entity persist backend (e.g. jsonb-persist).
	PersistBackend string `json:"persist_backend,omitempty"`
}

// HandleMetaApps lists every resolved App in this workspace (name + root_url)
// — Core §4.4. The renderer fetches this once, matches the current
// window.location.pathname against each root_url, and uses the winning
// App's name as the `app` query param on subsequent /_meta/ui calls.
func (b *RouterBuilder) HandleMetaApps() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		names := make([]string, 0, len(b.apps))
		for name := range b.apps {
			names = append(names, name)
		}
		sort.Strings(names)

		out := make([]appMetaSummary, 0, len(names))
		for _, name := range names {
			a := b.apps[name]
			out = append(out, appMetaSummary{
				Name:           a.Name,
				RootURL:        a.Spec.RootURL,
				AppRenderer:    a.Spec.AppRenderer,
				Access:         string(a.Spec.Access),
				StackFamily:    a.Spec.StackFamily,
				PersistBackend: a.Spec.PersistBackend,
			})
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: out,
			Meta: MetaSingle{RequestID: requestIDFromContext(r.Context()), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
	}
}

// resolveAppContext picks which App a /_meta/ui request is scoped to: the
// `app` query param if given, or the workspace's only App if there's exactly
// one. Returns an error message when the request is ambiguous.
func (b *RouterBuilder) resolveAppContext(r *http.Request) (ui.AppContext, string) {
	if len(b.apps) == 0 {
		return ui.AppContext{}, ""
	}
	name := r.URL.Query().Get("app")
	if name == "" {
		if len(b.apps) == 1 {
			for n := range b.apps {
				name = n
			}
		} else {
			return ui.AppContext{}, "workspace has more than one App — pass ?app=<name> (see /_meta/apps)"
		}
	}
	resolved, ok := b.apps[name]
	if !ok {
		return ui.AppContext{}, "unknown app " + name
	}
	return ui.AppContext{
		Name:           resolved.Name,
		Title:          resolved.Spec.Title,
		Logo:           resolved.Spec.Logo,
		RootURL:        resolved.Spec.RootURL,
		AppRenderer:    resolved.Spec.AppRenderer,
		Access:         string(resolved.Spec.Access),
		StackFamily:    resolved.Spec.StackFamily,
		PersistBackend: resolved.Spec.PersistBackend,
		Modules:        resolved.Modules,
		Menu:           resolved.Menu,
		Settings:       b.mergeRunningSettings(r.Context(), b.settings),
	}, ""
}

// mergeRunningSettings overlays the `app-setting` entity's running value over
// the manifest-declared settings (spec §10 Configuration Page pattern). The
// manifest `settings:` (kind: Config) is the default; the DB record is the
// admin-editable running value. Empty entity fields fall back to the manifest
// value, so the record only needs to store what the admin actually changed.
//
// The record is auto-created on first access via HandleFind's find-or-create
// (natural key "global"); until then (or if the entity isn't mounted) the
// manifest settings apply unchanged.
func (b *RouterBuilder) mergeRunningSettings(ctx context.Context, base *spec.Settings) *spec.Settings {
	if base == nil {
		base = spec.DefaultSettings()
	}
	store, err := b.registry.GetEntityStore("formspec.core", "app-setting")
	if err != nil {
		return base
	}
	rec, err := store.GetByID(ctx, db.GetByIDParams{
		WorkspaceID: workspaceFromContext(ctx),
		ID:          "global",
	})
	if err != nil || rec == nil {
		return base
	}

	out := spec.ResolveSettings(base)
	data := rec.Data
	if out.Currency == nil {
		out.Currency = &spec.CurrencySettings{}
	}
	if v, ok := data["currency_code"].(string); ok && v != "" {
		out.Currency.Code = v
	}
	if v, ok := data["currency_decimal_places"]; ok {
		if f, isNum := v.(float64); isNum {
			iv := int(f)
			out.Currency.DecimalPlaces = &iv
		}
	}
	if v, ok := data["currency_symbol"].(string); ok && v != "" {
		out.Currency.Symbol = v
	}
	if v, ok := data["locale"].(string); ok && v != "" {
		out.Locale = v
	}
	if v, ok := data["timezone"].(string); ok && v != "" {
		out.Timezone = v
	}
	if v, ok := data["date_format"].(string); ok && v != "" {
		out.DateFormat = v
	}
	if v, ok := data["decimal_scale"]; ok {
		if f, isNum := v.(float64); isNum {
			out.DecimalScale = int(f)
		}
	}
	if v, ok := data["rounding"].(string); ok && v != "" {
		out.Rounding = v
	}
	return out
}

// adminAccessPermission gates the `_admin` surface (Core §4.4 discussion):
// a single, binary "may see the unscoped, all-modules bundle" check — not a
// per-entity RBAC mechanism. Per-entity/per-view RBAC stays exclusive to
// authored Apps (menu.permissions).
const adminAccessPermission = "_admin.access"

// roleManagePermissions gate the `?grants=true` bundle variant: the caller
// must be able to manage roles (create or update) in the App. The grants
// editor is an admin tool — it must show every page/action in the App
// regardless of the caller's own permissions, so the bundle it consumes is
// app-scoped but NOT permission-filtered.
var roleManagePermissions = []string{
	"formspec.core.roles.create",
	"formspec.core.roles.update",
}

// canManageRoles reports whether the caller holds any role-management
// permission (create or update on formspec.core.role).
func canManageRoles(can ui.PermissionChecker) bool {
	for _, p := range roleManagePermissions {
		if can(p) {
			return true
		}
	}
	return false
}

// HandleMetaUI serves the full UI bundle with ETag/304 support. The bundle
// is permission-filtered per caller and scoped to one resolved App (see
// resolveAppContext), so the ETag is computed per response.
//
// `?admin=true` requests the `_admin` surface's bundle instead: unscoped by
// any App (every module's entities, Core §4.4 — _admin isn't App-scoped)
// and unfiltered by per-entity list/view permission (the binary
// adminAccessPermission gate is the only check). Gated separately since
// _admin has no AppContext to resolve in the first place.
func (b *RouterBuilder) HandleMetaUI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if b.uiRegistry == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "UI registry not configured")
			return
		}

		var bundle *ui.Bundle
		if r.URL.Query().Get("admin") == "true" {
			if !callerChecker(r)(adminAccessPermission) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "missing permission: "+adminAccessPermission)
				return
			}
			alwaysVisible := func(string) bool { return true }
			bundle = b.uiRegistry.BuildBundle(b.listEntityDescriptors, alwaysVisible, ui.AppContext{})
		} else {
			appCtx, errMsg := b.resolveAppContext(r)
			if errMsg != "" {
				writeError(w, http.StatusBadRequest, "BAD_REQUEST", errMsg)
				return
			}
			// `?grants=true` serves the grants-editor bundle: app-scoped
			// (only the App's modules) but NOT permission-filtered — the
			// role form must list every page/action in the App so an admin
			// can grant access to things they may not personally hold.
			// Gated by role-management permission (create/update).
			if r.URL.Query().Get("grants") == "true" {
				if !canManageRoles(callerChecker(r)) {
					writeError(w, http.StatusForbidden, "FORBIDDEN",
						"missing permission: formspec.core.roles.create/update")
					return
				}
				alwaysVisible := func(string) bool { return true }
				bundle = b.uiRegistry.BuildBundle(b.listEntityDescriptors, alwaysVisible, appCtx)
			} else {
				// An `access: public` App is entirely public (frontend/
				// 05-app-kinds.md §1): its bundle ships to anonymous callers with
				// every entity in its modules visible. Private Apps keep
				// per-entity permission filtering.
				can := callerChecker(r)
				if appCtx.Access == string(spec.AppAccessPublic) {
					can = func(string) bool { return true }
				}
				bundle = b.uiRegistry.BuildBundle(b.listEntityDescriptors, can, appCtx)
			}
		}

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
				App:         id.App,
				Roles:       id.Roles,
				Permissions: id.Permissions,
			}
		}
		if me.Workspace == "" {
			me.Workspace = workspaceFromContext(r.Context())
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

// HandleMetaVersion serves the current spec version — a lightweight polling
// endpoint the frontend uses to detect when the meta bundle has changed
// (e.g. after a spec hot-reload). Returns { spec_version: N }.
func (b *RouterBuilder) HandleMetaVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version := int64(0)
		if b.specVersionFn != nil {
			version = b.specVersionFn()
		}
		writeJSON(w, http.StatusOK, map[string]int64{"spec_version": version})
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
