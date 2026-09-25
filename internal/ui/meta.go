package ui

import (
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// ─── Meta API payloads (design doc §4.2) ───
//
// The renderer boots from one GET /_meta/ui round-trip: entity schemas
// (for UI derivation, D17) + all authored UI manifests, filtered by the
// caller's effective permissions. Derivation itself happens client-side.

// EntitySchema is the renderer-facing subset of a Document manifest.
type EntitySchema struct {
	Module         string             `json:"module"`
	Name           string             `json:"name"`
	Plural         string             `json:"plural"`
	Description    string             `json:"description,omitempty"`
	Characteristic string             `json:"characteristic,omitempty"`
	LabelField     string             `json:"label_field"`
	Fields         []spec.Field       `json:"fields"`
	StateMachine   *spec.StateMachine `json:"state_machine,omitempty"`
	Actions        []ActionSummary    `json:"actions"`
	Lifecycle      string             `json:"lifecycle"` // plain_crud | two_step_autosave (§1.7)
	HasQuickSubmit bool               `json:"has_quick_submit,omitempty"`
	Exposed        bool               `json:"exposed"`
	// AuthorizedActions is the set of entity actions THIS caller may perform,
	// resolved server-side with the same checker that decides whether the
	// entity ships at all.
	//
	// Without it the renderer has to GUESS: it built every derived CRUD route
	// and every action button from the entity's lifecycle alone, so a public
	// App granted only `[list, find]` still rendered a working-looking
	// "Create Menu Category" modal whose submit answered 401 (kafe 10.23), and
	// a cashier without `create` was shown a "New" button that opened a form
	// with no Save button (kafe 10.19).
	//
	// nil means "not resolved" (an older server, or a bundle built without a
	// permission checker) — the renderer then falls back to the caller's own
	// permission list. An empty but non-nil slice means "resolved: nothing".
	AuthorizedActions []string `json:"authorized_actions,omitempty"`
}

// ActionSummary is the renderer-facing view of one entity action.
type ActionSummary struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Permission  string             `json:"permission"`
	HasParams   bool               `json:"has_params,omitempty"`
	UI          *spec.ActionUIHint `json:"ui,omitempty"`
}

// AppSummary identifies which resolved App a Bundle was built for (Core §4.4).
type AppSummary struct {
	Name string `json:"name"`
	// Title is the human-readable display name (spaces allowed) for the
	// shell brand bar and document.title; falls back to Name when absent.
	Title string `json:"title,omitempty"`
	// Logo is the brand mark icon name (lucide) shown next to the title.
	Logo    string `json:"logo,omitempty"`
	RootURL string `json:"root_url"`
	// AppRenderer is the resolved App renderer archetype (frontend/
	// 05-app-kinds.md): sidebar-nav | topnav | no-nav. The renderer picks the
	// shell chrome for the whole App surface.
	AppRenderer string `json:"app_renderer,omitempty"`
	// Access is the resolved auth axis (frontend/05-app-kinds.md §1):
	// private | public. Public Apps boot anonymously.
	Access string `json:"access,omitempty"`
	// StackFamily is the shell implementation (frontend/03-renderer-kind.md),
	// e.g. react-shadcn.
	StackFamily string `json:"stack_family,omitempty"`
	// PersistBackend is the entity persist backend (backend/04-persist-
	// backend.md), e.g. jsonb-persist.
	PersistBackend string `json:"persist_backend,omitempty"`
	// Theme is the App's resolved default theme (frontend/05-app-kinds.md §6
	// theme binding): the Theme kind name from App.spec.theme_ref. The
	// renderer auto-applies it unless the user has picked a theme themselves.
	Theme string `json:"theme,omitempty"`
	// PageTransition is the resolved page-to-page navigation animation mode
	// (App.spec.page_transition): none | fade | slide. Empty/unknown values
	// resolve to "fade" — renderers read final values and never guess.
	PageTransition string `json:"page_transition,omitempty"`
	// Confirm is the App-wide default confirm-dialog configuration
	// (plan confirm-dialogs.md) — raw manifest declaration, passed through:
	// nil verb = off, "" = explicitly off, non-empty = dialog message.
	Confirm *ConfirmConfig `json:"confirm,omitempty"`
	// Chrome is the resolved, effective chrome composition (frontend/
	// 05-app-kinds.md §4.1) — archetype defaults already applied. Renderers
	// read these final values and never guess. Always non-nil.
	Chrome *ChromeConfig `json:"chrome"`
	// Auth is the resolved auth screen configuration (plan
	// docs_internal/plan/auth-screens-spec-driven.md) — each slot is the
	// App's override (App.spec.auth) or the framework default
	// (formspec.core/<slot>). Renderers read these final refs and never
	// guess. Nil when the App declares no auth overrides.
	Auth *AuthConfig `json:"auth,omitempty"`
}

// ConfirmConfig is the App-wide default confirm-dialog configuration shipped
// on the bundle (plan confirm-dialogs.md). Pointer semantics preserved from
// the manifest: nil = off, "" = explicitly off (opt out), non-empty = the
// dialog message. Forms resolve their own override on top.
type ConfirmConfig struct {
	Create *string `json:"create,omitempty"`
	Update *string `json:"update,omitempty"`
	Delete *string `json:"delete,omitempty"`
}

// ChromeConfig is the effective chrome composition for one App (frontend/
// 05-app-kinds.md §4.1). Resolved by resolveChrome: every "auto" in the
// manifest is replaced by the archetype's own default, so these are final
// values — show/hide, menu/none, links/button/none.
type ChromeConfig struct {
	Brand         string `json:"brand"`          // show | hide
	Nav           string `json:"nav"`            // menu | none
	Auth          string `json:"auth"`           // links | button | none
	Footer        string `json:"footer"`         // show | hide
	Breadcrumbs   string `json:"breadcrumbs"`    // show | hide
	ThemeSwitcher string `json:"theme_switcher"` // show | hide
	// ProfileRoute is the in-app route to the signed-in user's profile page
	// (opt-in via manifest `profile_route`; empty = no Profile menu item).
	ProfileRoute string `json:"profile_route,omitempty"`
}

// AuthConfig is the resolved auth screen configuration for one App (plan
// docs_internal/plan/auth-screens-spec-driven.md). Each slot is a `kind:
// Page` reference in `module/name` form — the App's override when declared
// (App.spec.auth), otherwise the framework default (formspec.core/<slot>).
// ChromeAuth is a component asset reference replacing the default chrome
// auth area; empty = the chrome.auth value drives the default AuthArea.
type AuthConfig struct {
	LoginPage          string `json:"login_page,omitempty"`
	SetupPage          string `json:"setup_page,omitempty"`
	ChangePasswordPage string `json:"change_password_page,omitempty"`
	ResetPasswordPage  string `json:"reset_password_page,omitempty"`
	OAuthCallbackPage  string `json:"oauth_callback_page,omitempty"`
	ChromeAuth         string `json:"chrome_auth,omitempty"`
}

