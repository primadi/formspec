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
	// Module and Entity name the entity the permission resolves to, so a caller
	// holding a grant can validate the grant's row scope against THAT entity's
	// fields (a row scope names a column of the row it restricts).
	Module string
	Entity string
}

// Materializer expands a role's page/tab/action grants into concrete
// `{module}.{entity}.{action}` permission strings (todo 5.12.5). It uses the
// page's footprint (derived from its blocks/tabs) to know which entity each
// action maps to. The page/tab structure is admin UX only — enforcement stays
// on the materialized permission strings.
type Materializer struct {
	uiReg *ui.Registry
	reg   *entity.Registry
	// approvalDuties resolves an approval gate by name to its grantable duties,
	// so a grant can name an approval DUTY (`{ page: "workflow:{name}", actions:
	// [{name: "{duty}"}] }`). It is a function rather than a registry so this
	// package keeps no dependency on the workflow engine — the same shape as
	// HandlerFactory.grantScopeLookup.
	//
	// nil = this deployment has no approval registry, and a `workflow:` grant is
	// then reported as an unknown page rather than silently ignored.
	approvalDuties func(name string) (module string, duties []spec.DutyRef, ok bool)
}

// SetApprovalDuties wires the workflow lookup used to materialize approval-duty
// grants (plan docs_internal/plan/approval-duty-permission.md, Fase 4).
//
// Without it, `{ page: "workflow:order-void-approval", actions: [{name:
// supervisor-check}] }` resolves to nothing: the grant looks configured, the
// role keeps its other permissions, and the approval duty is simply absent —
// measured on kafe, where the supervisor's 44 permissions contained no
// `workflow.*` at all. That is the same "looks right, enforces nothing" failure
// this file already documents for page typos.
func (m *Materializer) SetApprovalDuties(fn func(name string) (string, []spec.DutyRef, bool)) {
	m.approvalDuties = fn
}

// NewMaterializer creates a Materializer backed by the UI and entity registries.
func NewMaterializer(uiReg *ui.Registry, reg *entity.Registry) *Materializer {
	return &Materializer{uiReg: uiReg, reg: reg}
}

// GrantProblem records one grant that contributed nothing, and why. It exists
// so a partial materialization can be REPORTED instead of silently dropped —
// a grant that resolves to zero permissions is almost always a name typo, and
// before this the only trace of it was an unexplained missing button.
type GrantProblem struct {
	// Page is the grant's page reference, exactly as the role declared it.
	Page string
	// Reason is a human-readable explanation (e.g. "unknown page", or the
	// actions that matched nothing in the page's footprint).
	Reason string
	// HadRowScope marks a problem on a grant that DECLARED a row restriction.
	//
	// The two classes need opposite handling and must not be collapsed: a grant
	// whose page/action does not resolve costs the caller a permission (fail
	// closed on its own), while a row restriction that cannot be applied leaves
	// the permission intact and the boundary GONE — fail-open, silently. So the
	// row-scope class makes the runtime deny the permission outright instead of
	// logging and continuing (see PermissionResolver.GrantScope).
	HadRowScope bool
}

func (p GrantProblem) String() string { return fmt.Sprintf("%s: %s", p.Page, p.Reason) }

// MaterializedPermission is one concrete capability a role grants, together
// with the row scope declared on the action grant that produced it.
//
// The permission string alone answers "may this caller do X?"; RowScope answers
// the second half — "to which rows?". Keeping them together is what lets a role
// hold `list` on orders and still be confined to the paid ones (kafe 10.67).
type MaterializedPermission struct {
	// Page is the grant's page reference, as the role declared it. Carried for
	// diagnostics: "which grant is this restriction from?".
	Page string
	// Action is the admin-facing action name.
	Action string
	// Permission is the concrete `{module}.{entity}.{action}` string.
	Permission string
	// RowScope restricts the rows this permission applies to. Empty = no
	// restriction beyond the permission itself.
	RowScope []spec.FilterSpec
}

