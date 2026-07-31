package ui

import (
	"fmt"

	"strings"

	"github.com/primadi/forma/pkg/spec"
)

// normativeColumns are framework-managed columns present on every document
// table (Core §19) and therefore always valid as field references.
var normativeColumns = map[string]bool{
	"id": true, "version": true, "created_at": true, "updated_at": true,
	"created_by": true, "updated_by": true, "doc_status": true,
}

// builtinRowActions are renderer-provided actions that need no backing
// entity action (navigation / client-side operations).
var builtinRowActions = map[string]bool{
	"view": true, "edit": true, "delete": true, "export": true, "print": true,
}

// Validate cross-checks every registered UI manifest against the entity
// registry and sibling manifests (design doc §4.1):
//   - entity refs resolve
//   - form fields / table columns exist on the entity
//   - row/form actions exist (or are renderer builtins)
//   - page routes are unique; blocks/tabs refs resolve; blocks XOR tabs
//   - kanban columns ⊆ status field enum values
//   - dashboard/wizard refs resolve
//
// Errors are collected best-effort so `forma validate` can report all at once.
func (r *Registry) Validate(resolve EntityResolver) []error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []error
	addf := func(format string, a ...any) {
		errs = append(errs, fmt.Errorf(format, a...))
	}

	// ── Forms ──
	for _, name := range sortedKeys(r.Forms) {
		e := r.Forms[name]
		es, _, _, ok := resolveEntityRef(resolve, e.Module, e.Spec.Entity)
		if !ok {
			addf("%s: Form %q: entity %q not found", e.Source, name, e.Spec.Entity)
			continue
		}
		for _, sec := range e.Spec.Sections {
			for _, f := range sec.Fields {
				if !fieldPathExists(resolve, e.Module, es, f.Field) {
					addf("%s: Form %q: field %q not on entity %q", e.Source, name, f.Field, e.Spec.Entity)
				}
			}
		}
		for _, a := range e.Spec.Actions {
			if !actionExists(es, a.Action) {
				addf("%s: Form %q: action %q not on entity %q", e.Source, name, a.Action, e.Spec.Entity)
			}
		}
		var renderMode spec.FormRender
		if e.Spec.Render != nil {
			renderMode = e.Spec.Render.Mode
		}
		switch renderMode {
		case "", "modal", "drawer", "separate_page":
		default:
			addf("%s: Form %q: invalid render %q (modal|drawer|separate_page)", e.Source, name, string(renderMode))
		}
	}

	// ── Tables ──
	for _, name := range sortedKeys(r.Tables) {
		e := r.Tables[name]
		es, _, _, ok := resolveEntityRef(resolve, e.Module, e.Spec.Entity)
		if !ok {
			addf("%s: Table %q: entity %q not found", e.Source, name, e.Spec.Entity)
			continue
		}
		for _, c := range e.Spec.Columns {
			if !fieldPathExists(resolve, e.Module, es, c.Field) {
				addf("%s: Table %q: column %q not on entity %q", e.Source, name, c.Field, e.Spec.Entity)
			}
		}
		for _, f := range e.Spec.Filters {
			if !fieldPathExists(resolve, e.Module, es, f.Field) {
				addf("%s: Table %q: filter field %q not on entity %q", e.Source, name, f.Field, e.Spec.Entity)
			}
		}
		for _, a := range append(append([]spec.TableAction{}, e.Spec.RowActions...), e.Spec.BulkActions...) {
			if !builtinRowActions[a.Action] && !actionExists(es, a.Action) {
				addf("%s: Table %q: action %q not on entity %q and not a builtin (view|edit|delete|export|print)",
					e.Source, name, a.Action, e.Spec.Entity)
			}
		}
		if s := strings.TrimPrefix(e.Spec.DefaultSort, "-"); s != "" {
			if !fieldPathExists(resolve, e.Module, es, s) {
				addf("%s: Table %q: default_sort field %q not on entity %q", e.Source, name, s, e.Spec.Entity)
			}
		}
	}

	// ── Pages ──
	routes := map[string]string{} // route → page name
	for _, name := range sortedKeys(r.Pages) {
		e := r.Pages[name]
		if e.Spec.Route == "" {
			addf("%s: Page %q: route is required", e.Source, name)
		} else if other, dup := routes[e.Spec.Route]; dup {
			addf("%s: Page %q: duplicate route %q (also on page %q)", e.Source, name, e.Spec.Route, other)
		} else {
			routes[e.Spec.Route] = name
		}
		if len(e.Spec.Blocks) > 0 && len(e.Spec.Tabs) > 0 {
			addf("%s: Page %q: blocks and tabs are mutually exclusive", e.Source, name)
		}
		for i, b := range e.Spec.Blocks {
			r.validateBlockRef(addf, e.Source, name, fmt.Sprintf("blocks[%d]", i), b.Form, b.Table, b.Widget, b.Component)
		}
		for i, t := range e.Spec.Tabs {
			if t.Label == "" {
				addf("%s: Page %q: tabs[%d]: label is required", e.Source, name, i)
			}
			r.validateBlockRef(addf, e.Source, name, fmt.Sprintf("tabs[%d]", i), t.Form, t.Table, nil, t.Component)
		}
	}

	// ── Dashboards ──
	for _, name := range sortedKeys(r.Dashboards) {
		e := r.Dashboards[name]
		for _, w := range e.Spec.Widgets {
			if _, ok := r.Widgets[w.Ref]; !ok {
				addf("%s: Dashboard %q: widget ref %q not found", e.Source, name, w.Ref)
			}
		}
		for _, d := range e.Spec.Defaults {
			if _, ok := r.Widgets[d]; !ok {
				addf("%s: Dashboard %q: defaults widget %q not found", e.Source, name, d)
			}
		}
	}

	// ── Widgets / Reports / Prints / Timelines: entity refs ──
	for _, name := range sortedKeys(r.Widgets) {
		e := r.Widgets[name]
		if e.Spec.Entity != "" {
			if _, _, _, ok := resolveEntityRef(resolve, e.Module, e.Spec.Entity); !ok {
				addf("%s: Widget %q: entity %q not found", e.Source, name, e.Spec.Entity)
			}
		}
	}
	for _, name := range sortedKeys(r.Reports) {
		e := r.Reports[name]
		es, _, _, ok := resolveEntityRef(resolve, e.Module, e.Spec.Entity)
		if !ok {
			addf("%s: Report %q: entity %q not found", e.Source, name, e.Spec.Entity)
			continue
		}
		for _, c := range e.Spec.Columns {
			if !fieldPathExists(resolve, e.Module, es, c.Field) {
				addf("%s: Report %q: column %q not on entity %q", e.Source, name, c.Field, e.Spec.Entity)
			}
		}
	}
	for _, name := range sortedKeys(r.Prints) {
		e := r.Prints[name]
		if _, _, _, ok := resolveEntityRef(resolve, e.Module, e.Spec.Entity); !ok {
			addf("%s: Print %q: entity %q not found", e.Source, name, e.Spec.Entity)
		}
	}
	for _, name := range sortedKeys(r.Timelines) {
		e := r.Timelines[name]
		es, _, _, ok := resolveEntityRef(resolve, e.Module, e.Spec.Entity)
		if !ok {
			addf("%s: Timeline %q: entity %q not found", e.Source, name, e.Spec.Entity)
			continue
		}
		if e.Spec.DateField != "" && !fieldPathExists(resolve, e.Module, es, e.Spec.DateField) {
			addf("%s: Timeline %q: date_field %q not on entity %q", e.Source, name, e.Spec.DateField, e.Spec.Entity)
		}
		if e.Spec.BindParam != "" && !fieldPathExists(resolve, e.Module, es, e.Spec.BindParam) {
			addf("%s: Timeline %q: bind_param %q not on entity %q", e.Source, name, e.Spec.BindParam, e.Spec.Entity)
		}
	}

	// ── Kanbans ──
	for _, name := range sortedKeys(r.Kanbans) {
		e := r.Kanbans[name]
		es, _, _, ok := resolveEntityRef(resolve, e.Module, e.Spec.Entity)
		if !ok {
			addf("%s: Kanban %q: entity %q not found", e.Source, name, e.Spec.Entity)
			continue
		}
		var statusField *spec.Field
		for i, f := range es.Fields {
			if f.Name == e.Spec.StatusField {
				statusField = &es.Fields[i]
				break
			}
		}
		if statusField == nil {
			addf("%s: Kanban %q: status_field %q not on entity %q", e.Source, name, e.Spec.StatusField, e.Spec.Entity)
			continue
		}
		if len(statusField.EnumValues) > 0 {
			valid := map[string]bool{}
			for _, v := range statusField.EnumValues {
				valid[v] = true
			}
			for _, c := range e.Spec.Columns {
				if !valid[c.Status] {
					addf("%s: Kanban %q: column status %q not in enum_values of %q",
						e.Source, name, c.Status, e.Spec.StatusField)
				}
			}
		}
	}

	// ── Wizards ──
	for _, name := range sortedKeys(r.Wizards) {
		e := r.Wizards[name]
		var es *spec.EntitySpec
		if e.Spec.Entity != "" {
			var ok bool
			es, _, _, ok = resolveEntityRef(resolve, e.Module, e.Spec.Entity)
			if !ok {
				addf("%s: Wizard %q: entity %q not found", e.Source, name, e.Spec.Entity)
			}
		}
		for i, s := range e.Spec.Steps {
			if s.Form != "" {
				if _, ok := r.Forms[s.Form]; !ok {
					addf("%s: Wizard %q: steps[%d]: form %q not found", e.Source, name, i, s.Form)
				}
			}
			for _, hook := range []struct{ name, action string }{
				{"on_enter", s.OnEnter},
				{"on_next", s.OnNext},
				{"on_prev", s.OnPrev},
			} {
				if hook.action != "" && es != nil && !actionExists(es, hook.action) {
					addf("%s: Wizard %q: steps[%d]: %s %q not on entity %q", e.Source, name, i, hook.name, hook.action, e.Spec.Entity)
				}
			}
		}
	}

	return errs
}