// Default auth page slots (module formspec.core) — the framework's spec
// defaults used when an App declares no override for a slot.
const (
	DefaultLoginPage          = "formspec.core/login"
	DefaultSetupPage          = "formspec.core/setup"
	DefaultChangePasswordPage = "formspec.core/change-password"
	DefaultResetPasswordPage  = "formspec.core/reset-password"
	DefaultOAuthCallbackPage  = "formspec.core/oauth-callback"
)

// resolveAuth fills every auth slot with the App's override when declared,
// otherwise the framework default (formspec.core/<slot>). Returns nil when
// the App declares no auth overrides at all (renderers then use their
// built-in defaults).
func resolveAuth(a *spec.AppAuth) *AuthConfig {
	if a == nil {
		return nil
	}
	cfg := &AuthConfig{
		LoginPage:          DefaultLoginPage,
		SetupPage:          DefaultSetupPage,
		ChangePasswordPage: DefaultChangePasswordPage,
		ResetPasswordPage:  DefaultResetPasswordPage,
		OAuthCallbackPage:  DefaultOAuthCallbackPage,
	}
	if a.LoginPage != "" {
		cfg.LoginPage = a.LoginPage
	}
	if a.SetupPage != "" {
		cfg.SetupPage = a.SetupPage
	}
	if a.ChangePasswordPage != "" {
		cfg.ChangePasswordPage = a.ChangePasswordPage
	}
	if a.ResetPasswordPage != "" {
		cfg.ResetPasswordPage = a.ResetPasswordPage
	}
	if a.OAuthCallbackPage != "" {
		cfg.OAuthCallbackPage = a.OAuthCallbackPage
	}
	cfg.ChromeAuth = a.ChromeAuth
	return cfg
}

// AppContext scopes BuildBundle to one resolved App: which modules it mounts
// (Pages/Forms/Tables/... outside this set are excluded from the bundle) and
// its already-resolved menu tree (adopt nodes spliced, view leaves resolved
// to routes — see internal/app.Resolve). A zero-value AppContext (Modules
// nil) disables module filtering, for callers with no App concept yet.
type AppContext struct {
	Name        string
	Title       string
	Logo        string
	RootURL     string
	AppRenderer string
	Access      string
	// PublicEntities is the raw `public_entities` allowlist of an
	// `access: public` App. It is the ONLY statement of what an anonymous
	// caller may see, so BuildBundle uses it to build its permission checker
	// rather than treating every mounted entity as public.
	PublicEntities *[]spec.PublicEntityDecl
	StackFamily    string
	PersistBackend string
	// ThemeRef is the raw manifest declaration (App.spec.theme_ref) — the
	// Theme kind name applied as this App's default theme (§6 theme binding).
	ThemeRef string
	// Chrome is the raw manifest declaration (App.spec.chrome) — defaults
	// are applied later by resolveChrome, not here.
	Chrome *spec.AppChrome
	// Auth is the raw manifest declaration (App.spec.auth) — page refs for
	// the auth screens (login/setup/change-password/reset-password/oauth-
	// callback) + chrome auth area. Empty slots resolve to the framework
	// defaults (formspec.core) in BuildBundle.
	Auth *spec.AppAuth
	// PageTransition is the raw manifest declaration (App.spec.
	// page_transition) — resolved to the default (fade) in BuildBundle.
	PageTransition string
	// Confirm is the raw manifest declaration (App.spec.confirm) — the
	// App-wide default confirm dialogs (plan confirm-dialogs.md).
	Confirm *spec.AppConfirm
	Modules map[string]bool
	Menu    []spec.MenuItem
	// Settings is the resolved global presentation/config namespace (spec §10).
	// Always non-nil — resolved with standard defaults by the caller.
	Settings *spec.Settings
}

// allows reports whether a manifest belonging to module may ship in this
// App's bundle. "core" is a repo-wide convention for App-level/cross-module
// content that isn't owned by any one declared Module (e.g. a cross-module
// Dashboard, or an app-level Config) — it always ships, since it was never
// meant to be gated by spec.modules in the first place. "formspec.core" is
// the framework core module (internal/auth/core.go) — its system pages
// (auth screens, access management) always ship too.
func (c AppContext) allows(module string) bool {
	if c.Modules == nil || module == "core" || module == "formspec.core" {
		return true
	}
	return c.Modules[module]
}

// owns reports whether the App explicitly declares this module in
// spec.modules — the stricter question, for a PUBLIC App where anything
// implicitly-allowed (the framework's own admin surface) must NOT ship to
// anonymous callers.
func (c AppContext) owns(module string) bool {
	return c.Modules != nil && c.Modules[module]
}

// isAuthScreen reports whether a page is one of the framework's auth screens
// (routes under /_auth/). A public App needs these to sign a visitor in, while
// /access-management — the other page internal/auth/module ships — must not
// reach anonymous callers.
func isAuthScreen(e *Entry[spec.PageSpec]) bool {
	return e.Spec != nil && strings.HasPrefix(e.Spec.Route, "/_auth/")
}

// publicEntityChecker derives the permission checker for an ANONYMOUS caller of
// an `access: public` App from that App's `public_entities` allowlist.
//
// Why it lives here rather than at the HTTP layer: the allowlist is written in
// entity refs ("cafe-master/menu-item", "cafe-master.menu-item") while the
// bundle builder asks in permissions ("{module}.{plural}.{action}"). Deriving
// the checker next to the code that performs the checks keeps the two in step.
//
// A nil allowlist keeps the documented legacy behaviour (the App opted into
// `access: public` without narrowing anything), so the checker is absent and
// the caller supplies its own.
func publicEntityChecker(ix *entityIndex, decls *[]spec.PublicEntityDecl) PermissionChecker {
	if decls == nil {
		return nil
	}
	grants := map[string]map[string]bool{}
	for _, decl := range *decls {
		key, ok := spec.NormalizeEntityRef(decl.Entity) // "module/entity-name"
		if !ok {
			continue // validation rejects this; never widen access here
		}
		if grants[key] == nil {
			grants[key] = map[string]bool{}
		}
		for _, act := range decl.Actions {
			grants[key][act] = true
		}
	}
	return func(perm string) bool {
		module, plural, action, ok := splitEntityPermission(perm)
		if !ok {
			// Not an entity permission (a page permission, say) — pages are
			// judged separately, so do not filter on it here.
			return true
		}
		// The allowlist is written in the GRANT vocabulary ("find") while the
		// permission asks in the RESOURCE vocabulary ("view"), so translate
		// before looking the pair up. Without this `find` never matched: a grant
		// of `[find]` (e.g. `cafe-master.dining-table`) shipped the entity to
		// nobody, even though the router happily served its `find` route —
		// measured on kafe, the entity was absent from the bundle while
		// `GET /_ui/entity/.../dining-table/{id}` answered 200 anonymously.
		action = grantActionForPermission(action)
		// The allowlist names the ENTITY ("cafe-master/menu-item") while a
		// permission names the PLURAL ("cafe-master.menu-items.list"), so the
		// route segment is mapped back through the same index the router uses.
		// Skipping this step made every allowlisted entity fail to match
		// (measured: the bundle shipped 0 entities for `kafe-qr`).
		ref, ok := ix.moduleOfPlural(module, plural)
		if !ok {
			return false
		}
		acts, ok := grants[ref]
		if !ok {
			return false
		}
		return acts[action]
	}
}