// MaterializeDetailed expands a role's grants into concrete permissions PLUS
// the row scope attached to each granted action.
//
// Resolution is per grant (see MaterializePartial for why), so one unresolvable
// page cannot disarm the whole role.
func (m *Materializer) MaterializeDetailed(grants []Grant) ([]MaterializedPermission, []GrantProblem) {
	var out []MaterializedPermission
	var problems []GrantProblem

	// record adds one matched grant action — unless its row scope cannot be
	// applied, in which case the permission is NOT granted (deny) and the reason
	// is reported as a row-scope problem so the runtime denies instead of
	// quietly reading unscoped.
	record := func(fa FootprintAction, ag ActionGrant, page string) {
		if len(ag.RowScope) > 0 {
			if probs := m.grantRowScopeProblems(fa, page, ag.Name, ag.RowScope); len(probs) > 0 {
				problems = append(problems, probs...)
				return
			}
		}
		out = append(out, MaterializedPermission{
			Page: page, Action: ag.Name,
			Permission: fa.Permission, RowScope: ag.RowScope,
		})
	}

	for _, g := range grants {
		footprint, err := m.resolveFootprint(g.Page)
		if err != nil {
			problems = append(problems, GrantProblem{Page: g.Page, Reason: err.Error()})
			continue
		}

		matched := 0

		// Tabbed page: match granted tabs against footprint tabs.
		if len(g.Tabs) > 0 {
			for _, tabGrant := range g.Tabs {
				for _, fa := range footprint {
					if fa.Tab != tabGrant.Tab {
						continue
					}
					for _, ag := range tabGrant.Actions {
						if fa.Action == ag.Name {
							matched++
							record(fa, ag, g.Page)
						}
					}
				}
			}
		} else {
			// Block page: match granted actions against footprint (no tab).
			for _, ag := range g.Actions {
				for _, fa := range footprint {
					if fa.Tab == "" && fa.Action == ag.Name {
						matched++
						record(fa, ag, g.Page)
					}
				}
			}
		}

		// A page that resolves but grants nothing is the 10.47 class: the page
		// is real, the action name is not one it exposes. Reported for the same
		// reason — otherwise the grant silently disappears.
		//
		// Matched actions that were BLOCKED by an unusable row scope are counted
		// as matched: their problem is already reported, and adding "the action
		// name does not exist" on top would point the operator at the wrong
		// thing.
		if matched == 0 {
			problems = append(problems, GrantProblem{
				Page:   g.Page,
				Reason: "no granted action matches this page's footprint (check the action names)",
			})
		}
	}

	return out, problems
}

// grantRowScopeProblems validates one action grant's row scope against the
// entity the granted action resolves to.
//
// Three defects are possible and all three used to be silent: a field the entity
// does not have, an operator the query builder cannot express (which produces no
// clause — a filter that protects nothing), and a row_scope entry with no usable
// value source. Validation runs at MATERIALIZATION time because that is the
// first point where the target entity is known, and the same call serves the
// runtime (resolver) and `formspec check` — one rule, two consumers.
func (m *Materializer) grantRowScopeProblems(fa FootprintAction, page, action string, scope []spec.FilterSpec) []GrantProblem {
	origin := fmt.Sprintf("grant row_scope on %s/%s", page, action)
	var fields map[string]bool
	if info, ok := m.reg.GetEntity(fa.Module, fa.Entity); ok && info.EntitySpec != nil {
		fields = make(map[string]bool, len(info.EntitySpec.Fields))
		for _, f := range info.EntitySpec.Fields {
			fields[f.Name] = true
		}
	}
	var exists func(string) bool
	if fields != nil {
		exists = func(name string) bool { return fields[name] || spec.IsReservedField(name) }
	}
	if err := spec.ValidateRowScopeFilters(origin, scope, exists); err != nil {
		return []GrantProblem{{Page: page, Reason: err.Error(), HadRowScope: true}}
	}
	return nil
}

// MaterializePartial expands a role's grants into permissions, resolving each
// grant INDEPENDENTLY: a grant that cannot be resolved is skipped and reported
// rather than voiding the whole role.
//
// Why this exists (kafe 10.53): resolveFootprint returns an error for a page it
// does not recognize, and the strict Materialize propagates it — so ONE bad
// page name in a role's grants removed EVERY permission that role granted. The
// symptom was not a missing button but "this role appears to have no
// permissions at all", with the cause buried in one grant the operator had to
// find by eye. The failure was fail-closed (no privilege leaked), but a typo in
// one page reference should not be able to disarm a whole role.
//
// `app` is accepted for symmetry with the resolver and is not used for
// filtering here — App scoping happens in the resolver (role.App), before this
// point.
func (m *Materializer) MaterializePartial(grants []Grant) ([]string, []GrantProblem) {
	detailed, problems := m.MaterializeDetailed(grants)
	seen := map[string]bool{}
	out := []string{}
	for _, d := range detailed {
		if d.Permission != "" && !seen[d.Permission] {
			seen[d.Permission] = true
			out = append(out, d.Permission)
		}
	}
	return out, problems
}