// validateBlockRef checks one page block/tab's references against the registry.
func (r *Registry) validateBlockRef(addf func(string, ...any), source, page, where string, form, table, widget, component *spec.BlockRef) {
	if form != nil && form.Ref != "" {
		if _, ok := r.Forms[form.Ref]; !ok {
			addf("%s: Page %q: %s: form ref %q not found", source, page, where, form.Ref)
		}
	}
	if table != nil && table.Ref != "" {
		if _, ok := r.Tables[table.Ref]; !ok {
			addf("%s: Page %q: %s: table ref %q not found", source, page, where, table.Ref)
		}
	}
	if widget != nil && widget.Ref != "" {
		if _, ok := r.Widgets[widget.Ref]; !ok {
			addf("%s: Page %q: %s: widget ref %q not found", source, page, where, widget.Ref)
		}
	}
	// component refs point to asset files — existence is checked at deploy time.
	_ = component
}

// fieldPathExists reports whether a (possibly dotted) field path exists on the
// entity. Dot-paths traverse relations in two spellings (Frontend §5):
// the raw field name ("customer_id.name") or the relation alias — the field
// name minus its "_id" suffix, or the target resource name ("customer.name").
// Child fields resolve one level ("items.price"). When the relation target
// entity is not loaded, the tail is accepted leniently (deploy-time concern).
func fieldPathExists(resolve EntityResolver, module string, es *spec.EntitySpec, path string) bool {
	head, rest, hasRest := strings.Cut(path, ".")
	if normativeColumns[head] {
		return !hasRest
	}
	for i := range es.Fields {
		f := &es.Fields[i]
		if f.Name != head {
			continue
		}
		if !hasRest {
			return true
		}
		switch f.Type {
		case spec.FieldRelation:
			return relationTailExists(resolve, module, f, rest)
		case spec.FieldChild:
			if f.Child == nil {
				return false
			}
			for _, cf := range f.Child.Fields {
				if cf.Name == rest {
					return true
				}
			}
			return false
		default:
			return false
		}
	}

	// Relation alias: "customer.name" → relation field "customer_id" or a
	// relation whose target resource is named "customer".
	if hasRest {
		for i := range es.Fields {
			f := &es.Fields[i]
			if f.Type != spec.FieldRelation || f.Relation == nil {
				continue
			}
			targetName := f.Relation.Resource
			if j := strings.LastIndexByte(targetName, '.'); j >= 0 {
				targetName = targetName[j+1:]
			}
			if f.Name == head+"_id" || targetName == head {
				return relationTailExists(resolve, module, f, rest)
			}
		}
	}
	return false
}

// relationTailExists validates the remainder of a dot-path against a
// relation field's target entity.
func relationTailExists(resolve EntityResolver, module string, f *spec.Field, rest string) bool {
	if f.Relation == nil {
		return false
	}
	target, _, _, ok := resolveEntityRef(resolve, module, f.Relation.Resource)
	if !ok {
		return true // lenient: target entity not loaded (e.g. other app)
	}
	return fieldPathExists(resolve, module, target, rest)
}

// actionExists reports whether a non-disabled action with the given name is
// declared on the entity, or is one of the reserved actions that exist
// implicitly (Core §4.1: create, update, submit, cancel, delete, amend,
// create-submit, amend-submit — unless explicitly disabled).
func actionExists(es *spec.EntitySpec, name string) bool {
	for _, a := range es.Actions {
		if a.Name == name {
			return !a.Disabled
		}
	}
	return spec.IsReservedAction(name) // implicit reserved actions exist unless disabled
}