// splitEntityPermission splits "{module}.{plural}.{action}", tolerating dotted
// module names ("formspec.core.roles.list") by taking the last two segments as
// plural and action.
func splitEntityPermission(perm string) (module, entity, action string, ok bool) {
	parts := strings.Split(perm, ".")
	if len(parts) < 3 {
		return "", "", "", false
	}
	action = parts[len(parts)-1]
	entity = parts[len(parts)-2]
	module = strings.Join(parts[:len(parts)-2], ".")
	return module, entity, action, true
}

// Bundle is the full /_meta/ui payload.
type Bundle struct {
	App        AppSummary                   `json:"app"`
	Entities   []EntitySchema               `json:"entities"`
	Pages      []*Entry[spec.PageSpec]      `json:"pages"`
	Forms      []*Entry[spec.FormSpec]      `json:"forms"`
	Tables     []*Entry[spec.TableSpec]     `json:"tables"`
	Dashboards []*Entry[spec.DashboardSpec] `json:"dashboards"`
	Widgets    []*Entry[spec.WidgetSpec]    `json:"widgets"`
	Reports    []*Entry[spec.ReportSpec]    `json:"reports"`
	Wizards    []*Entry[spec.WizardSpec]    `json:"wizards"`
	Kanbans    []*Entry[spec.KanbanSpec]    `json:"kanbans"`
	Timelines  []*Entry[spec.TimelineSpec]  `json:"timelines"`
	Menu       []spec.MenuItem              `json:"menu"`
	Prints     []*Entry[spec.PrintSpec]     `json:"prints"`
	Themes     []*Entry[spec.ThemeSpec]     `json:"themes"`
	Listings   []*Entry[spec.ListingSpec]   `json:"listings"`
	Calendars  []*Entry[spec.CalendarSpec]  `json:"calendars"`
	// ApprovalInboxes / NotificationCenters are zero-config pages — always
	// ship (their data is the caller's own pending approvals/notifications).
	ApprovalInboxes     []*Entry[spec.ApprovalInboxSpec]      `json:"approval_inboxes"`
	NotificationCenters []*Entry[spec.NotificationCenterSpec] `json:"notification_centers"`
	// Settings is the resolved global presentation/config namespace (spec §10).
	// Always present (resolved with standard defaults) so renderers never guess.
	Settings *spec.Settings `json:"settings"`
	// SetupRequired reports whether the workspace has no users yet — the SPA
	// redirects to the first-run setup wizard when true (self-hosted prod
	// bootstrap without formspec-ctl).
	SetupRequired bool `json:"setup_required"`
	// OAuthProviders lists the configured external auth provider names (auth
	// redesign Fase 5) — the login screen renders a button per provider.
	OAuthProviders []string `json:"oauth_providers,omitempty"`
}

// PermissionChecker reports whether the caller holds a permission
// (wildcards included). Implemented by auth.Identity.HasPermission.
type PermissionChecker func(permission string) bool

// EntityLister enumerates registered entities. Implemented by the entity
// registry (adapter in resource/formspec.go).
type EntityLister func() []EntityDescriptor

// EntityDescriptor pairs an entity spec with its metadata for bundle building.
type EntityDescriptor struct {
	Module      string
	Name        string
	Description string
	Spec        *spec.EntitySpec
}

// resolveChrome applies the chrome default matrix (frontend/05-app-kinds.md
// §4.1) on top of the raw manifest declaration. Unknown/empty values are
// treated as "auto" (strict validation happens at manifest load time via
// ValidateAppSpec + JSON Schema).
func resolveChrome(appRenderer string, c *spec.AppChrome) *ChromeConfig {
	cfg := &ChromeConfig{}
	if appRenderer == "no-nav" {
		cfg.Brand, cfg.Nav, cfg.Auth = spec.ChromeShow, spec.ChromeNone, spec.ChromeNone
		cfg.Footer, cfg.Breadcrumbs, cfg.ThemeSwitcher = spec.ChromeShow, spec.ChromeHide, spec.ChromeHide
	} else {
		// sidebar-nav / topnav: full chrome.
		cfg.Brand, cfg.Nav, cfg.Auth = spec.ChromeShow, spec.ChromeMenu, spec.ChromeLinks
		cfg.Footer, cfg.Breadcrumbs, cfg.ThemeSwitcher = spec.ChromeHide, spec.ChromeShow, spec.ChromeShow
	}
	if c == nil {
		return cfg
	}
	if c.Brand == spec.ChromeShow || c.Brand == spec.ChromeHide {
		cfg.Brand = c.Brand
	}
	if c.Nav == spec.ChromeMenu || c.Nav == spec.ChromeNone {
		cfg.Nav = c.Nav
	}
	if c.Auth == spec.ChromeLinks || c.Auth == spec.ChromeButton || c.Auth == spec.ChromeNone {
		cfg.Auth = c.Auth
	}
	if c.Footer == spec.ChromeShow || c.Footer == spec.ChromeHide {
		cfg.Footer = c.Footer
	}
	if c.Breadcrumbs == spec.ChromeShow || c.Breadcrumbs == spec.ChromeHide {
		cfg.Breadcrumbs = c.Breadcrumbs
	}
	if c.ThemeSwitcher == spec.ChromeShow || c.ThemeSwitcher == spec.ChromeHide {
		cfg.ThemeSwitcher = c.ThemeSwitcher
	}
	// ProfileRoute is a plain route string (no auto/show/hide matrix) —
	// pass through as-is.
	cfg.ProfileRoute = c.ProfileRoute
	return cfg
}

// resolvePageTransition normalizes the raw App.spec.page_transition
// declaration to the effective mode: empty or unknown values fall back to
// the default (fade). Strict validation happens at manifest load time
// (ValidateAppSpec + JSON Schema).
func resolvePageTransition(raw string) string {
	if spec.PageTransitionNames[raw] {
		return raw
	}
	return spec.DefaultPageTransition
}

// resolveConfirm maps the App-level confirm declaration onto the bundle's
// ConfirmConfig (plan confirm-dialogs.md). Nil verbs stay nil (off) — the
// renderer resolves the form-level override on top.
func resolveConfirm(c *spec.AppConfirm) *ConfirmConfig {
	if c == nil {
		return nil
	}
	return &ConfirmConfig{Create: c.Create, Update: c.Update, Delete: c.Delete}
}

