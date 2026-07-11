// Package ui parses, indexes, and validates the frontend UI kinds
// (Frontend Spec §2–§13: Page, Form, Table, Dashboard, Widget, Report,
// Wizard, Kanban, Timeline, Menu, Print, Theme) and serves them to the
// manifest-driven renderer through the Meta API (implemented in internal/api).
//
// Design doc: docs/implementation/frontend-renderer.md §4.1–§4.2.
package ui

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/primadi/forma/internal/manifest"
	"github.com/primadi/forma/pkg/spec"
)

// Entry is one parsed UI manifest of kind T.
type Entry[T any] struct {
	Name        string `json:"name"`
	Module      string `json:"module"`
	Description string `json:"description,omitempty"`
	Source      string `json:"-"`
	Spec        *T     `json:"spec"`
}

// Registry indexes all frontend-kind manifests by metadata.name.
// Names are the reference keys used by Page blocks, Dashboard widgets, and
// Wizard steps, so they must be unique per kind across modules.
type Registry struct {
	mu sync.RWMutex

	Pages      map[string]*Entry[spec.PageSpec]
	Forms      map[string]*Entry[spec.FormSpec]
	Tables     map[string]*Entry[spec.TableSpec]
	Dashboards map[string]*Entry[spec.DashboardSpec]
	Widgets    map[string]*Entry[spec.WidgetSpec]
	Reports    map[string]*Entry[spec.ReportSpec]
	Wizards    map[string]*Entry[spec.WizardSpec]
	Kanbans    map[string]*Entry[spec.KanbanSpec]
	Timelines  map[string]*Entry[spec.TimelineSpec]
	Menus      map[string]*Entry[spec.MenuSpec]
	Prints     map[string]*Entry[spec.PrintSpec]
	Themes     map[string]*Entry[spec.ThemeSpec]
}

// NewRegistry creates an empty UI registry.
func NewRegistry() *Registry {
	return &Registry{
		Pages:      map[string]*Entry[spec.PageSpec]{},
		Forms:      map[string]*Entry[spec.FormSpec]{},
		Tables:     map[string]*Entry[spec.TableSpec]{},
		Dashboards: map[string]*Entry[spec.DashboardSpec]{},
		Widgets:    map[string]*Entry[spec.WidgetSpec]{},
		Reports:    map[string]*Entry[spec.ReportSpec]{},
		Wizards:    map[string]*Entry[spec.WizardSpec]{},
		Kanbans:    map[string]*Entry[spec.KanbanSpec]{},
		Timelines:  map[string]*Entry[spec.TimelineSpec]{},
		Menus:      map[string]*Entry[spec.MenuSpec]{},
		Prints:     map[string]*Entry[spec.PrintSpec]{},
		Themes:     map[string]*Entry[spec.ThemeSpec]{},
	}
}

// LoadDir discovers and loads all frontend-kind manifests under basePath.
// Non-frontend kinds are ignored. Parse errors are collected best-effort.
func (r *Registry) LoadDir(basePath string) []error {
	loader := manifest.NewLoader(basePath)
	result, err := loader.LoadAll()
	if err != nil {
		return []error{fmt.Errorf("ui manifest load: %w", err)}
	}
	var errs []error
	for _, pe := range result.Errors {
		pe := pe
		errs = append(errs, &pe)
	}
	errs = append(errs, r.Load(result.Manifests)...)
	return errs
}

// Load parses and registers frontend-kind manifests from an already-loaded
// raw manifest list (shared with the entity registry's LoadAll pass).
func (r *Registry) Load(manifests []manifest.RawManifest) []error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, raw := range manifests {
		if err := r.register(raw); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (r *Registry) register(raw manifest.RawManifest) error {
	switch spec.Kind(raw.Kind) {
	case spec.KindPage:
		return registerInto(r.Pages, raw)
	case spec.KindForm:
		return registerInto(r.Forms, raw)
	case spec.KindTable:
		return registerInto(r.Tables, raw)
	case spec.KindDashboard:
		return registerInto(r.Dashboards, raw)
	case spec.KindWidget:
		return registerInto(r.Widgets, raw)
	case spec.KindReport:
		return registerInto(r.Reports, raw)
	case spec.KindWizard:
		return registerInto(r.Wizards, raw)
	case spec.KindKanban:
		return registerInto(r.Kanbans, raw)
	case spec.KindTimeline:
		return registerInto(r.Timelines, raw)
	case spec.KindMenu:
		return registerInto(r.Menus, raw)
	case spec.KindPrint:
		return registerInto(r.Prints, raw)
	case spec.KindTheme:
		return registerInto(r.Themes, raw)
	default:
		return nil // not a frontend kind
	}
}

func registerInto[T any](m map[string]*Entry[T], raw manifest.RawManifest) error {
	parsed, err := manifest.RawSpecTo[T](raw.Spec)
	if err != nil {
		return fmt.Errorf("%s: parse %s spec: %w", raw.Source, raw.Kind, err)
	}
	name := raw.Metadata.Name
	if existing, ok := m[name]; ok {
		return fmt.Errorf("%s: duplicate %s name %q (first defined at %s)",
			raw.Source, raw.Kind, name, existing.Source)
	}
	m[name] = &Entry[T]{
		Name:        name,
		Module:      raw.Metadata.Module,
		Description: raw.Metadata.Description,
		Source:      raw.Source,
		Spec:        parsed,
	}
	return nil
}

// Count returns the total number of registered UI manifests.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Pages) + len(r.Forms) + len(r.Tables) + len(r.Dashboards) +
		len(r.Widgets) + len(r.Reports) + len(r.Wizards) + len(r.Kanbans) +
		len(r.Timelines) + len(r.Menus) + len(r.Prints) + len(r.Themes)
}

// EntityResolver resolves an entity reference to its spec. Implemented by
// the entity registry (adapter in resource/forma.go).
type EntityResolver func(module, name string) (*spec.EntitySpec, bool)

// resolveEntityRef resolves "customer" (module-local) or "clinic.visit"
// (cross-module) relative to the referencing manifest's module.
func resolveEntityRef(resolve EntityResolver, defaultModule, ref string) (*spec.EntitySpec, string, string, bool) {
	module, name := defaultModule, ref
	if i := strings.IndexByte(ref, '.'); i > 0 {
		module, name = ref[:i], ref[i+1:]
	}
	es, ok := resolve(module, name)
	return es, module, name, ok
}

// sortedKeys returns map keys in deterministic order (for stable output).
func sortedKeys[T any](m map[string]*Entry[T]) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
