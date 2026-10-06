// ─── App surface (menu ∪ registered_views) ───
//
// The reachable surface of an App is "every menu leaf target ∪
// registered_views" (plan docs_internal/plan/registered-views.md). Anything
// outside it gets NO route.
//
// This file owns that computation ONCE. Two callers need it and must not drift:
//
//  1. `BuildBundle` (meta.go) uses it to decide which routes/entities the
//     surface offers at all — `Routable`, and the page/kind passes.
//  2. `DerivePublicGrants` uses the SAME set to derive what an anonymous
//     caller of an `access: public` App may fetch (plan
//     docs_internal/plan/implicit-public-grants.md). Before this, the
//     anonymous allowlist was written by hand in the manifest
//     (`App.spec.public_entities`) and could disagree with the surface the App
//     actually exposes.

package ui

import (
	"sort"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// appSurface is the set of routes and entities an App exposes, computed from
// its menu leaves ∪ registered_views.
type appSurface struct {
	// gate is whether the allowlist is active at all. An unknown App context
	// (zero AppContext — the `_admin` surface) or the `?grants=true` editor
	// disables it, and then every module is reachable as before.
	gate bool
	// owns reports whether a module is explicitly mounted by the App. Modules
	// the framework implies (formspec.core, core) are NOT gated.
	owns func(module string) bool
	// routes holds reachable view routes ("/t/:qr_token", "/app/pos/orders").
	routes map[string]bool
	// entities holds reachable entities as canonical "module/name".
	entities map[string]bool
}

// computeAppSurfaceLocked builds the App surface. The caller must hold r.mu
// (it calls resolveViewRouteLocked).
func (r *Registry) computeAppSurfaceLocked(
	ix *entityIndex,
	gate bool,
	owns func(module string) bool,
	menu []spec.MenuItem,
	regViews []spec.RegisteredViewDecl,
) appSurface {
	s := appSurface{
		gate:     gate,
		owns:     owns,
		routes:   map[string]bool{},
		entities: map[string]bool{},
	}
	if !gate {
		return s
	}

	var addMenu func(items []spec.MenuItem)
	addMenu = func(items []spec.MenuItem) {
		for _, it := range items {
			if len(it.Children) > 0 {
				addMenu(it.Children)
				continue
			}
			if it.Route == "" {
				continue
			}
			s.routes[it.Route] = true
			// A "/<module>/<plural>" route targets a derived entity page —
			// register the entity it displays.
			if parts := strings.Split(strings.Trim(it.Route, "/"), "/"); len(parts) == 2 {
				if ref, ok := ix.moduleOfPlural(parts[0], parts[1]); ok {
					s.entities[ref] = true
				}
			}
		}
	}
	addMenu(menu)

	for _, rv := range regViews {
		if rv.View != "" {
			mod, name, ok := strings.Cut(rv.View, "/")
			if !ok {
				continue
			}
			if route, err := r.resolveViewRouteLocked(mod, name); err == nil {
				s.routes[route] = true
			}
			continue
		}
		if canonical, ok := spec.NormalizeEntityRef(rv.Entity); ok {
			s.entities[canonical] = true
		}
	}
	return s
}

// entityRoutable reports whether the entity's derived routes exist. A module
// the App does not explicitly mount is never gated.
func (s appSurface) entityRoutable(module, name string) bool {
	if !s.gate || !s.owns(module) {
		return true
	}
	return s.entities[module+"/"+name]
}

// viewRoutable reports whether a derived route for a non-entity kind exists.
func (s appSurface) viewRoutable(module, route string) bool {
	if !s.gate || !s.owns(module) {
		return true
	}
	return s.routes[route]
}

// gated reports whether a module is subject to the App surface allowlist.
func (s appSurface) gated(module string) bool {
	return s.gate && s.owns(module)
}

// ─── Public grant derivation ───

// PublicGrantInput is the App surface a grant is derived from. It mirrors the
// fields AppContext carries for an App, so both `internal/api` (which owns the
// resolved Apps) and the meta bundle can derive from the same pieces.
type PublicGrantInput struct {
	// Modules is the set of modules the App explicitly mounts.
	Modules map[string]bool
	// Menu is the App's resolved menu tree (adopt nodes spliced, view leaves
	// resolved to routes) — the same tree ResolvedApp.Menu carries.
	Menu []spec.MenuItem
	// RegisteredViews is App.spec.registered_views.
	RegisteredViews []spec.RegisteredViewDecl
}

// DerivePublicGrants computes the anonymous allowlist for one public App from
// the surface it exposes (plan docs_internal/plan/implicit-public-grants.md).
//
// The rule is the client's FETCH graph, not the spec's reference graph: only
// constructs that make the renderer issue a request to the entity API grant an
// action. A `picker.display.category_field`, an inlined relation alias, a
// `computed` field and a snapshot all resolve without a client request, so they
// grant nothing — which is why derivation is more precise than the hand-written
// allowlist it replaces (it drops grants the client never used).
//
// Every grant is further gated by the view's own `public` flag: a view that
// declares `public: false` contributes nothing, so placing a staff page inside
// a public App does not leak its data.
//
// `delete` is NEVER granted implicitly — it is an administrative operation that
// belongs in a private App.
func (r *Registry) DerivePublicGrants(entities EntityLister, in PublicGrantInput) []spec.PublicEntityDecl {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ix := newEntityIndex(entities)
	owns := func(m string) bool { return in.Modules[m] }
	surface := r.computeAppSurfaceLocked(ix, in.Modules != nil, owns, in.Menu, in.RegisteredViews)

	b := &grantBuilder{r: r, surface: surface, specs: map[string]*spec.EntitySpec{}, g: newGrantSet()}
	for _, d := range entities() {
		if d.Spec != nil {
			b.specs[d.Module+"/"+d.Name] = d.Spec
		}
	}

	// 1. Entities the surface exposes directly — a menu leaf pointing at a
	//    derived list route, or `registered_views: [{entity: X}]`, which exposes
	//    the entity's derived CRUD views (list, /new, /:id, /:id/edit — see
	//    shell/router.tsx buildRoutes). `delete` has no derived route.
	for ref := range surface.entities {
		mod, name, ok := strings.Cut(ref, "/")
		if !ok {
			continue
		}
		b.addCanonical(mod, name, "list")
		b.addCanonical(mod, name, "find")
		b.addCanonical(mod, name, "create")
		b.addCanonical(mod, name, "update")
	}

	// 2. Authored views whose renderer issues fetches.
	for _, e := range r.Pages {
		if !surface.gated(e.Module) || !surface.routes[e.Spec.Route] || !spec.IsPublic(e.Spec.Public) {
			continue
		}
		b.visitPage(e)
	}
	for _, e := range r.Forms {
		if !surface.gated(e.Module) || !surface.routes["/"+e.Module+"/form/"+e.Name] || !spec.IsPublic(e.Spec.Public) {
			continue
		}
		b.visitForm(e.Module, e.Spec)
	}
	for _, e := range r.Tables {
		if !surface.gated(e.Module) || !surface.routes["/"+e.Module+"/table/"+e.Name] || !spec.IsPublic(e.Spec.Public) {
			continue
		}
		b.addRef(e.Module, e.Spec.Entity, "list", nil)
	}
	// Kinds whose renderer reads a list of their entity.
	for _, e := range r.Kanbans {
		if b.viewReachable(e.Module, "/kanban/"+e.Name, e.Spec.Public) {
			b.addRef(e.Module, e.Spec.Entity, "list", nil)
		}
	}
	for _, e := range r.Timelines {
		if b.viewReachable(e.Module, "/timeline/"+e.Name, e.Spec.Public) {
			b.addRef(e.Module, e.Spec.Entity, "list", nil)
		}
	}
	for _, e := range r.Reports {
		if b.viewReachable(e.Module, "/report/"+e.Name, e.Spec.Public) {
			b.addRef(e.Module, e.Spec.Entity, "list", nil)
		}
	}
	for _, e := range r.Listings {
		// A Listing has no `public` flag — a public catalog is its purpose, so
		// a reachable one always contributes.
		if b.viewReachable(e.Module, "/listing/"+e.Name, nil) {
			b.addRef(e.Module, e.Spec.Entity, "list", nil)
		}
	}
	for _, e := range r.Calendars {
		if b.viewReachable(e.Module, "/calendar/"+e.Name, e.Spec.Public) {
			b.addRef(e.Module, e.Spec.Entity, "list", nil)
		}
	}

	return b.g.decls()
}

// grantBuilder traverses one App's reachable, public views and records the
// entity actions their renderers actually fetch.
type grantBuilder struct {
	r       *Registry
	surface appSurface
	specs   map[string]*spec.EntitySpec // "module/name" → spec
	g       *grantSet
}

func (b *grantBuilder) lookup(module, name string) (*spec.EntitySpec, bool) {
	es, ok := b.specs[module+"/"+name]
	return es, ok
}

// viewReachable reports whether a standalone view of a non-entity kind is both
// on the surface and public.
func (b *grantBuilder) viewReachable(module, route string, public *bool) bool {
	return b.surface.gated(module) && b.surface.routes[route] && spec.IsPublic(public)
}

// addCanonical records an action for an entity already known by (module, name).
func (b *grantBuilder) addCanonical(module, name, action string) {
	if module == "" || name == "" {
		return
	}
	b.g.add(module+"/"+name, action, nil)
}

// addRef records an action for an entity reference, which may be module-local
// ("order") or cross-module ("cafe-order.order" / "cafe-order/order").
func (b *grantBuilder) addRef(module, ref, action string, scope []spec.FilterSpec) {
	if ref == "" {
		return
	}
	_, mod, name, ok := resolveEntityRef(b.lookup, module, ref)
	if !ok {
		return
	}
	b.g.add(mod+"/"+name, action, scope)
}

// visitPage walks one authored Page: its blocks (form/table/widget), render
// context, and the declared footprint of a `mode: custom` page. Section/HTML
// blocks fetch nothing.
func (b *grantBuilder) visitPage(e *Entry[spec.PageSpec]) {
	for i := range e.Spec.Context {
		if cd := &e.Spec.Context[i]; cd.Source == "entity" {
			b.addRef(e.Module, cd.Entity, "find", nil)
		}
	}
	if binds := e.Spec.Binds; binds != nil {
		for _, ref := range binds.Entities {
			b.addRef(e.Module, ref, "list", nil)
			b.addRef(e.Module, ref, "find", nil)
		}
		for _, act := range binds.Actions {
			mod, ent, action := splitActionRef(act)
			if action == "" {
				continue
			}
			b.addRef(e.Module, mod+"."+ent, action, nil)
		}
	}
	b.visitBlocks(e.Module, e.Spec.Blocks)
	for i := range e.Spec.Tabs {
		tab := &e.Spec.Tabs[i]
		b.visitBlock(e.Module, spec.PageBlock{Form: tab.Form, Table: tab.Table, Component: tab.Component})
	}
}

// visitBlocks visits every block that resolves to a data-fetching renderer.
func (b *grantBuilder) visitBlocks(module string, blocks []spec.PageBlock) {
	for i := range blocks {
		b.visitBlock(module, blocks[i])
	}
}

// visitBlock visits one block: a Form ref (recurse), a Table ref (list, plus
// any route-param row scope), or a Widget ref (list). Component/HTML/section
// blocks fetch nothing.
func (b *grantBuilder) visitBlock(module string, blk spec.PageBlock) {
	switch {
	case blk.Form != nil && blk.Form.Ref != "":
		if fe := b.r.Forms[blk.Form.Ref]; fe != nil && fe.Module == module {
			b.visitForm(module, fe.Spec)
		}
	case blk.Table != nil && blk.Table.Ref != "":
		te := b.r.Tables[blk.Table.Ref]
		if te == nil || te.Module != module {
			return
		}
		b.addRef(module, te.Spec.Entity, "list", blockScope(blk.Table.Param))
	case blk.Widget != nil && blk.Widget.Ref != "":
		if we := b.r.Widgets[blk.Widget.Ref]; we != nil && we.Module == module && we.Spec.Entity != "" {
			b.addRef(module, we.Spec.Entity, "list", nil)
		}
	}
}

// visitForm walks one FormSpec: its entity (by mode), render context, and the
// fields it actually renders. A field the form does not render contributes
// nothing — a `widget: hidden` relation is stored, not picked, so no
// RelationPicker is mounted and no fetch happens.
func (b *grantBuilder) visitForm(module string, fs *spec.FormSpec) {
	if fs.AuthAction != "" {
		return // auth forms carry no entity
	}
	switch fs.Mode {
	case "create":
		b.addRef(module, fs.Entity, "create", nil)
	case "edit":
		b.addRef(module, fs.Entity, "update", nil)
		b.addRef(module, fs.Entity, "find", nil)
	default: // "view" or empty
		b.addRef(module, fs.Entity, "find", nil)
	}

	for i := range fs.Context {
		if cd := &fs.Context[i]; cd.Source == "entity" {
			b.addRef(module, cd.Entity, "find", nil)
		}
	}

	if es := b.entitySpec(module, fs.Entity); es != nil {
		rendered := renderedFieldNames(fs)
		for i := range es.Fields {
			f := &es.Fields[i]
			if !rendered[f.Name] || hiddenWidget(fs, f.Name) {
				continue
			}
			b.visitField(module, f)
		}
	}
}

// visitField derives the fetches one rendered entity field performs.
func (b *grantBuilder) visitField(module string, f *spec.Field) {
	// Relation field → the RelationPicker fetches candidates (list) and, for a
	// set value, the selected record (find).
	if f.Type == "relation" && f.Relation != nil && f.Relation.Resource != "" {
		b.addRef(module, f.Relation.Resource, "list", nil)
		b.addRef(module, f.Relation.Resource, "find", nil)
		return
	}
	// Child field with a picker → PickerPanel fetches the source list and,
	// when a price entity is declared, joins it client-side with its own list.
	if f.Type == "child" && f.Child != nil && f.Child.Picker != nil {
		p := f.Child.Picker
		b.addRef(module, p.Entity, "list", nil)
		if p.Display.PriceEntity != "" {
			b.addRef(module, p.Display.PriceEntity, "list", nil)
		}
	}
}

// entitySpec resolves a form's entity reference to its spec.
func (b *grantBuilder) entitySpec(module, ref string) *spec.EntitySpec {
	es, _, _, ok := resolveEntityRef(b.lookup, module, ref)
	if !ok {
		return nil
	}
	return es
}

// renderedFieldNames collects the field names a form declares in its sections.
func renderedFieldNames(fs *spec.FormSpec) map[string]bool {
	out := map[string]bool{}
	for i := range fs.Sections {
		for j := range fs.Sections[i].Fields {
			if name := fs.Sections[i].Fields[j].Field; name != "" {
				out[name] = true
			}
		}
	}
	return out
}

// hiddenWidget reports whether the form renders this field with `widget: hidden`
// — stored in form state but not mounted, so no control and no fetch.
func hiddenWidget(fs *spec.FormSpec, name string) bool {
	for i := range fs.Sections {
		for j := range fs.Sections[i].Fields {
			f := &fs.Sections[i].Fields[j]
			if f.Field == name {
				return f.Widget == "hidden"
			}
		}
	}
	return false
}

// blockScope turns a table block's `param` into the row scope the server must
// enforce for anonymous reads. A value of ":name" means the parameter is
// supplied by the route — the client cannot widen or drop it, which is what
// makes it a safe server-enforced scope. A literal value is NOT a scope: the
// client could change it, so it stays a client-side filter.
func blockScope(param map[string]any) []spec.FilterSpec {
	if len(param) == 0 {
		return nil
	}
	keys := make([]string, 0, len(param))
	for k := range param {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]spec.FilterSpec, 0, len(keys))
	for _, k := range keys {
		s, ok := param[k].(string)
		if !ok || !strings.HasPrefix(s, ":") {
			continue
		}
		out = append(out, spec.FilterSpec{Field: k, Op: "eq", From: "route", Param: strings.TrimPrefix(s, ":")})
	}
	return out
}