// BuildBundle assembles the /_meta/ui payload for one caller, scoped to one
// resolved App (appCtx — Core §4.4). Manifests belonging to a module outside
// appCtx.Modules are excluded entirely, on top of permission filtering.
//
// Permission filtering (Frontend §1.4, defense in depth — the renderer
// re-filters per element): an entity schema ships when the caller can list
// or view it; entity-backed manifests follow their entity; pages with
// explicit permissions require at least one; navigation-only kinds
// (Dashboard, Theme, Wizard) always ship — their leaf elements are
// permission-gated client-side against /_meta/me. The menu itself comes
// straight from appCtx.Menu — already resolved (adopt nodes spliced, view
// leaves turned into routes) by internal/app.Resolve.
func (r *Registry) BuildBundle(entities EntityLister, can PermissionChecker, appCtx AppContext) *Bundle {
	r.mu.RLock()
	defer r.mu.RUnlock()

	menu := appCtx.Menu
	if menu == nil {
		menu = []spec.MenuItem{}
	}

	// Drop menu items whose view the caller cannot reach (kafe 10.10/10.11).
	//
	// An App menu is authored ONCE for every role, while entities are filtered
	// per role — so a curated item routinely points at an entity this user has
	// no grant for. Serving it produces a dead link, and because the SPA's
	// catch-all used to redirect silently, the user saw a DIFFERENT entity's
	// list and believed the click worked. The filter itself runs at the END of
	// this function, once every section is final (it asks which routes exist).
	//
	// AppContext.Access decides the SCOPE of what may ship at all:
	//
	//	public   the App is anonymous, so `can` is always-true (see
	//	         internal/api/meta.go) and only its own modules may ship —
	//	         EXCEPT the framework's auth screens, which a public App needs
	//	         to sign someone in. Without this, an `access: public` App
	//	         shipped the framework admin surface to anonymous visitors:
	//	         measured on kafe, `GET /kafe/_ui/_meta/ui?app=kafe-qr` returned
	//	         `formspec.core` entities (user, role, api-key, session) and the
	//	         `/access-management` page, and the anonymous browser rendered an
	//	         "Access Management" table whose columns include `Password Hash`.
	//	         The data endpoints still answered 401, so nothing leaked — but
	//	         the admin surface must not be reachable at all.
	//	private  per-entity permission filtering decides, as before.
	publicScope := appCtx.Access == string(spec.AppAccessPublic)
	// Built once and shared by the public-App allowlist check, the dashboard
	// widget check, and the menu pass — all of which ask the same question:
	// "which entity does this name refer to?".
	ix := newEntityIndex(entities)
	if publicScope {
		// An anonymous caller of a public App has no session, so the App's own
		// allowlist stands in for one. `internal/api` passes nil to say "derive
		// it"; a non-nil checker (the `?grants=true` editor) is left alone.
		if derived := publicEntityChecker(ix, appCtx.PublicEntities); can == nil && derived != nil {
			can = derived
		}
	}
	b := &Bundle{
		App: AppSummary{
			Name:           appCtx.Name,
			Title:          appCtx.Title,
			Logo:           appCtx.Logo,
			RootURL:        appCtx.RootURL,
			AppRenderer:    appCtx.AppRenderer,
			Access:         appCtx.Access,
			StackFamily:    appCtx.StackFamily,
			PersistBackend: appCtx.PersistBackend,
			Theme:          appCtx.ThemeRef,
			PageTransition: resolvePageTransition(appCtx.PageTransition),
			Chrome:         resolveChrome(appCtx.AppRenderer, appCtx.Chrome),
			Auth:           resolveAuth(appCtx.Auth),
			Confirm:        resolveConfirm(appCtx.Confirm),
		},
		Menu: menu, // Entities MUST be non-nil: a bundle whose caller can see no entity
		// still has to serialize `"entities": []`. Left nil it becomes
		// `null`, and the SPA iterates it unconditionally
		// (renderers/react-shadcn/src/stores/meta.ts `createLookups`) — a
		// `for (const e of bundle.entities)` on null throws
		// "e.entities is not iterable", which the ErrorBoundary turns into
		// a dead panel. Observed on kafe as role `dapur` opening the owner
		// dashboard: widgets renderer called getWidget → createLookups → boom.
		Entities: []EntitySchema{}, Pages: []*Entry[spec.PageSpec]{},
		Forms:               []*Entry[spec.FormSpec]{},
		Tables:              []*Entry[spec.TableSpec]{},
		Dashboards:          []*Entry[spec.DashboardSpec]{},
		Widgets:             []*Entry[spec.WidgetSpec]{},
		Reports:             []*Entry[spec.ReportSpec]{},
		Wizards:             []*Entry[spec.WizardSpec]{},
		Kanbans:             []*Entry[spec.KanbanSpec]{},
		Timelines:           []*Entry[spec.TimelineSpec]{},
		Prints:              []*Entry[spec.PrintSpec]{},
		Themes:              []*Entry[spec.ThemeSpec]{},
		Listings:            []*Entry[spec.ListingSpec]{},
		Calendars:           []*Entry[spec.CalendarSpec]{},
		ApprovalInboxes:     []*Entry[spec.ApprovalInboxSpec]{},
		NotificationCenters: []*Entry[spec.NotificationCenterSpec]{},
		Settings:            appCtx.Settings,
	}

	visible := map[string]bool{} // "module/name" → caller can see entity
	for _, d := range entities() {
		if !appCtx.allows(d.Module) {
			continue
		}
		if publicScope && !appCtx.owns(d.Module) {
			continue
		}
		schema := buildEntitySchema(d)
		listPerm := d.Module + "." + schema.Plural + ".list"
		viewPerm := d.Module + "." + schema.Plural + ".view"
		if !can(listPerm) && !can(viewPerm) {
			continue
		}
		// Which actions this caller may actually perform. Resolved with the
		// SAME checker that just decided the entity ships, so the bundle and
		// the endpoints can never disagree about it.
		schema.AuthorizedActions = authorizedActions(d, schema, can)
		visible[d.Module+"/"+d.Name] = true
		b.Entities = append(b.Entities, schema)
	}

	entityVisible := func(module, ref string) bool {
		m, n := module, ref
		if i := strings.IndexByte(ref, '.'); i > 0 {
			m, n = ref[:i], ref[i+1:]
		}
		return visible[m+"/"+n]
	}

	for _, k := range sortedKeys(r.Pages) {
		e := r.Pages[k]
		if !appCtx.allows(e.Module) || !allowedPage(e, can) {
			continue
		}
		if publicScope && !appCtx.owns(e.Module) && !isAuthScreen(e) {
			continue
		}
		b.Pages = append(b.Pages, e)
	}
	for _, k := range sortedKeys(r.Forms) {
		// Auth forms (spec.auth_action, plan custom-screens-spec-driven) have
		// no entity — include them directly: they carry no data, only the
		// declarative field layout for the public /_ui/auth/* endpoints.
		//
		// They are also the one thing internal/auth ships that a PUBLIC App
		// legitimately needs (a visitor must be able to sign in), which is why
		// they survive the publicScope gate that drops the rest of
		// `formspec.core` (notably /access-management).
		e := r.Forms[k]
		if !appCtx.allows(e.Module) {
			continue
		}
		if publicScope && !appCtx.owns(e.Module) && e.Spec.AuthAction == "" {
			continue
		}
		if e.Spec.AuthAction != "" || entityVisible(e.Module, e.Spec.Entity) {
			b.Forms = append(b.Forms, e)
		}
	}
	for _, k := range sortedKeys(r.Tables) {
		if e := r.Tables[k]; appCtx.allows(e.Module) && (!publicScope || appCtx.owns(e.Module)) && entityVisible(e.Module, e.Spec.Entity) {
			b.Tables = append(b.Tables, e)
		}
	}
	for _, k := range sortedKeys(r.Widgets) {
		e := r.Widgets[k]
		if !appCtx.allows(e.Module) {
			continue
		}
		if publicScope && !appCtx.owns(e.Module) {
			continue
		}
		if e.Spec.Entity == "" || entityVisible(e.Module, e.Spec.Entity) {
			b.Widgets = append(b.Widgets, e)
		}
	}
	for _, k := range sortedKeys(r.Reports) {
		e := r.Reports[k]
		if !appCtx.allows(e.Module) {
			continue
		}
		if e.Spec.RequiredPermission != "" && !can(qualifyPerm(e.Module, e.Spec.RequiredPermission)) {
			continue
		}
		if entityVisible(e.Module, e.Spec.Entity) {
			b.Reports = append(b.Reports, e)
		}
	}
	for _, k := range sortedKeys(r.Kanbans) {
		if e := r.Kanbans[k]; appCtx.allows(e.Module) && entityVisible(e.Module, e.Spec.Entity) {
			b.Kanbans = append(b.Kanbans, e)
		}
	}
	for _, k := range sortedKeys(r.Timelines) {
		if e := r.Timelines[k]; appCtx.allows(e.Module) && entityVisible(e.Module, e.Spec.Entity) {
			b.Timelines = append(b.Timelines, e)
		}
	}
	for _, k := range sortedKeys(r.Prints) {
		if e := r.Prints[k]; appCtx.allows(e.Module) && entityVisible(e.Module, e.Spec.Entity) {
			b.Prints = append(b.Prints, e)
		}
	}
	for _, k := range sortedKeys(r.Listings) {
		if e := r.Listings[k]; appCtx.allows(e.Module) && entityVisible(e.Module, e.Spec.Entity) {
			b.Listings = append(b.Listings, e)
		}
	}
	for _, k := range sortedKeys(r.Calendars) {
		if e := r.Calendars[k]; appCtx.allows(e.Module) && entityVisible(e.Module, e.Spec.Entity) {
			b.Calendars = append(b.Calendars, e)
		}
	}
	for _, k := range sortedKeys(r.ApprovalInboxes) {
		if e := r.ApprovalInboxes[k]; appCtx.allows(e.Module) {
			b.ApprovalInboxes = append(b.ApprovalInboxes, e)
		}
	}
	for _, k := range sortedKeys(r.NotificationCenters) {
		if e := r.NotificationCenters[k]; appCtx.allows(e.Module) {
			b.NotificationCenters = append(b.NotificationCenters, e)
		}
	}
	// A dashboard is an aggregate of widgets, so it is only as visible as the
	// widgets behind it: ship it when at least ONE placed widget survives.
	//
	// Measured on kafe as role `dapur`: the widget list was empty (no grant on
	// `cafe-order.order` / `cafe-report.*`), yet `owner-overview` still shipped,
	// so the role's only menu entry opened a page of four
	// "Widget definition not found" placeholders. An all-permission-filtered
	// dashboard is dead, not merely thin.
	// (ix is built above; the widget check and the menu pass share it.)
	for _, k := range sortedKeys(r.Dashboards) {
		e := r.Dashboards[k]
		if !appCtx.allows(e.Module) {
			continue
		}
		if publicScope && !appCtx.owns(e.Module) {
			continue
		}
		if r.dashboardHasVisibleWidget(e, ix, can) {
			b.Dashboards = append(b.Dashboards, e)
		}
	}
	for _, k := range sortedKeys(r.Wizards) {
		if e := r.Wizards[k]; appCtx.allows(e.Module) && (!publicScope || appCtx.owns(e.Module)) {
			b.Wizards = append(b.Wizards, e)
		}
	}
	for _, k := range sortedKeys(r.Themes) {
		b.Themes = append(b.Themes, r.Themes[k])
	}

	// ── Derived Page wrappers for public visual kinds ──
	// Every visual kind with public: true (default) that can be embedded as
	// a Page block gets an auto-generated Page wrapper — unless already
	// covered by an authored Page block. The derived Page provides a
	// standalone route so the kind can be navigated directly (or referenced
	// via `view` in menu items).
	//
	// Forms and Tables are block kinds — they appear inside PageBlocks.
	// Dashboard/Wizard/Kanban/Timeline/Report/Print already have their own
	// routes generated by the frontend router (renderers/react-shadcn/src/shell/router.tsx)
	// and don't need derived Pages.

	// Build set of (module, kind, ref) already covered by authored Pages.
	covered := map[string]bool{}
	for _, page := range b.Pages {
		m := page.Module
		for _, blk := range page.Spec.Blocks {
			if blk.Form != nil && blk.Form.Ref != "" {
				covered[m+"/form/"+blk.Form.Ref] = true
			}
			if blk.Table != nil && blk.Table.Ref != "" {
				covered[m+"/table/"+blk.Table.Ref] = true
			}
		}
		for _, tab := range page.Spec.Tabs {
			if tab.Form != nil && tab.Form.Ref != "" {
				covered[m+"/form/"+tab.Form.Ref] = true
			}
			if tab.Table != nil && tab.Table.Ref != "" {
				covered[m+"/table/"+tab.Table.Ref] = true
			}
		}
	}

	// Helper: create a single-block derived Page.
	makeDerivedPage := func(mod, name, route, title string, block spec.PageBlock, perms []string) *Entry[spec.PageSpec] {
		t := true
		return &Entry[spec.PageSpec]{
			Name:   name,
			Module: mod,
			Spec: &spec.PageSpec{
				Public:      &t,
				Route:       route,
				Title:       title,
				Blocks:      []spec.PageBlock{block},
				Permissions: perms,
			},
		}
	}

	for _, k := range sortedKeys(r.Forms) {
		e := r.Forms[k]
		if !appCtx.allows(e.Module) || !entityVisible(e.Module, e.Spec.Entity) {
			continue
		}
		if !spec.IsPublic(e.Spec.Public) {
			continue
		}
		key := e.Module + "/form/" + e.Name
		if covered[key] {
			continue
		}
		b.Pages = append(b.Pages, makeDerivedPage(
			e.Module,
			e.Name+"-page",
			"/"+e.Module+"/form/"+e.Name,
			nonEmpty(e.Description, e.Name),
			spec.PageBlock{Form: &spec.BlockRef{Ref: e.Name}},
			permForEntityBacked(e.Module, e.Spec.Entity, formActionPerm(e.Spec.Mode)),
		))
	}

	for _, k := range sortedKeys(r.Tables) {
		e := r.Tables[k]
		if !appCtx.allows(e.Module) || !entityVisible(e.Module, e.Spec.Entity) {
			continue
		}
		if !spec.IsPublic(e.Spec.Public) {
			continue
		}
		key := e.Module + "/table/" + e.Name
		if covered[key] {
			continue
		}
		b.Pages = append(b.Pages, makeDerivedPage(
			e.Module,
			e.Name+"-page",
			"/"+e.Module+"/table/"+e.Name,
			nonEmpty(e.Description, e.Name),
			spec.PageBlock{Table: &spec.BlockRef{Ref: e.Name}},
			permForEntityBacked(e.Module, e.Spec.Entity, "list"),
		))
	}

	// ── Filter the menu against the finished bundle ──
	//
	// An App menu is authored ONCE for every role, while entities, forms,
	// reports and dashboards are each filtered per role. A curated item
	// therefore routinely points at something this caller cannot open, and
	// serving it produces a dead link — historically one the SPA's catch-all
	// answered with a SILENT redirect, so the user saw a different entity's
	// list and believed the click worked.
	//
	// The rule is "does this route exist in THIS bundle", not "does the caller
	// hold a permission". The two differ exactly where it matters: a dashboard
	// whose widgets are all filtered out is absent from the bundle while the
	// caller may still hold a grant on it — asking about permissions would keep
	// a link to a route that was never registered.
	//
	// Order matters: this reads every section, so it must run last.
	b.Menu = r.filterMenu(b.Menu, b, can)

	return b
}

