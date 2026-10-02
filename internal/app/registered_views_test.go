package app

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
)

// registered_views resolve-time validation (plan registered-views.md): an entry
// that names a view/entity which does not exist contributes nothing to the
// reachable surface, so it must fail at boot rather than silently expose less
// than the manifest claims.

func moduleManifest(name string) manifest.RawManifest {
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Module",
		Metadata:   manifest.RawMetadata{Name: name},
		Spec:       map[string]any{"version": "1.0.0"},
	}
}

func entityManifest(module, name string) manifest.RawManifest {
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Metadata:   manifest.RawMetadata{Name: name, Module: module},
		Spec:       map[string]any{},
	}
}

// appWithViews builds a kind: App raw manifest mounting modules and declaring
// registered_views.
func appWithViews(name string, modules []string, views []map[string]any) manifest.RawManifest {
	ms := make([]any, len(modules))
	for i, m := range modules {
		ms[i] = m
	}
	vs := make([]any, len(views))
	for i, v := range views {
		vs[i] = v
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "App",
		Metadata:   manifest.RawMetadata{Name: name, Module: "core"},
		Spec: map[string]any{
			"root_url":         "/app/" + name,
			"modules":          ms,
			"registered_views": vs,
		},
	}
}

// pageRegistry returns a UI registry carrying one Page so `view:` refs resolve.
func pageRegistry(module, name, route string) *ui.Registry {
	r := ui.NewRegistry()
	r.Pages[name] = &ui.Entry[spec.PageSpec]{
		Name:   name,
		Module: module,
		Spec:   &spec.PageSpec{Route: route, Title: name},
	}
	return r
}

func TestResolve_RegisteredViews_Valid(t *testing.T) {
	manifests := []manifest.RawManifest{
		moduleManifest("cafe-order"),
		entityManifest("cafe-order", "order"),
		appWithViews("qr", []string{"cafe-order"}, []map[string]any{
			{"entity": "cafe-order/order"},
			{"view": "cafe-order/menu-catalog"},
		}),
	}
	if _, err := Resolve(manifests, pageRegistry("cafe-order", "menu-catalog", "/menu/:session_id")); err != nil {
		t.Fatalf("expected OK, got %v", err)
	}
}

func TestResolve_RegisteredViews_Rejects(t *testing.T) {
	cases := []struct {
		name      string
		manifests []manifest.RawManifest
		reg       *ui.Registry
		wantSub   string
	}{
		{
			name: "unknown entity",
			manifests: []manifest.RawManifest{
				moduleManifest("cafe-order"),
				appWithViews("qr", []string{"cafe-order"}, []map[string]any{
					{"entity": "cafe-order/ghost"},
				}),
			},
			reg:     ui.NewRegistry(),
			wantSub: "not found in module",
		},
		{
			name: "unknown view",
			manifests: []manifest.RawManifest{
				moduleManifest("cafe-order"),
				appWithViews("qr", []string{"cafe-order"}, []map[string]any{
					{"view": "cafe-order/ghost-page"},
				}),
			},
			reg:     ui.NewRegistry(),
			wantSub: "not found in module",
		},
		{
			name: "module not mounted",
			manifests: []manifest.RawManifest{
				moduleManifest("cafe-order"),
				moduleManifest("other"),
				appWithViews("qr", []string{"cafe-order"}, []map[string]any{
					{"view": "other/some-page"},
				}),
			},
			reg:     pageRegistry("other", "some-page", "/x"),
			wantSub: "is not mounted",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Resolve(c.manifests, c.reg)
			if err == nil {
				t.Fatalf("expected an error")
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), c.wantSub)
			}
		})
	}
}
