package app

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// appManifest builds a minimal kind: App raw manifest.
func appManifest(name, rootURL string, access string) manifest.RawManifest {
	specMap := map[string]any{
		"root_url": rootURL,
		"modules":  []any{},
	}
	if access != "" {
		specMap["access"] = access
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "App",
		Metadata:   manifest.RawMetadata{Name: name, Module: "core"},
		Spec:       specMap,
	}
}

func TestResolve_FlexibleRootURL(t *testing.T) {
	cases := []struct {
		name    string
		rootURL string
		access  string
		wantErr string
	}{
		// Free-form prefixes inside the workspace are now valid.
		{name: "workspace root", rootURL: "/"},
		{name: "single segment", rootURL: "/barbershop"},
		{name: "nested", rootURL: "/apps/barbershop"},
		{name: "legacy /app", rootURL: "/app"},
		{name: "legacy /app/*", rootURL: "/app/barbershop"},
		{name: "trailing slash normalized", rootURL: "/barbershop/"},
		// Reserved first segments collide with fixed engine surfaces.
		{name: "reserved _ui", rootURL: "/_ui", wantErr: "reserved segment"},
		{name: "reserved api", rootURL: "/api", wantErr: "reserved segment"},
		{name: "reserved _admin", rootURL: "/_admin", wantErr: "reserved segment"},
		{name: "reserved assets", rootURL: "/assets", wantErr: "reserved segment"},
		{name: "reserved health", rootURL: "/health", wantErr: "reserved segment"},
		{name: "reserved login", rootURL: "/login", wantErr: "reserved segment"},
		{name: "reserved register", rootURL: "/register", wantErr: "reserved segment"},
		{name: "reserved _ws", rootURL: "/_ws", wantErr: "reserved segment"},
		{name: "reserved print", rootURL: "/print", wantErr: "reserved segment"},
		// Malformed.
		{name: "relative", rootURL: "barbershop", wantErr: `must start with "/"`},
		// Uniqueness still enforced (after trailing-slash normalization).
		{name: "duplicate", rootURL: "/dup", wantErr: "already used"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifests := []manifest.RawManifest{appManifest("a", tc.rootURL, tc.access)}
			if tc.name == "duplicate" {
				manifests = append(manifests, appManifest("b", "/dup", ""))
			}
			_, err := Resolve(manifests, nil)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("root_url %q: expected OK, got: %v", tc.rootURL, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("root_url %q: expected error containing %q, got: %v", tc.rootURL, tc.wantErr, err)
			}
		})
	}
}

func TestResolve_RootURLNormalized(t *testing.T) {
	resolved, err := Resolve([]manifest.RawManifest{appManifest("a", "/barbershop/", "")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolved["a"].Spec.RootURL; got != "/barbershop" {
		t.Fatalf("expected trailing slash stripped, got %q", got)
	}
}
