// Cross-manifest dangling-reference validation (kafe ledger 8.3 / gap #21).
//
// Two references in an App manifest point at resources that live elsewhere in
// the spec tree, and neither was checked — so a typo validated green while the
// App silently mounted nothing (or a menu item pointed at a view that does not
// exist):
//
//  1. `App.spec.modules` — a module name that matches no `kind: Module` means
//     the App mounts a module that is not there. The bundle is simply smaller
//     than the author intended, with no error.
//  2. `MenuItem.view` — a view name that matches no registered Form/Table/Page/
//     Wizard/Report/etc. means the menu entry navigates nowhere.
//
// Both are answerable with the whole spec tree in view, so they are refused
// here. (`impl.ref` to a missing `.star` is already covered by the honesty scan
// in 8.7.)
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// viewKinds are the kinds a MenuItem.view may name — the "View resources" the
// menu can navigate to.
var viewKinds = map[spec.Kind]bool{
	spec.KindPage:      true,
	spec.KindForm:      true,
	spec.KindTable:     true,
	spec.KindWizard:    true,
	spec.KindReport:    true,
	spec.KindKanban:    true,
	spec.KindTimeline:  true,
	spec.KindCalendar:  true,
	spec.KindDashboard: true,
	spec.KindListing:   true,
}

// validateDanglingRefs checks App.module and MenuItem.view references against
// the rest of the spec tree. Returns a map of manifest source → error message.
func validateDanglingRefs(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}

	// Index the declared modules and the registered views.
	modules := map[string]bool{}
	views := map[string]bool{} // "module/name" and bare "name"
	for _, m := range manifests {
		switch spec.Kind(m.Kind) {
		case spec.KindModule:
			modules[m.Metadata.Name] = true
		default:
			if viewKinds[spec.Kind(m.Kind)] {
				views[m.Metadata.Module+"/"+m.Metadata.Name] = true
				views[m.Metadata.Name] = true
			}
		}
	}

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindApp || m.Spec == nil {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		app, err := manifest.RawSpecToAppSpec(sm)
		if err != nil || app == nil {
			continue
		}

		// 1. App.spec.modules must name declared modules.
		var missing []string
		for _, mod := range app.Modules {
			if !modules[mod] {
				missing = append(missing, mod)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			rejects[m.Source] = fmt.Sprintf(
				"App mounts module(s) %s, which no `kind: Module` declares — the App would mount nothing for them (declared modules: %s)",
				strings.Join(missing, ", "), strings.Join(sortedKeys(modules), ", "))
			continue
		}

		// 2. MenuItem.view must name a registered view.
		if msg := danglingMenuView(app.Menu, views); msg != "" {
			rejects[m.Source] = msg
		}
	}

	return rejects
}

// danglingMenuView walks the menu tree and returns the first view reference that
// resolves to nothing, or "" when all resolve.
func danglingMenuView(items []spec.MenuItem, views map[string]bool) string {
	for _, it := range items {
		if it.View != "" && !views[it.View] {
			return fmt.Sprintf(
				"menu item %q references view %q, which is not a registered Form/Table/Page/Wizard/Report/Kanban/Timeline/Calendar/Dashboard/Listing — the menu entry would navigate nowhere",
				it.Label, it.View)
		}
		if msg := danglingMenuView(it.Children, views); msg != "" {
			return msg
		}
	}
	return ""
}

// sortedKeys returns the map's keys sorted, for a stable error message.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