// splitActionRef splits "module.entity.action" (or "module/entity/action") into
// its three parts. Returns an empty action when the shape does not match.
func splitActionRef(ref string) (module, entity, action string) {
	sep := "."
	if strings.Contains(ref, "/") {
		sep = "/"
	}
	parts := strings.Split(ref, sep)
	if len(parts) < 3 {
		return "", "", ""
	}
	action = parts[len(parts)-1]
	entity = parts[len(parts)-2]
	module = strings.Join(parts[:len(parts)-2], sep)
	return module, entity, action
}

// ─── Grant accumulation ───

// grantSet accumulates (entity → actions) plus any row scope, keyed by
// canonical "module/name".
type grantSet struct {
	actions map[string]map[string]bool
	scopes  map[string][]spec.FilterSpec
	order   []string
}

func newGrantSet() *grantSet {
	return &grantSet{actions: map[string]map[string]bool{}, scopes: map[string][]spec.FilterSpec{}}
}

func (g *grantSet) add(ref, action string, scope []spec.FilterSpec) {
	if ref == "" || action == "" {
		return
	}
	if g.actions[ref] == nil {
		g.actions[ref] = map[string]bool{}
		g.order = append(g.order, ref)
	}
	g.actions[ref][action] = true
	if len(scope) > 0 {
		g.scopes[ref] = scope
	}
}

// decls emits the allowlist, sorted by entity for deterministic output.
//
// `find` is dropped when a scope is present: find resolves by id, which a row
// scope cannot guard, so the grant would look filtered while returning any
// record whose id is known — the rule the removed manifest validator enforced.
func (g *grantSet) decls() []spec.PublicEntityDecl {
	refs := append([]string(nil), g.order...)
	sort.Strings(refs)

	out := make([]spec.PublicEntityDecl, 0, len(refs))
	for _, ref := range refs {
		acts := g.actions[ref]
		if len(acts) == 0 {
			continue
		}
		scope := g.scopes[ref]
		if len(scope) > 0 {
			delete(acts, "find")
		}
		list := make([]string, 0, len(acts))
		for _, a := range grantActionOrder {
			if acts[a] {
				list = append(list, a)
			}
		}
		if len(list) == 0 {
			continue
		}
		out = append(out, spec.PublicEntityDecl{Entity: ref, Actions: list, Scope: scope})
	}
	return out
}

// grantActionOrder fixes the action order in the emitted declaration.
var grantActionOrder = []string{"list", "find", "create", "update", "delete"}
