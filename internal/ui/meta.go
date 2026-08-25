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
	Name    string `json:"name"`
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
}

// AppContext scopes BuildBundle to one resolved App: which modules it mounts
// (Pages/Forms/Tables/... outside this set are excluded from the bundle) and
// its already-resolved menu tree (adopt nodes spliced, view leaves resolved
// to routes — see internal/app.Resolve). A zero-value AppContext (Modules
// nil) disables module filtering, for callers with no App concept yet.
type AppContext struct {
	Name           string
	RootURL        string
	AppRenderer    string
	Access         string
	StackFamily    string
	PersistBackend string
	Modules        map[string]bool
	Menu           []spec.MenuItem
	// Settings is the resolved global presentation/config namespace (spec §10).
	// Always non-nil — resolved with standard defaults by the caller.
	Settings *spec.Settings
}

// allows reports whether a manifest belonging to module may ship in this
// App's bundle. "core" is a repo-wide convention for App-level/cross-module
// content that isn't owned by any one declared Module (e.g. a cross-module
// Dashboard, or an app-level Config) — it always ships, since it was never
// meant to be gated by spec.modules in the first place.
func (c AppContext) allows(module string) bool {
	if c.Modules == nil || module == "core" {
		return true
	}
	return c.Modules[module]
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
	b := &Bundle{
		App: AppSummary{
			Name:           appCtx.Name,
			RootURL:        appCtx.RootURL,
			AppRenderer:    appCtx.AppRenderer,
			Access:         appCtx.Access,
			StackFamily:    appCtx.StackFamily,
			PersistBackend: appCtx.PersistBackend,
		},
		Menu:                menu,
		Pages:               []*Entry[spec.PageSpec]{},
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
		schema := buildEntitySchema(d)
		listPerm := d.Module + "." + schema.Plural + ".list"
		viewPerm := d.Module + "." + schema.Plural + ".view"
		if !can(listPerm) && !can(viewPerm) {
			continue
		}
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
		if appCtx.allows(e.Module) && allowedPage(e, can) {
			b.Pages = append(b.Pages, e)
		}
	}
	for _, k := range sortedKeys(r.Forms) {
		if e := r.Forms[k]; appCtx.allows(e.Module) && entityVisible(e.Module, e.Spec.Entity) {
			b.Forms = append(b.Forms, e)
		}
	}
	for _, k := range sortedKeys(r.Tables) {
		if e := r.Tables[k]; appCtx.allows(e.Module) && entityVisible(e.Module, e.Spec.Entity) {
			b.Tables = append(b.Tables, e)
		}
	}
	for _, k := range sortedKeys(r.Widgets) {
		e := r.Widgets[k]
		if !appCtx.allows(e.Module) {
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
	for _, k := range sortedKeys(r.Dashboards) {
		if e := r.Dashboards[k]; appCtx.allows(e.Module) {
			b.Dashboards = append(b.Dashboards, e)
		}
	}
	for _, k := range sortedKeys(r.Wizards) {
		if e := r.Wizards[k]; appCtx.allows(e.Module) {
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

	return b
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
