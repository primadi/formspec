package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// Gap #21 (kafe 8.3): an App that mounts a module no `kind: Module` declares,
// or a menu item whose `view:` names no registered view, must be refused — both
// otherwise validate green while the App mounts nothing / the menu navigates
// nowhere.
func TestValidateDanglingRefs(t *testing.T) {
	module := func(name string) manifest.RawManifest {
		return manifest.RawManifest{
			APIVersion: "formspec.dev/v1",
			Kind:       "Module",
			Source:     name + "/module.yaml",
			Metadata:   manifest.RawMetadata{Name: name},
			Spec:       map[string]any{"version": "1.0.0"},
		}
	}
	form := func(module, name string) manifest.RawManifest {
		return manifest.RawManifest{
			APIVersion: "formspec.dev/v1",
			Kind:       "Form",
			Source:     module + "/" + name + ".yaml",
			Metadata:   manifest.RawMetadata{Name: name, Module: module},
			Spec:       map[string]any{"entity": module + ".thing"},
		}
	}
	app := func(modules []any, menu []any) manifest.RawManifest {
		body := map[string]any{
			"version":  "1.0.0",
			"root_url": "/app/test",
			"modules":  modules,
		}
		if menu != nil {
			body["menu"] = menu
		}
		return manifest.RawManifest{
			APIVersion: "formspec.dev/v1",
			Kind:       "App",
			Source:     "apps/test.yaml",
			Metadata:   manifest.RawMetadata{Name: "test"},
			Spec:       body,
		}
	}

	t.Run("declared module and resolvable view are accepted", func(t *testing.T) {
		rejects := validateDanglingRefs([]manifest.RawManifest{
			module("alpha"),
			form("alpha", "thing-form"),
			app([]any{"alpha"}, []any{map[string]any{"label": "Things", "view": "thing-form"}}),
		})
		if len(rejects) != 0 {
			t.Fatalf("expected a clean App, got %v", rejects)
		}
	})

	t.Run("module that no Module declares is rejected", func(t *testing.T) {
		rejects := validateDanglingRefs([]manifest.RawManifest{
			module("alpha"),
			app([]any{"alpha", "ghost"}, nil),
		})
		msg, ok := rejects["apps/test.yaml"]
		if !ok {
			t.Fatalf("expected rejection of the unknown module, got %v", rejects)
		}
		if !strings.Contains(msg, "ghost") {
			t.Errorf("error %q should name the missing module", msg)
		}
	})

	t.Run("menu view that resolves to nothing is rejected", func(t *testing.T) {
		rejects := validateDanglingRefs([]manifest.RawManifest{
			module("alpha"),
			form("alpha", "thing-form"),
			app([]any{"alpha"}, []any{map[string]any{"label": "Ghost", "view": "no-such-view"}}),
		})
		msg, ok := rejects["apps/test.yaml"]
		if !ok {
			t.Fatalf("expected rejection of the dangling view, got %v", rejects)
		}
		if !strings.Contains(msg, "no-such-view") {
			t.Errorf("error %q should name the missing view", msg)
		}
	})

	t.Run("nested menu view is checked", func(t *testing.T) {
		rejects := validateDanglingRefs([]manifest.RawManifest{
			module("alpha"),
			form("alpha", "thing-form"),
			app([]any{"alpha"}, []any{map[string]any{
				"label": "Group",
				"children": []any{
					map[string]any{"label": "Ghost", "view": "no-such-view"},
				},
			}}),
		})
		if _, ok := rejects["apps/test.yaml"]; !ok {
			t.Fatalf("expected rejection of the nested dangling view, got %v", rejects)
		}
	})

	// Regression (kafe-pos, todo 5.25.9): Widget, Print, ApprovalInbox and
	// NotificationCenter each have a client route registered by buildRoutes, but
	// were absent from viewKinds — so a genuinely navigable menu entry
	// (`view: approval-inbox:supervisor-inbox`) was refused with "would navigate
	// nowhere". An author's only escape was to NOT put the view in the menu,
	// which is the opposite of what the surface rule wants.
	t.Run("every kind with a client route is accepted as a menu view", func(t *testing.T) {
		view := func(kind, name string) manifest.RawManifest {
			return manifest.RawManifest{
				APIVersion: "formspec.dev/v1",
				Kind:       kind,
				Source:     "alpha/" + name + ".yaml",
				Metadata:   manifest.RawMetadata{Name: name, Module: "alpha"},
				Spec:       map[string]any{},
			}
		}
		manifests := []manifest.RawManifest{module("alpha")}
		menu := []any{}
		for _, v := range []struct{ kind, name string }{
			{"Widget", "catalog-metric"},
			{"Print", "receipt"},
			{"ApprovalInbox", "supervisor-inbox"},
			{"NotificationCenter", "alerts"},
		} {
			manifests = append(manifests, view(v.kind, v.name))
			menu = append(menu, map[string]any{"label": v.name, "view": v.name})
		}
		manifests = append(manifests, app([]any{"alpha"}, menu))

		if rejects := validateDanglingRefs(manifests); len(rejects) != 0 {
			t.Fatalf("expected these views to be accepted, got %v", rejects)
		}
	})
}
