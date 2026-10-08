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

	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/auth"
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
	Username    string   `json:"username,omitempty"` // display identity (UserMenu/avatar)
	Workspace   string   `json:"workspace"`
	App         string   `json:"app,omitempty"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	// EmailVerified reports whether the signed-in user's email is verified
	// (account pre-hijacking protection). Omitted for anonymous callers.
	EmailVerified bool `json:"email_verified,omitempty"`
	// OAuthProvider is the external identity linked to this account (e.g.
	// "google"). Empty = password-only account. Omitted for anonymous callers.
	OAuthProvider string `json:"oauth_provider,omitempty"`
	// Context is the session context this caller is ACTING IN (backend §8.7):
	// the single role, and the dimension value the session is bound to. Omitted
	// for a boundary-less session (owner / service account).
	//
	// It is reported because a client cannot otherwise answer the two questions a
	// switcher asks — "which context am I in?" and "how do I leave it?" — from
	// `roles` alone: `roles` says what the principal IS, never what the session
	// is scoped to, and with a context-scoped session the token deliberately
	// carries one role plus its `attrs`.
	Context *metaContext `json:"context,omitempty"`
	// ContextChoices lists every context this principal MAY act in, so a client
	// can offer the switch without first triggering a 409 to discover them.
	// Same shape as the `choices` a 409 CONTEXT_REQUIRED carries, and validated
	// by the same server code when one is submitted.
	ContextChoices []auth.ContextChoice `json:"context_choices,omitempty"`
}

// metaContext is the (role, dimension, value) triple a session acts in.
type metaContext struct {
	Role      string `json:"role"`
	Dimension string `json:"dimension,omitempty"`
	Value     string `json:"value,omitempty"`
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
	// AcceptsLogin reports whether this App presents an auth entry point — its
	// resolved `chrome.auth` is not `none` (ui.ChromeAcceptsLogin). Login is
	// per-App (plan app-scoped-login.md D1/D3), and this is the same test the
	// login endpoint enforces, so the client never has to infer it from
	// `access` (a public App may still accept login — e.g. the registry portal).
	AcceptsLogin bool `json:"accepts_login"`
	// StackFamily is the shell implementation (e.g. react-shadcn).
	StackFamily string `json:"stack_family,omitempty"`
	// PersistBackend is the entity persist backend (e.g. jsonb-persist).
	PersistBackend string `json:"persist_backend,omitempty"`
	// Version/Vendor are publish metadata (07-marketplace.md) — surfaced so
	// operators/control plane can see which app version is live.
	Version string `json:"version,omitempty"`
	Vendor  string `json:"vendor,omitempty"`
}

// HandleMetaApps lists every resolved App in this workspace (name + root_url)
// — Core §4.4. The renderer fetches this once, matches the current
// window.location.pathname against each root_url, and uses the winning
// App's name as the `app` query param on subsequent /_meta/ui calls.
// Apps whose Workspaces allowlist excludes the current workspace are not
// listed (AppSpec.MountsWithin — plan named-workspaces.md).
func (b *RouterBuilder) HandleMetaApps() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws := workspaceFromContext(r.Context())
		names := make([]string, 0, len(b.apps))
		for name, a := range b.apps {
			if a.Spec != nil && a.Spec.MountsWithin(ws) {
				names = append(names, name)
			}
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
				AcceptsLogin:   ui.ChromeAcceptsLogin(a.Spec.AppRenderer, a.Spec.Chrome),
				StackFamily:    a.Spec.StackFamily,
				PersistBackend: a.Spec.PersistBackend,
				Version:        a.Spec.Version,
				Vendor:         a.Spec.Vendor,
			})
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: out,
			Meta: MetaSingle{RequestID: requestIDFromContext(r.Context()), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
	}
}

// loginAppCheck validates the App named by an App-scoped login (plan
// app-scoped-login.md D6). It returns an empty code when the App is a valid
// login target, or a machine code + message when it is not:
//
//   - APP_REQUIRED        — the workspace has no App to sign in to at all
//   - UNKNOWN_APP         — no such App, or it is not mounted in this
//     workspace (indistinguishable from missing — anti-enumeration)
//   - APP_PUBLIC_NO_LOGIN — the App advertises no auth entry point, so it
//     accepts no login (D3: a public App with `chrome.auth: none`)
func (b *RouterBuilder) loginAppCheck(workspace, name string) (string, string) {
	if len(b.apps) == 0 {
		return "APP_REQUIRED", "this workspace has no App to sign in to"
	}
	a, ok := b.apps[name]
	if !ok || a.Spec == nil || !a.Spec.MountsWithin(workspace) {
		return "UNKNOWN_APP", "unknown app " + name
	}
	// The App must present a way in. An auth-less App has no login form, and a
	// session minted for it could not be used (D3).
	if !ui.ChromeAcceptsLogin(a.Spec.AppRenderer, a.Spec.Chrome) {
		return "APP_PUBLIC_NO_LOGIN", "app " + name + " does not accept login"
	}
	return "", ""
}

// resolveAppContext picks which App a /_meta/ui request is scoped to: the
// `app` query param if given, the session's own App scope when the request is
// authenticated (plan app-scoped-login.md D7 — the App is not a free choice),
// or the workspace's only App if there's exactly one. Returns an error message
// when the request is ambiguous.
func (b *RouterBuilder) resolveAppContext(r *http.Request) (ui.AppContext, string) {
	if len(b.apps) == 0 {
		return ui.AppContext{}, ""
	}
	name := r.URL.Query().Get("app")
	if name == "" {
		if id := IdentityFromContext(r.Context()); id != nil && id.App != "" {
			name = id.App
		}
	}
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
	// Workspace allowlist (AppSpec.MountsWithin — plan named-workspaces.md):
	// an App not mounted in this workspace is indistinguishable from one
	// that does not exist (anti-enumeration — same message as above).
	if resolved.Spec != nil && !resolved.Spec.MountsWithin(workspaceFromContext(r.Context())) {
		return ui.AppContext{}, "unknown app " + name
	}
	return ui.AppContext{
		Name:            resolved.Name,
		Title:           resolved.Spec.Title,
		Logo:            resolved.Spec.Logo,
		RootURL:         resolved.Spec.RootURL,
		AppRenderer:     resolved.Spec.AppRenderer,
		Access:          string(resolved.Spec.Access),
		PublicEntities:  b.derivedPublicEntitiesFor(resolved),
		StackFamily:     resolved.Spec.StackFamily,
		PersistBackend:  resolved.Spec.PersistBackend,
		ThemeRef:        resolved.Spec.ThemeRef,
		Chrome:          resolved.Spec.Chrome,
		Auth:            resolved.Spec.Auth,
		PageTransition:  resolved.Spec.PageTransition,
		Confirm:         resolved.Spec.Confirm,
		Modules:         resolved.Modules,
		Menu:            resolved.Menu,
		RegisteredViews: resolved.Spec.RegisteredViews,
		Settings:        b.mergeRunningSettings(r.Context(), b.settings),
	}, ""
}

// derivedPublicEntitiesFor returns the anonymous allowlist implied by the
// surface this App exposes, or nil when the App is not public (the caller then
// uses its own checker). The pointer distinguishes "derived, possibly empty"
// from "not applicable" — an App with no public views must expose nothing, not
// fall back to the legacy module-wide grant.
func (b *RouterBuilder) derivedPublicEntitiesFor(app *formspec_app.ResolvedApp) *[]spec.PublicEntityDecl {
	if app == nil || app.Spec == nil || app.Spec.Access != spec.AppAccessPublic {
		return nil
	}
	if b.uiRegistry == nil || b.registry == nil {
		return nil // nothing to derive from — keep the caller's checker
	}
	decls := b.uiRegistry.DerivePublicGrants(b.listEntityDescriptors, ui.PublicGrantInput{
		Modules:         app.Modules,
		Menu:            app.Menu,
		RegisteredViews: app.Spec.RegisteredViews,
	})
	return &decls
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
// The unscoped `_admin` bundle (`?admin=true`) was REMOVED (plan
// app-scoped-login.md D4): a single binary permission that unlocked every
// module's entities was an attack surface, and the admin panel is no longer
// a surface. Callers must name the App they want.
func (b *RouterBuilder) HandleMetaUI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if b.uiRegistry == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "UI registry not configured")
			return
		}

		if r.URL.Query().Get("admin") == "true" {
			writeError(w, http.StatusBadRequest, "ADMIN_BUNDLE_REMOVED",
				"the unscoped admin bundle (?admin=true) was removed; use ?app=<name>")
			return
		}

		// A session is scoped to ONE App (D1), so the App cannot be a free
		// query param: a token issued for App A must never render App B's
		// bundle (plan app-scoped-login.md D7).
		if id := IdentityFromContext(r.Context()); id != nil && id.App != "" {
			if reqApp := r.URL.Query().Get("app"); reqApp != "" && reqApp != id.App {
				writeError(w, http.StatusForbidden, "APP_MISMATCH",
					"this session is scoped to app "+id.App)
				return
			}
		}

		appCtx, errMsg := b.resolveAppContext(r)
		if errMsg != "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", errMsg)
			return
		}

		var bundle *ui.Bundle
		// `?grants=true` serves the grants-editor bundle: app-scoped (only the
		// App's modules) but NOT permission-filtered — the role form must list
		// every page/action in the App so an admin can grant access to things
		// they may not personally hold. Gated by role-management permission.
		if r.URL.Query().Get("grants") == "true" {
			if !canManageRoles(callerChecker(r)) {
				writeError(w, http.StatusForbidden, "FORBIDDEN",
					"missing permission: formspec.core.roles.create/update")
				return
			}
			alwaysVisible := func(string) bool { return true }
			appCtx.Unfiltered = true
			bundle = b.uiRegistry.BuildBundle(b.listEntityDescriptors, alwaysVisible, appCtx)
		} else {
			// An `access: public` App serves its bundle to anonymous
			// callers, so there is no session to check permissions against —
			// but "no session" must not mean "everything". The App's
			// `public_entities` allowlist is exactly the answer to what an
			// anonymous caller may see, and skipping it shipped the WHOLE
			// mounted module set.
			//
			// Measured on kafe (`kafe-qr`, which mounts cafe-master +
			// cafe-order behind a narrow allowlist): the anonymous bundle
			// carried 13 entities including `cafe-master.members` (customer
			// phone numbers), `employees`, `menu-item-prices`,
			// `cafe-order.shifts` and `cash-movements`. The data endpoints
			// still enforced the allowlist, so no row leaked — but the
			// schema of private data was handed out and the SPA generated
			// routes for it.
			can := callerChecker(r)
			if appCtx.Access == string(spec.AppAccessPublic) {
				// The App's `public_entities` allowlist decides what an
				// anonymous caller may see; BuildBundle derives the checker
				// from appCtx.PublicEntities (it owns the route/entity
				// conventions the allowlist is expressed in).
				can = nil
			}
			bundle = b.uiRegistry.BuildBundle(b.listEntityDescriptors, can, appCtx)
		}

		// First-run setup flag: the workspace has no users yet → the SPA
		// redirects to the setup wizard (self-hosted prod bootstrap).
		// Non-fatal: a setup-detection failure must not break the bundle.
		if authService != nil {
			if required, err := authService.SetupRequired(r.Context(), workspaceFromContext(r.Context())); err == nil {
				bundle.SetupRequired = required
			}
		}
		// External auth providers (auth redesign Fase 5) — the login screen
		// renders a button per configured provider.
		bundle.OAuthProviders = oauthProviderNames()

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

// sessionContextOf reports the context a session is acting in, plus the contexts
// its principal may act in.
//
// Two sources, deliberately: the CHOICES come from the principal's assignments
// (the authority on what they may become), while the ACTIVE context is read off
// the session's own identity (`role` + `attrs`) — that is what authorization
// actually uses, so reporting it from anywhere else could disagree with the
// enforced boundary.
//
// The dimension NAME is recovered by matching the token's attribute against the
// assignments: the token carries only `{branch_id: "KFE-JKT-01"}`, and the
// switcher needs to say which dimension that is. When nothing matches (a revoked
// assignment, or a token issued before the assignment list changed) the raw
// attribute is reported rather than pretending the session has no boundary —
// the next refresh will ask for a choice anyway.
func sessionContextOf(id *auth.Identity, u *auth.User) (*metaContext, []auth.ContextChoice) {
	choices := make([]auth.ContextChoice, 0, len(u.Assignments))
	for _, a := range u.Assignments {
		if !a.Complete() {
			continue
		}
		choices = append(choices, auth.ContextChoice{
			ID: a.ID(), Role: a.Role, Dimension: a.Dimension, Value: a.Value,
		})
	}

	// A context-scoped session carries exactly one role; several means the
	// session is boundary-less (legacy union), so there is no single role to
	// report.
	role := ""
	if len(id.Roles) == 1 {
		role = id.Roles[0]
	}
	ctx := &metaContext{Role: role}
	for _, a := range choices {
		if a.Role != role {
			continue
		}
		if v, ok := id.Attributes[a.Dimension]; ok && v == a.Value {
			ctx.Dimension, ctx.Value = a.Dimension, a.Value
			break
		}
	}
	if ctx.Dimension == "" && len(id.Attributes) == 1 {
		for k, v := range id.Attributes {
			ctx.Dimension, ctx.Value = k, v
		}
	}
	if ctx.Role == "" && ctx.Dimension == "" {
		return nil, choices
	}
	return ctx, choices
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
				Username:    id.Username,
				Workspace:   id.WorkspaceID,
				App:         id.App,
				Roles:       id.Roles,
				Permissions: id.Permissions,
			}
			// Expose email-verification state (best-effort — the user record
			// may be gone or the service unwired).
			if authService != nil {
				if u, err := authService.GetUserByID(r.Context(), id.WorkspaceID, id.UserID); err == nil {
					me.EmailVerified = u.EmailVerified
					me.OAuthProvider = u.OAuthProvider
					me.Context, me.ContextChoices = sessionContextOf(id, u)
				}
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
