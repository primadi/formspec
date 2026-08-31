package auth

import (
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/permission"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
)

// FootprintAction is one derived capability of a page: the admin-facing
// action name mapped to the concrete entity-action permission string.
type FootprintAction struct {
	// Tab is the tab label this action belongs to (empty for block pages).
	Tab string
	// Action is the admin-facing action name (create, list, submit, custom).
	Action string
	// Permission is the concrete {module}.{entity}.{action} string.
	Permission string
}

// Materializer expands a role's page/tab/action grants into concrete
// `{module}.{entity}.{action}` permission strings (todo 5.12.5). It uses the
// page's footprint (derived from its blocks/tabs) to know which entity each
// action maps to. The page/tab structure is admin UX only — enforcement stays
// on the materialized permission strings.
type Materializer struct {
	uiReg *ui.Registry
	reg   *entity.Registry
}

// NewMaterializer creates a Materializer backed by the UI and entity registries.
func NewMaterializer(uiReg *ui.Registry, reg *entity.Registry) *Materializer {
	return &Materializer{uiReg: uiReg, reg: reg}
}

// Materialize expands a role's grants into a deduplicated set of permission
// strings. It returns an error if a grant references an unknown page/tab/action.
func (m *Materializer) Materialize(grants []Grant) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	for _, g := range grants {
		footprint, err := m.resolveFootprint(g.Page)
		if err != nil {
			return nil, fmt.Errorf("materialize page %q: %w", g.Page, err)
		}

		// Tabbed page: match granted tabs against footprint tabs.
		if len(g.Tabs) > 0 {
			for _, tabGrant := range g.Tabs {
				for _, fa := range footprint {
					if fa.Tab != tabGrant.Tab {
						continue
					}
					for _, ag := range tabGrant.Actions {
						if fa.Action == ag.Name {
							add(fa.Permission)
						}
					}
				}
			}
			continue
		}

		// Block page: match granted actions against footprint (no tab).
		for _, ag := range g.Actions {
			for _, fa := range footprint {
				if fa.Tab == "" && fa.Action == ag.Name {
					add(fa.Permission)
				}
			}
		}
	}
	return out, nil
}

// resolveFootprint returns the footprint for a grant page reference, resolving
// three page kinds:
//   - authored page (registered in the UI registry) — page/tab/action footprint
//   - navigation kind ("{kind}:{name}", e.g. "dashboard:cafe-summary-dashboard")
//   - derived entity page ("{entity}-page", e.g. "order-page")
func (m *Materializer) resolveFootprint(pageRef string) ([]FootprintAction, error) {
	// 1. Authored page.
	if page, ok := m.uiReg.Pages[pageRef]; ok {
		return m.pageFootprint(page)
	}

	// 2. Navigation kind: "{kind}:{name}".
	if i := strings.IndexByte(pageRef, ':'); i > 0 {
		return m.navigationFootprint(pageRef[:i], pageRef[i+1:])
	}

	// 3. Derived entity page: "{entity}-page".
	if strings.HasSuffix(pageRef, "-page") {
		if entityName := strings.TrimSuffix(pageRef, "-page"); entityName != "" {
			if module, ok := m.findEntityModule(entityName); ok {
				return m.entityFootprint(module, entityName)
			}
		}
	}

	return nil, fmt.Errorf("materialize: unknown page %q", pageRef)
}

// findEntityModule resolves an entity name to its owning module. Returns false
// when the name is unknown or ambiguous (registered in more than one module).
func (m *Materializer) findEntityModule(name string) (string, bool) {
	var module string
	count := 0
	for _, e := range m.reg.ListEntities() {
		if e.Name == name {
			module = e.Module
			count++
		}
	}
	if count == 1 {
		return module, true
	}
	return "", false
}

