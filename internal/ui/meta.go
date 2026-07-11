package ui

import (
	"strings"

	"github.com/primadi/forma/pkg/spec"
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

// Bundle is the full /_meta/ui payload.
type Bundle struct {
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
	Menus      []*Entry[spec.MenuSpec]      `json:"menus"`
	Prints     []*Entry[spec.PrintSpec]     `json:"prints"`
	Themes     []*Entry[spec.ThemeSpec]     `json:"themes"`
}

// PermissionChecker reports whether the caller holds a permission
// (wildcards included). Implemented by auth.Identity.HasPermission.
type PermissionChecker func(permission string) bool

// EntityLister enumerates registered entities. Implemented by the entity
// registry (adapter in resource/forma.go).
type EntityLister func() []EntityDescriptor

// EntityDescriptor pairs an entity spec with its metadata for bundle building.
type EntityDescriptor struct {
	Module      string
	Name        string
	Description string
	Spec        *spec.EntitySpec
}

// BuildBundle assembles the /_meta/ui payload for one caller.
//
// Permission filtering (Frontend §1.4, defense in depth — the renderer
// re-filters per element): an entity schema ships when the caller can list
// or view it; entity-backed manifests follow their entity; pages with
// explicit permissions require at least one; navigation-only kinds
// (Menu, Dashboard, Theme, Wizard) always ship — their leaf elements are
// permission-gated client-side against /_meta/me.
func (r *Registry) BuildBundle(entities EntityLister, can PermissionChecker) *Bundle {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b := &Bundle{}

	visible := map[string]bool{} // "module/name" → caller can see entity
	for _, d := range entities() {
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
		if allowedPage(e, can) {
			b.Pages = append(b.Pages, e)
		}
	}
	for _, k := range sortedKeys(r.Forms) {
		if e := r.Forms[k]; entityVisible(e.Module, e.Spec.Entity) {
			b.Forms = append(b.Forms, e)
		}
	}
	for _, k := range sortedKeys(r.Tables) {
		if e := r.Tables[k]; entityVisible(e.Module, e.Spec.Entity) {
			b.Tables = append(b.Tables, e)
		}
	}
	for _, k := range sortedKeys(r.Widgets) {
		e := r.Widgets[k]
		if e.Spec.Entity == "" || entityVisible(e.Module, e.Spec.Entity) {
			b.Widgets = append(b.Widgets, e)
		}
	}
	for _, k := range sortedKeys(r.Reports) {
		e := r.Reports[k]
		if e.Spec.RequiredPermission != "" && !can(qualifyPerm(e.Module, e.Spec.RequiredPermission)) {
			continue
		}
		if entityVisible(e.Module, e.Spec.Entity) {
			b.Reports = append(b.Reports, e)
		}
	}
	for _, k := range sortedKeys(r.Kanbans) {
		if e := r.Kanbans[k]; entityVisible(e.Module, e.Spec.Entity) {
			b.Kanbans = append(b.Kanbans, e)
		}
	}
	for _, k := range sortedKeys(r.Timelines) {
		if e := r.Timelines[k]; entityVisible(e.Module, e.Spec.Entity) {
			b.Timelines = append(b.Timelines, e)
		}
	}
	for _, k := range sortedKeys(r.Prints) {
		if e := r.Prints[k]; entityVisible(e.Module, e.Spec.Entity) {
			b.Prints = append(b.Prints, e)
		}
	}
	for _, k := range sortedKeys(r.Dashboards) {
		b.Dashboards = append(b.Dashboards, r.Dashboards[k])
	}
	for _, k := range sortedKeys(r.Wizards) {
		b.Wizards = append(b.Wizards, r.Wizards[k])
	}
	for _, k := range sortedKeys(r.Menus) {
		b.Menus = append(b.Menus, r.Menus[k])
	}
	for _, k := range sortedKeys(r.Themes) {
		b.Themes = append(b.Themes, r.Themes[k])
	}

	return b
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
// (Frontend §1.7): explicitly disabled → plain CRUD; otherwise the default
// draft→submit lifecycle with silent auto-save.
func lifecycle(es *spec.EntitySpec) string {
	for _, a := range es.Actions {
		if a.Name == "submit" && a.Disabled {
			return "plain_crud"
		}
	}
	return "two_step_autosave"
}

// labelField picks the field used to represent a record in pickers, links,
// and kanban cards: natural key → "name" → "title" → "number" → id.
func labelField(es *spec.EntitySpec) string {
	for _, f := range es.Fields {
		if f.NaturalKey {
			return f.Name
		}
	}
	for _, candidate := range []string{"name", "title", "number"} {
		for _, f := range es.Fields {
			if f.Name == candidate {
				return f.Name
			}
		}
	}
	return "id"
}
