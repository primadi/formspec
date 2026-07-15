// Package app resolves kind: App and kind: Module manifests into fully-formed
// Apps: validated module sets and a final navigation tree with every leaf
// resolved to a concrete route (Core Basic §4.4/§4.5).
//
// A Workspace MAY contain more than one App; all Apps in a workspace run
// simultaneously, distinguished by root_url. The same Module MAY be mounted
// by more than one App.
package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/forma/internal/manifest"
	"github.com/primadi/forma/internal/ui"
	"github.com/primadi/forma/pkg/spec"
)

// ResolvedApp is one fully-resolved App: its spec, the set of modules it
// mounts, and its final menu tree (adopt nodes spliced, view leaves resolved
// to concrete routes).
type ResolvedApp struct {
	Name    string
	Spec    *spec.AppSpec
	Modules map[string]bool
	Menu    []spec.MenuItem
}

// Resolve parses every kind: App / kind: Module manifest out of manifests,
// validates them, and produces one ResolvedApp per App.
func Resolve(manifests []manifest.RawManifest, uiReg *ui.Registry) (map[string]*ResolvedApp, error) {
	apps := map[string]*spec.AppSpec{}
	appSources := map[string]string{}
	modules := map[string]*spec.ModuleSpec{}
	moduleSources := map[string]string{}

	for _, raw := range manifests {
		switch spec.Kind(raw.Kind) {
		case spec.KindApp:
			as, err := manifest.RawSpecToAppSpec(raw.Spec)
			if err != nil {
				return nil, fmt.Errorf("%s: parse App spec: %w", raw.Source, err)
			}
			name := raw.Metadata.Name
			if existing, ok := appSources[name]; ok {
				return nil, fmt.Errorf("%s: duplicate App name %q (first defined at %s)", raw.Source, name, existing)
			}
			apps[name] = as
			appSources[name] = raw.Source
		case spec.KindModule:
			ms, err := manifest.RawSpecToModuleSpec(raw.Spec)
			if err != nil {
				return nil, fmt.Errorf("%s: parse Module spec: %w", raw.Source, err)
			}
			name := raw.Metadata.Name
			if existing, ok := moduleSources[name]; ok {
				return nil, fmt.Errorf("%s: duplicate Module name %q (first defined at %s)", raw.Source, name, existing)
			}
			modules[name] = ms
			moduleSources[name] = raw.Source
		}
	}

	rootURLs := map[string]string{}
	for _, name := range sortedAppNames(apps) {
		as := apps[name]
		if as.RootURL == "" {
			return nil, fmt.Errorf("app %q: spec.root_url is required", name)
		}
		// The renderer SPA is only mounted at /{workspace}/app/* (and
		// /{workspace}/_admin, which isn't App-scoped) — see
		// internal/api/router.go BuildHTTP. root_url must live under that
		// mount for the static handler to actually serve the App's shell.
		if as.RootURL != "/app" && !strings.HasPrefix(as.RootURL, "/app/") {
			return nil, fmt.Errorf("app %q: root_url %q must start with \"/app/\"", name, as.RootURL)
		}
		if existing, ok := rootURLs[as.RootURL]; ok {
			return nil, fmt.Errorf("app %q: root_url %q already used by app %q", name, as.RootURL, existing)
		}
		rootURLs[as.RootURL] = name
	}

	result := map[string]*ResolvedApp{}
	for _, name := range sortedAppNames(apps) {
		as := apps[name]

		moduleSet := map[string]bool{}
		for _, m := range as.Modules {
			if _, ok := modules[m]; !ok {
				return nil, fmt.Errorf("app %q: modules[] references unknown module %q", name, m)
			}
			moduleSet[m] = true
		}

		resolvedMenu, err := resolveMenuList(as.Menu, moduleSet, modules, uiReg, 1)
		if err != nil {
			return nil, fmt.Errorf("app %q: menu: %w", name, err)
		}

		result[name] = &ResolvedApp{
			Name:    name,
			Spec:    as,
			Modules: moduleSet,
			Menu:    resolvedMenu,
		}
	}

	return result, nil
}