// entityIndex answers the two questions menu filtering asks: "what is the
// plural of entity X in module M?" and "which module/plural does this route
// display?".
//
// Built once per bundle from the SAME entity list the bundle itself uses, so the
// menu can never disagree with the entities actually being served.
type entityIndex struct {
	// pluralByRef["M/name"] = plural
	pluralByRef map[string]string
	// refByPlural["M/plural"] = "M/name"
	refByPlural map[string]string
	// modules holds every module that owns an entity.
	modules map[string]bool
}

func newEntityIndex(entities EntityLister) *entityIndex {
	ix := &entityIndex{
		pluralByRef: map[string]string{},
		refByPlural: map[string]string{},
		modules:     map[string]bool{},
	}
	for _, d := range entities() {
		plural := d.Spec.Plural
		if plural == "" {
			plural = d.Name + "s"
		}
		ix.pluralByRef[d.Module+"/"+d.Name] = plural
		ix.refByPlural[d.Module+"/"+plural] = d.Module + "/" + d.Name
		ix.modules[d.Module] = true
	}
	return ix
}

// isModule reports whether the module owns at least one entity. Used to decide
// whether a "/M/<segment>" route is a derived entity page (M is a module, so
// the segment MUST name one of its entities) or just an opaque authored route.
func (ix *entityIndex) isModule(module string) bool { return ix.modules[module] }

