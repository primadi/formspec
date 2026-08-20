package auth

import (
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/entity"
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
		page, ok := m.uiReg.Pages[g.Page]
		if !ok {
			return nil, fmt.Errorf("materialize: unknown page %q", g.Page)
		}
		footprint, err := m.pageFootprint(page)
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
func (m *Materializer) blockFootprint(module, tab string, form, table, component *spec.BlockRef) ([]FootprintAction, error) {
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
		for _, action := range []string{"list", "view"} {
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