func sortedAppNames(apps map[string]*spec.AppSpec) []string {
	names := make([]string, 0, len(apps))
	for n := range apps {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// resolveMenuList validates and resolves one list of MenuItem siblings at the
// given nesting level (1-indexed). Adopt nodes expand into zero-or-more items
// (the spliced module's own root list), so the output length may differ from
// the input.
func resolveMenuList(items []spec.MenuItem, moduleSet map[string]bool, modules map[string]*spec.ModuleSpec, uiReg *ui.Registry, level int) ([]spec.MenuItem, error) {
	if level > 3 {
		return nil, fmt.Errorf("menu nesting exceeds the 3-level cap")
	}
	var out []spec.MenuItem
	for i, item := range items {
		resolved, err := resolveMenuItem(item, moduleSet, modules, uiReg, level)
		if err != nil {
			return nil, fmt.Errorf("item %d (%q): %w", i, item.Label, err)
		}
		out = append(out, resolved...)
	}
	return out, nil
}

// resolveMenuItem validates a single MenuItem's shape (adopt / group / leaf,
// Core Basic §4.4) and resolves it. Adopt nodes return the spliced module
// suggestion (possibly more than one item); group/leaf nodes return exactly
// one resolved item.
func resolveMenuItem(item spec.MenuItem, moduleSet map[string]bool, modules map[string]*spec.ModuleSpec, uiReg *ui.Registry, level int) ([]spec.MenuItem, error) {
	isAdopt := item.Type == "module"
	isGroup := len(item.Children) > 0

	switch {
	case isAdopt:
		if level != 1 {
			return nil, fmt.Errorf("type: module is only allowed at level 1")
		}
		if item.Module == "" {
			return nil, fmt.Errorf("type: module requires `module`")
		}
		if item.Label != "" || item.Icon != "" || item.View != "" || item.Route != "" || len(item.Children) > 0 {
			return nil, fmt.Errorf("type: module node cannot set label/icon/view/route/children")
		}
		if !moduleSet[item.Module] {
			return nil, fmt.Errorf("module %q is not in this App's spec.modules", item.Module)
		}
		mod, ok := modules[item.Module]
		if !ok {
			return nil, fmt.Errorf("module %q not found", item.Module)
		}
		stamped := stampModule(mod.Menu, item.Module)
		resolved, err := resolveMenuList(stamped, moduleSet, modules, uiReg, 1)
		if err != nil {
			return nil, fmt.Errorf("adopted menu from module %q: %w", item.Module, err)
		}
		if item.Order != 0 {
			for i := range resolved {
				resolved[i].Order = item.Order
			}
		}
		return resolved, nil

	case isGroup:
		if item.Module != "" || item.View != "" || item.Route != "" {
			return nil, fmt.Errorf("group node (has children) cannot set module/view/route directly — only its children may")
		}
		if item.Label == "" {
			return nil, fmt.Errorf("group node requires a label")
		}
		children, err := resolveMenuList(item.Children, moduleSet, modules, uiReg, level+1)
		if err != nil {
			return nil, err
		}
		item.Children = children
		return []spec.MenuItem{item}, nil

	default: // leaf
		if item.Label == "" {
			return nil, fmt.Errorf("leaf node requires a label")
		}
		if item.Module == "" {
			return nil, fmt.Errorf("leaf node requires a module")
		}
		if !moduleSet[item.Module] {
			return nil, fmt.Errorf("module %q is not in this App's spec.modules", item.Module)
		}
		hasView := item.View != ""
		hasRoute := item.Route != ""
		if hasView == hasRoute {
			return nil, fmt.Errorf("leaf node requires exactly one of view/route")
		}
		if hasView {
			route, err := uiReg.ResolveViewRoute(item.Module, item.View)
			if err != nil {
				return nil, err
			}
			item.Route = route
		}
		return []spec.MenuItem{item}, nil
	}
}

// stampModule fills in Module on every leaf of a Module.spec.menu tree
// (module-relative items never set it themselves) before it's spliced into
// an App's menu. Group nodes are left alone — Module stays forbidden there.
func stampModule(items []spec.MenuItem, module string) []spec.MenuItem {
	out := make([]spec.MenuItem, len(items))
	for i, item := range items {
		item.Children = stampModule(item.Children, module)
		if len(item.Children) == 0 && item.Type != "module" && item.Module == "" {
			item.Module = module
		}
		out[i] = item
	}
	return out
}