// pluralOf returns the plural (route segment) of a module-local entity.
func (ix *entityIndex) pluralOf(module, name string) (string, bool) {
	p, ok := ix.pluralByRef[module+"/"+name]
	return p, ok
}

// moduleOfPlural inverts the route segment back to (module, name).
func (ix *entityIndex) moduleOfPlural(module, plural string) (string, bool) {
	ref, ok := ix.refByPlural[module+"/"+plural]
	return ref, ok
}

// filterMenu removes a menu item when the caller may not have it, on two
// independent grounds, bottom-up: a group whose children all disappear goes
// with them.
//
//  1. ROUTE EXISTENCE. The bundle may not serve the route the item points at,
//     because the entity/kind behind it was filtered out for this caller.
//     Permission was the earlier approach and it kept links the bundle had
//     already dropped — measured on kafe as role `dapur`, whose sidebar offered
//     "Ringkasan Pemilik" (`/dashboard/owner-overview`) although
//     `dashboardHasVisibleWidget` had removed that dashboard (every one of its
//     widgets reads an entity the role cannot see). The link rendered the 404
//     page. A route that was never registered cannot be navigated to, whoever
//     asks.
//
//  2. `permissions:` on the item itself (RBAC). This is the ONLY place it is
//     enforced — enforced here so the item never reaches a caller who lacks it,
//     and so the check cannot drift from the one deciding the bundle (same
//     `can`). A client-side RBAC check would be bypassable and would silently
//     disagree with the server as soon as the two diverged; the client
//     therefore no longer looks at `permissions` at all (see
//     renderers/react-shadcn/src/hooks/useResolvedMenu.ts).
//
// `when:` is deliberately NOT evaluated here: it is a business condition that
// may depend on the clock, and the bundle body is ETag-hashed
// (internal/api/meta.go), so a time-dependent filter would invalidate the cache
// continuously. The client evaluates it.
//
// The `?admin=true` and `?grants=true` bundles pass an always-true checker
// (internal/api/meta.go), so both variants ignore `permissions` — intended: the
// grants editor must offer every page/action so an admin can grant things they
// do not personally hold.
//
// Items that are not a route at all are kept: a leaf with no `route` (`type:
// module` groups are already spliced by internal/app.Resolve) is not
// navigable, so there is nothing to check.
func (r *Registry) filterMenu(items []spec.MenuItem, b *Bundle, can PermissionChecker) []spec.MenuItem {
	out := make([]spec.MenuItem, 0, len(items))
	for _, item := range items {
		if len(item.Children) > 0 {
			item.Children = r.filterMenu(item.Children, b, can)
			if len(item.Children) == 0 {
				continue // group lost every child — nothing left to show
			}
			out = append(out, item)
			continue
		}
		if !menuItemAllowed(item, can) {
			continue
		}
		if item.Route == "" || r.routeExists(item.Module, item.Route, b) {
			out = append(out, item)
		}
	}
	return out
}

// menuItemAllowed reports whether the caller holds at least one of the item's
// declared permissions (any-of). An item with no `permissions:` is allowed.
func menuItemAllowed(item spec.MenuItem, can PermissionChecker) bool {
	if len(item.Permissions) == 0 {
		return true
	}
	for _, p := range item.Permissions {
		if can(p) {
			return true
		}
	}
	return false
}