// Materialize expands a role's grants into a deduplicated set of permission
// strings. It returns an error if a grant references an unknown page/tab/action.
//
// Strict by design: validation and tests want "this role's grants are all
// resolvable" as a single yes/no. The RESOLVER uses MaterializePartial instead,
// so a runtime typo degrades one grant rather than the entire role — see the
// comment there.
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

	// Read the UNION (declared `actions:` ∪ transition `via`) ONCE and use it for
	// both the disabled set and the custom-action loop below. A transition `via`
	// IS an action since L3, so `es.Actions` alone answered a different question
	// than the one this footprint asks (kafe 10.60, plan
	// via-sebagai-action-penuh.md L9). The union read is behaviour-neutral for
	// `disabled` — a synthesised action is never disabled — but keeps this the
	// last direct `es.Actions` reader removed from the file.
	sources := es.ActionSources()

	disabled := map[string]bool{}
	for _, a := range sources {
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
		out = append(out, FootprintAction{Action: action, Permission: module + "." + plural + "." + action, Module: module, Entity: entityName})
	}

	for _, action := range []string{"list", "view", "create", "update", "delete", "submit", "cancel", "amend"} {
		add(action)
	}
	if es.SoftDeactivate != nil && es.SoftDeactivate.Enabled {
		add("deactivate")
		add("reactivate")
	}

	// Custom actions (non-reserved) with their own permission strings.
	//
	// Read the UNION (declared actions ∪ transition `via`), not `es.Actions`
	// alone (plan docs_internal/plan/via-sebagai-action-penuh.md, L5). A grant
	// names an ACTION, and since L3 a transition's `via` IS an action; once the
	// duplicated `actions:` entry is removed (L4), reading only `es.Actions`
	// makes the grant EDITOR stop offering the transition — measured: dining-table
	// kept `reserve`/`release` in `authorized_actions` only while the duplicate
	// `actions:` block existed, and lost them the moment it was removed.
	for _, a := range sources {
		if a.Disabled || spec.IsReservedAction(a.Name) {
			continue
		}
		perm := a.RequiredPermission
		if perm == "" {
			perm = module + "." + plural + "." + a.Name
		} else {
			perm = spec.QualifyPermission(perm, module)
		}
		out = append(out, FootprintAction{Action: a.Name, Permission: perm, Module: module, Entity: entityName})
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
			mod, ent, _, terr := m.entityTarget(d.Module, widget.Spec.Entity)
			if terr != nil {
				continue
			}
			perm, err := m.entityPerm(d.Module, widget.Spec.Entity, "view")
			if err != nil {
				continue
			}
			out = append(out, FootprintAction{Action: "view", Permission: perm, Module: mod, Entity: ent})
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
			return []FootprintAction{{Action: "view", Permission: spec.QualifyPermission(r.Spec.RequiredPermission, r.Module)}}, nil
		}
		mod, ent, _, terr := m.entityTarget(r.Module, r.Spec.Entity)
		if terr != nil {
			return nil, terr
		}
		perm, err := m.entityPerm(r.Module, r.Spec.Entity, "list")
		if err != nil {
			return nil, err
		}
		return []FootprintAction{{Action: "view", Permission: perm, Module: mod, Entity: ent}}, nil

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

	case "workflow":
		// One footprint entry per grantable DUTY, keyed by its NAME — so
		// `{ page: "workflow:order.void-order", actions: [{name:
		// "supervisor-check"}] }` grants exactly that step's approval right, and
		// `{ name: "manager-check" }` grants the escalation takeover.
		//
		// Each permission is DERIVED from the gate's own declaration
		// (`spec.ApprovalDuties`), never transcribed: a hand-written duty string
		// in a grant would be a second copy of a fact the gate already owns, free
		// to drift from it.
		if m.approvalDuties == nil {
			return nil, fmt.Errorf("materialize: approval gate %q cannot be granted — no approval registry is wired", name)
		}
		module, duties, ok := m.approvalDuties(name)
		if !ok {
			return nil, fmt.Errorf("materialize: unknown approval gate %q", name)
		}
		out := make([]FootprintAction, 0, len(duties))
		for _, d := range duties {
			out = append(out, FootprintAction{Action: d.Name, Permission: d.Permission, Module: module})
		}
		if len(out) == 0 {
			// Every step of this gate lets a ROLE approve instead of a duty and
			// declares no escalation. Granting it would materialize to nothing,
			// which is precisely the silent no-op this branch exists to prevent.
			return nil, fmt.Errorf("materialize: approval gate %q declares no grantable duty — nothing to grant", name)
		}
		return out, nil

	default:
		return nil, fmt.Errorf("materialize: unknown navigation kind %q", kind)
	}
}