// entityFootprint derives the grantable actions of an entity's CRUD surface:
// standard actions (list/view/create/update/delete + lifecycle) plus custom
// actions. Mirrors entity.registerStandardPermissions so the admin-facing
// grant tree and the materialized permission strings stay in sync.
func (m *Materializer) entityFootprint(module, entityName string) ([]FootprintAction, error) {
	info, ok := m.reg.GetEntity(module, entityName)
	if !ok || info.EntitySpec == nil {
		return nil, fmt.Errorf("unknown entity %q", module+"/"+entityName)
	}
	es := info.EntitySpec
	plural := es.Plural
	if plural == "" {
		plural = entityName + "s"
	}

	disabled := map[string]bool{}
	for _, a := range es.Actions {
		if a.Disabled {
			disabled[a.Name] = true
		}
	}
	isSummary := es.Characteristic == spec.CharSummary

	var out []FootprintAction
	add := func(action string) {
		if disabled[action] {
			return
		}
		if isSummary && (action == "create" || action == "update" || action == "delete") {
			return
		}
		out = append(out, FootprintAction{Action: action, Permission: module + "." + plural + "." + action})
	}

	for _, action := range []string{"list", "view", "create", "update", "delete", "submit", "cancel", "amend"} {
		add(action)
	}
	if es.SoftDeactivate != nil && es.SoftDeactivate.Enabled {
		add("deactivate")
		add("reactivate")
	}

	// Custom actions (non-reserved) with their own permission strings.
	for _, a := range es.Actions {
		if a.Disabled || spec.IsReservedAction(a.Name) {
			continue
		}
		perm := a.RequiredPermission
		if perm == "" {
			perm = module + "." + plural + "." + a.Name
		} else {
			perm = permission.AutoPrefixPermission(perm, module)
		}
		out = append(out, FootprintAction{Action: a.Name, Permission: perm})
	}
	return out, nil
}

// navigationFootprint derives the footprint for a navigation-only kind
// (Dashboard/Report/Wizard/Kanban/Timeline/Print). These kinds expose a single
// "view" action that materializes to the underlying entity/required permission.
func (m *Materializer) navigationFootprint(kind, name string) ([]FootprintAction, error) {
	switch kind {
	case "dashboard":
		d, ok := m.uiReg.Dashboards[name]
		if !ok {
			return nil, fmt.Errorf("materialize: unknown dashboard %q", name)
		}
		var out []FootprintAction
		for _, w := range d.Spec.Widgets {
			widget, ok := m.uiReg.Widgets[w.Ref]
			if !ok || widget.Spec.Entity == "" {
				continue
			}
			perm, err := m.entityPerm(d.Module, widget.Spec.Entity, "view")
			if err != nil {
				continue
			}
			out = append(out, FootprintAction{Action: "view", Permission: perm})
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("materialize: dashboard %q has no grantable widgets", name)
		}
		return out, nil

	case "report":
		r, ok := m.uiReg.Reports[name]
		if !ok {
			return nil, fmt.Errorf("materialize: unknown report %q", name)
		}
		if r.Spec.RequiredPermission != "" {
			return []FootprintAction{{Action: "view", Permission: permission.AutoPrefixPermission(r.Spec.RequiredPermission, r.Module)}}, nil
		}
		perm, err := m.entityPerm(r.Module, r.Spec.Entity, "list")
		if err != nil {
			return nil, err
		}
		return []FootprintAction{{Action: "view", Permission: perm}}, nil

	case "wizard":
		w, ok := m.uiReg.Wizards[name]
		if !ok {
			return nil, fmt.Errorf("materialize: unknown wizard %q", name)
		}
		return m.entityViewFootprint(w.Module, w.Spec.Entity)

	case "kanban":
		k, ok := m.uiReg.Kanbans[name]
		if !ok {
			return nil, fmt.Errorf("materialize: unknown kanban %q", name)
		}
		return m.entityViewFootprint(k.Module, k.Spec.Entity)

	case "timeline":
		t, ok := m.uiReg.Timelines[name]
		if !ok {
			return nil, fmt.Errorf("materialize: unknown timeline %q", name)
		}
		return m.entityViewFootprint(t.Module, t.Spec.Entity)

	case "print":
		p, ok := m.uiReg.Prints[name]
		if !ok {
			return nil, fmt.Errorf("materialize: unknown print %q", name)
		}
		return m.entityViewFootprint(p.Module, p.Spec.Entity)

	default:
		return nil, fmt.Errorf("materialize: unknown navigation kind %q", kind)
	}
}