// routeExists reports whether the bundle serves this route — i.e. whether the
// SPA registered a Route for it in buildRoutes
// (renderers/react-shadcn/src/shell/router.tsx). Every shape that function
// generates is covered here; the two must be kept in step, which is why the
// section checks come first and the entity conventions second.
func (r *Registry) routeExists(module, route string, b *Bundle) bool {
	parts := strings.Split(strings.Trim(route, "/"), "/")

	// 1. Authored pages: the bundle ships exactly the routes it carries.
	for _, e := range b.Pages {
		if e.Spec != nil && e.Spec.Route == route {
			return true
		}
	}

	// 2. Form/Table views: "/M/form/<n>" and "/M/table/<n>" — the shapes
	//    ResolveViewRoute generates for those two kinds.
	if len(parts) == 3 && parts[0] == module {
		switch parts[1] {
		case "form":
			return findEntry(b.Forms, module, parts[2]) != nil
		case "table":
			return findEntry(b.Tables, module, parts[2]) != nil
		}
	}

	// 3. Navigation kinds: /dashboard/<n>, /report/<n>, /kanban/<n>, …
	if len(parts) == 2 {
		if r.navigationPrefix(parts[0]) {
			return r.hasNavigationView(parts[1], parts[0], module, b)
		}
	}

	// 4. Derived entity routes: /<module>/<plural>[/new|/:id[/edit]]. The SPA
	//    generates these for every entity in the bundle, so the entity list —
	//    not the router — is the source of truth.
	if len(parts) >= 2 {
		for _, e := range b.Entities {
			if e.Module == parts[0] && e.Plural == parts[1] {
				return true
			}
		}
	}

	// 5. The App root — the SPA always renders a route for it.
	if root := strings.Trim(b.App.RootURL, "/"); root != "" && root == strings.Trim(route, "/") {
		return true
	}
	return false
}

// navigationPrefix reports whether a path segment names a navigation kind.
func (r *Registry) navigationPrefix(seg string) bool {
	switch seg {
	case "dashboard", "report", "kanban", "timeline", "calendar",
		"print", "widget", "wizard", "listing", "approval-inbox",
		"notification-center":
		return true
	}
	return false
}

// hasNavigationView reports whether the bundle ships the named navigation kind.
func (r *Registry) hasNavigationView(name, prefix, module string, b *Bundle) bool {
	switch prefix {
	case "dashboard":
		return findEntry(b.Dashboards, module, name) != nil
	case "report":
		return findEntry(b.Reports, module, name) != nil
	case "kanban":
		return findEntry(b.Kanbans, module, name) != nil
	case "timeline":
		return findEntry(b.Timelines, module, name) != nil
	case "calendar":
		return findEntry(b.Calendars, module, name) != nil
	case "print":
		return findEntry(b.Prints, module, name) != nil
	case "widget":
		return findEntry(b.Widgets, module, name) != nil
	case "wizard":
		return findEntry(b.Wizards, module, name) != nil
	case "listing":
		return findEntry(b.Listings, module, name) != nil
	case "approval-inbox":
		return findEntry(b.ApprovalInboxes, module, name) != nil
	case "notification-center":
		return findEntry(b.NotificationCenters, module, name) != nil
	}
	return false
}

// findEntry returns the named entry of a kind, or nil.
func findEntry[T any](entries []*Entry[T], module, name string) *Entry[T] {
	for _, e := range entries {
		if e.Name == name && (module == "" || e.Module == module) {
			return e
		}
	}
	return nil
}

// viewVerdict is what a menu route resolves to.
type viewVerdict int

const (
	// viewUnbacked — no entity behind the view (dashboard, wizard, custom asset
	// page, opaque authored route). "Not permission-checkable" is not
	// "forbidden": the item is kept.
	viewUnbacked viewVerdict = iota
	// viewBacked — an entity backs the view; require its read permission.
	viewBacked
	// viewDead — the route names one of ours (a navigation kind, or a module's
	// entity page) but the target does not exist for ANYONE, so no route was
	// generated. Dropped regardless of permission.
	viewDead
)

// dashboardHasVisibleWidget reports whether any widget placed on the dashboard
// resolves to a widget the caller may read. A widget with no entity (a static
// note, a metric the caller always sees) keeps the dashboard alive — this only
// drops a dashboard whose every placement is filtered out.
func (r *Registry) dashboardHasVisibleWidget(d *Entry[spec.DashboardSpec], ix *entityIndex, can PermissionChecker) bool {
	if d.Spec == nil || len(d.Spec.Widgets) == 0 {
		// No placements — nothing to filter, and the dashboard itself is not
		// entity-backed.
		return true
	}
	for _, w := range d.Spec.Widgets {
		e, ok := r.Widgets[w.Ref]
		if !ok || e.Module != d.Module || e.Spec == nil {
			// A ref this module does not own, or one whose spec is missing:
			// cannot judge, so do not hide.
			return true
		}
		if e.Spec.Entity == "" {
			return true
		}
		_, plural, v := r.entityRefBacking(ix, e.Module, e.Spec.Entity)
		if v != viewBacked || can(e.Module+"."+plural+".list") {
			return true
		}
	}
	return false
}

// entityRefBacking resolves an entity reference that may be cross-module
// ("cafe-master.member") or module-local ("member").
func (r *Registry) entityRefBacking(ix *entityIndex, module, ref string) (string, string, viewVerdict) {
	if ref == "" {
		return "", "", viewUnbacked
	}
	m, name := module, ref
	// Split at the LAST dot: module names may be dotted ("formspec.core.role").
	if i := strings.LastIndexByte(ref, '.'); i > 0 {
		m, name = ref[:i], ref[i+1:]
	}
	if plural, ok := ix.pluralOf(m, name); ok {
		return m, plural, viewBacked
	}
	// The manifest names a resource the bundle does not have — a broken
	// reference (the kafe `trial-balance` report pointed at a non-existent
	// `ledger` entity). Dead: no route backs it.
	return "", "", viewDead
}

// nonEmpty returns a if non-empty, otherwise b.
func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// formActionPerm maps a form mode to the entity permission action.
func formActionPerm(mode string) string {
	switch mode {
	case "edit", "view":
		return "update"
	default:
		return "create"
	}
}

// FormActionPerm is the exported form of formActionPerm, used by the auth
// materializer to derive a form's entity-action permission from its mode.
func FormActionPerm(mode string) string { return formActionPerm(mode) }

// permForEntityBacked derives the required_permission for an entity-backed
// view: {module}.{entity}.{action}. entityRef may be "customer" (module-local)
// or "billing.customer" (cross-module).
func permForEntityBacked(module, entityRef, action string) []string {
	m, name := module, entityRef
	if i := strings.IndexByte(entityRef, '.'); i >= 0 {
		m, name = entityRef[:i], entityRef[i+1:]
	}
	return []string{m + "." + name + "." + action}
}