// entityViewFootprint returns a single "view" footprint for an entity-backed kind.
func (m *Materializer) entityViewFootprint(module, entityRef string) ([]FootprintAction, error) {
	if entityRef == "" {
		return nil, fmt.Errorf("materialize: kind has no entity")
	}
	mod, name, _, err := m.entityTarget(module, entityRef)
	if err != nil {
		return nil, err
	}
	perm, err := m.entityPerm(module, entityRef, "view")
	if err != nil {
		return nil, err
	}
	return []FootprintAction{{Action: "view", Permission: perm, Module: mod, Entity: name}}, nil
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
		fMod, fEnt, _, terr := m.entityTarget(module, f.Spec.Entity)
		if terr != nil {
			return nil, terr
		}
		perm, err := m.entityPerm(module, f.Spec.Entity, ui.FormActionPerm(f.Spec.Mode))
		if err != nil {
			return nil, err
		}
		out = append(out, FootprintAction{Tab: tab, Action: ui.FormActionPerm(f.Spec.Mode), Permission: perm, Module: fMod, Entity: fEnt})
		// Custom form actions.
		for _, a := range f.Spec.Actions {
			p, err := m.entityPerm(module, f.Spec.Entity, a.Action)
			if err != nil {
				return nil, err
			}
			out = append(out, FootprintAction{Tab: tab, Action: a.Action, Permission: p, Module: fMod, Entity: fEnt})
		}
	}

	if table != nil && table.Ref != "" {
		t, ok := m.uiReg.Tables[table.Ref]
		if !ok {
			return nil, fmt.Errorf("unknown table %q", table.Ref)
		}
		tMod, tEnt, _, terr := m.entityTarget(module, t.Spec.Entity)
		if terr != nil {
			return nil, terr
		}
		for _, action := range []string{"list", "view", "create", "update", "delete"} {
			p, err := m.entityPerm(module, t.Spec.Entity, action)
			if err != nil {
				return nil, err
			}
			out = append(out, FootprintAction{Tab: tab, Action: action, Permission: p, Module: tMod, Entity: tEnt})
		}
		// Row + bulk actions.
		for _, a := range append(append([]spec.TableAction{}, t.Spec.RowActions...), t.Spec.BulkActions...) {
			p, err := m.entityPerm(module, t.Spec.Entity, a.Action)
			if err != nil {
				return nil, err
			}
			out = append(out, FootprintAction{Tab: tab, Action: a.Action, Permission: p, Module: tMod, Entity: tEnt})
		}
	}

	// Component blocks declare their needs explicitly (todo 5.9.6) — not
	// derivable here; skipped for now.

	return out, nil
}

// entityPerm builds the {module}.{entity}.{action} permission string,
// resolving the entity's plural from the registry.
func (m *Materializer) entityPerm(module, entityRef, action string) (string, error) {
	mod, name, es, err := m.entityTarget(module, entityRef)
	if err != nil {
		return "", err
	}
	plural := es.Plural
	if plural == "" {
		plural = name + "s"
	}
	return mod + "." + plural + "." + action, nil
}

// entityTarget resolves an entity reference (bare name, or "module.entity") to
// the owning module, the entity name, and its spec.
//
// It is the ONE place that turns a reference into a target: the permission the
// materializer emits and the entity a grant's row scope is validated against are
// derived from the same resolution, so a grant can never be validated against a
// different entity than the one it grants access to.
func (m *Materializer) entityTarget(module, entityRef string) (string, string, *spec.EntitySpec, error) {
	mod, name := module, entityRef
	if i := strings.IndexByte(entityRef, '.'); i > 0 {
		mod, name = entityRef[:i], entityRef[i+1:]
	}
	info, ok := m.reg.GetEntity(mod, name)
	if !ok || info.EntitySpec == nil {
		return "", "", nil, fmt.Errorf("unknown entity %q", mod+"/"+name)
	}
	return mod, name, info.EntitySpec, nil
}