// entityViewFootprint returns a single "view" footprint for an entity-backed kind.
func (m *Materializer) entityViewFootprint(module, entityRef string) ([]FootprintAction, error) {
	if entityRef == "" {
		return nil, fmt.Errorf("materialize: kind has no entity")
	}
	perm, err := m.entityPerm(module, entityRef, "view")
	if err != nil {
		return nil, err
	}
	return []FootprintAction{{Action: "view", Permission: perm}}, nil
}

// pageFootprint derives the set of (tab, action, permission) for a page by
// walking its blocks/tabs and resolving each Form/Table to its entity.
func (m *Materializer) pageFootprint(page *ui.Entry[spec.PageSpec]) ([]FootprintAction, error) {
	var out []FootprintAction

	// Tabbed page.
	for _, tab := range page.Spec.Tabs {
		fa, err := m.blockFootprint(page.Module, tab.Label, tab.Form, tab.Table, tab.Component)
		if err != nil {
			return nil, err
		}
		out = append(out, fa...)
	}

	// Block page (blocks and tabs are mutually exclusive).
	for _, blk := range page.Spec.Blocks {
		fa, err := m.blockFootprint(page.Module, "", blk.Form, blk.Table, blk.Component)
		if err != nil {
			return nil, err
		}
		out = append(out, fa...)
	}

	return out, nil
}

// blockFootprint derives actions for a single block (form/table/component).
func (m *Materializer) blockFootprint(module, tab string, form, table, _ *spec.BlockRef) ([]FootprintAction, error) {
	var out []FootprintAction

	if form != nil && form.Ref != "" {
		f, ok := m.uiReg.Forms[form.Ref]
		if !ok {
			return nil, fmt.Errorf("unknown form %q", form.Ref)
		}
		perm, err := m.entityPerm(module, f.Spec.Entity, ui.FormActionPerm(f.Spec.Mode))
		if err != nil {
			return nil, err
		}
		out = append(out, FootprintAction{Tab: tab, Action: ui.FormActionPerm(f.Spec.Mode), Permission: perm})
		// Custom form actions.
		for _, a := range f.Spec.Actions {
			p, err := m.entityPerm(module, f.Spec.Entity, a.Action)
			if err != nil {
				return nil, err
			}
			out = append(out, FootprintAction{Tab: tab, Action: a.Action, Permission: p})
		}
	}

	if table != nil && table.Ref != "" {
		t, ok := m.uiReg.Tables[table.Ref]
		if !ok {
			return nil, fmt.Errorf("unknown table %q", table.Ref)
		}
		for _, action := range []string{"list", "view", "create", "update", "delete"} {
			p, err := m.entityPerm(module, t.Spec.Entity, action)
			if err != nil {
				return nil, err
			}
			out = append(out, FootprintAction{Tab: tab, Action: action, Permission: p})
		}
		// Row + bulk actions.
		for _, a := range append(append([]spec.TableAction{}, t.Spec.RowActions...), t.Spec.BulkActions...) {
			p, err := m.entityPerm(module, t.Spec.Entity, a.Action)
			if err != nil {
				return nil, err
			}
			out = append(out, FootprintAction{Tab: tab, Action: a.Action, Permission: p})
		}
	}

	// Component blocks declare their needs explicitly (todo 5.9.6) — not
	// derivable here; skipped for now.

	return out, nil
}

// entityPerm builds the {module}.{entity}.{action} permission string,
// resolving the entity's plural from the registry.
func (m *Materializer) entityPerm(module, entityRef, action string) (string, error) {
	mod, name := module, entityRef
	if i := strings.IndexByte(entityRef, '.'); i > 0 {
		mod, name = entityRef[:i], entityRef[i+1:]
	}
	info, ok := m.reg.GetEntity(mod, name)
	if !ok || info.EntitySpec == nil {
		return "", fmt.Errorf("unknown entity %q", mod+"/"+name)
	}
	plural := info.EntitySpec.Plural
	if plural == "" {
		plural = name + "s"
	}
	return mod + "." + plural + "." + action, nil
}