// allowedPage checks a page's explicit permission list (any-of). Permission
// strings in page manifests may be module-relative ("visits.list").
func allowedPage(e *Entry[spec.PageSpec], can PermissionChecker) bool {
	if len(e.Spec.Permissions) == 0 {
		return true
	}
	for _, p := range e.Spec.Permissions {
		if can(qualifyPerm(e.Module, p)) {
			return true
		}
	}
	return false
}

// qualifyPerm prefixes a module-relative permission ("visits.list") with the
// manifest's module ("clinic.visits.list"). Already-qualified permissions
// (3+ segments) pass through unchanged.
func qualifyPerm(module, perm string) string {
	if strings.Count(perm, ".") >= 2 || module == "" {
		return perm
	}
	return module + "." + perm
}

// BuildEntitySchema builds the renderer-facing schema for one entity
// (also served alone via /_meta/entities/{module}/{name}).
func BuildEntitySchema(d EntityDescriptor) EntitySchema { return buildEntitySchema(d) }

func buildEntitySchema(d EntityDescriptor) EntitySchema {
	es := d.Spec
	plural := es.Plural
	if plural == "" {
		plural = d.Name + "s"
	}

	schema := EntitySchema{
		Module:         d.Module,
		Name:           d.Name,
		Plural:         plural,
		Description:    d.Description,
		Characteristic: string(es.Characteristic),
		LabelField:     labelField(es),
		Fields:         es.Fields,
		StateMachine:   es.StateMachine,
		Lifecycle:      lifecycle(es),
		Exposed:        len(es.Expose) > 0,
	}

	for _, a := range es.Actions {
		if a.Disabled {
			continue
		}
		perm := a.RequiredPermission
		if perm == "" {
			perm = d.Module + "." + plural + "." + a.Name
		} else {
			perm = qualifyPerm(d.Module, perm)
		}
		schema.Actions = append(schema.Actions, ActionSummary{
			Name:        a.Name,
			Description: a.Description,
			Permission:  perm,
			HasParams:   a.Params != nil && len(a.Params.Validate) > 0,
			UI:          a.UI,
		})
		if a.Name == "create-submit" {
			schema.HasQuickSubmit = true
		}
	}

	return schema
}

// entityActionPermission returns the permission a standard entity action
// requires, using the SAME mapping the router registers routes with
// (`internal/api/descriptor.go` StandardRESTActions). The two vocabularies
// differ on purpose — the UI says "find"/"edit", the resource says
// "view"/"update" — and the renderer used to spell the permission out by hand
// from the UI word, so a cashier holding `.update` was denied the Edit button
// that `.edit` (which never exists) failed to match.
func entityActionPermission(module, plural, action string) string {
	base := module + "." + plural + "."
	switch action {
	case "find":
		return base + "view"
	default:
		return base + action
	}
}

// grantActionForPermission is entityActionPermission in reverse: it turns the
// RESOURCE action a permission asks about ("view") back into the GRANT word a
// `public_entities` entry is written with ("find").
//
// The two vocabularies are the same pair the router uses (StandardRESTActions
// maps the `find` action to the `view` permission), and the allowlist is
// authored in grant words. Comparing them without translating made a grant of
// `[find]` match nothing.
func grantActionForPermission(action string) string {
	switch action {
	case "view":
		return "find"
	default:
		return action
	}
}

// authorizedActions returns the entity actions THIS caller may perform — the
// same question the renderer asks before registering a derived CRUD route or
// rendering an action button.
//
// The set mirrors the routes the router actually registers for the UI surface
// (internal/api/generator.go GenerateUIRoutes): standard CRUD, then the subset
// of lifecycle actions the entity really has, plus soft-deactivate pairs. A
// `characteristic: summary` entity is a system-managed projection, so its write
// actions are excluded exactly as the router excludes their routes.
//
// Custom actions are not listed: the renderer already takes their permission
// from `ActionSummary.Permission`, which the bundle carries per action.
func authorizedActions(d EntityDescriptor, schema EntitySchema, can PermissionChecker) []string {
	es := d.Spec
	if es == nil {
		return nil
	}
	isSummary := es.Characteristic == spec.CharSummary
	disabled := map[string]bool{}
	for _, a := range es.Actions {
		if a.Disabled {
			disabled[a.Name] = true
		}
	}
	// Same rule the router applies (internal/api/generator.go disabledActions):
	// a lifecycle-free entity has no draft→submit workflow, and disabling
	// `submit` transitively disables cancel/amend.
	if es.LifecycleFree() {
		disabled["submit"] = true
	}
	if disabled["submit"] {
		disabled["cancel"] = true
		disabled["amend"] = true
	}
	if disabled["cancel"] {
		disabled["amend"] = true
	}

	var out []string
	add := func(action string) {
		if disabled[action] {
			return
		}
		if !can(entityActionPermission(d.Module, schema.Plural, action)) {
			return
		}
		out = append(out, action)
	}

	for _, action := range []string{"list", "find", "create", "update", "delete"} {
		if isSummary && action != "list" && action != "find" {
			continue // summary entities take no CUD (router registers none)
		}
		add(action)
	}
	if !isSummary {
		for _, action := range []string{"submit", "cancel", "amend"} {
			add(action)
		}
	}
	if es.SoftDeactivate != nil && es.SoftDeactivate.Enabled {
		add("deactivate")
		add("reactivate")
	}
	return out
}

// lifecycle derives the UI pattern from the reserved `submit` action
// (Frontend §1.7):
//   - explicit lifecycle in YAML → use that value directly
//   - submit exists and not disabled → two_step_manual (Save Draft + Submit)
//   - submit disabled or absent    → plain_crud
func lifecycle(es *spec.EntitySpec) string {
	if es.Lifecycle != "" {
		return es.Lifecycle
	}
	for _, a := range es.Actions {
		if a.Name == "submit" {
			if a.Disabled {
				return "plain_crud"
			}
			return "two_step_manual"
		}
	}
	return "plain_crud"
}

// labelField picks the field used to represent a record in pickers, links,
// and kanban cards: natural key → "name" → "title" → "number" → id.
func labelField(es *spec.EntitySpec) string {
	// 1. Explicit display_field in manifest
	if es.DisplayField != "" {
		return es.DisplayField
	}
	// 2. Natural key field
	for _, f := range es.Fields {
		if f.NaturalKey {
			return f.Name
		}
	}
	// 3. Convention: name / title / number
	for _, candidate := range []string{"name", "title", "number"} {
		for _, f := range es.Fields {
			if f.Name == candidate {
				return f.Name
			}
		}
	}
	// 4. Fallback to id
	return "id"
}
