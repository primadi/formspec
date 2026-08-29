package ui

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// resolveChrome applies the archetype default matrix (frontend/05-app-kinds.md
// §4.1); these tests pin the matrix and the override behavior.

func TestResolveChrome_DefaultsSidebarNav(t *testing.T) {
	c := resolveChrome("sidebar-nav", nil)
	want := ChromeConfig{
		Brand: spec.ChromeShow, Nav: spec.ChromeMenu, Auth: spec.ChromeLinks,
		Footer: spec.ChromeHide, Breadcrumbs: spec.ChromeShow, ThemeSwitcher: spec.ChromeShow,
	}
	if *c != want {
		t.Errorf("sidebar-nav defaults mismatch:\n got %+v\nwant %+v", *c, want)
	}
}

func TestResolveChrome_DefaultsTopNav(t *testing.T) {
	c := resolveChrome("topnav", nil)
	if c.Nav != spec.ChromeMenu || c.Auth != spec.ChromeLinks {
		t.Errorf("topnav defaults mismatch: %+v", c)
	}
}

func TestResolveChrome_DefaultsNoNav(t *testing.T) {
	// no-nav = truly no navigation: no nav links, no auth UI.
	c := resolveChrome("no-nav", nil)
	want := ChromeConfig{
		Brand: spec.ChromeShow, Nav: spec.ChromeNone, Auth: spec.ChromeNone,
		Footer: spec.ChromeShow, Breadcrumbs: spec.ChromeHide, ThemeSwitcher: spec.ChromeHide,
	}
	if *c != want {
		t.Errorf("no-nav defaults mismatch:\n got %+v\nwant %+v", *c, want)
	}
}

func TestResolveChrome_Overrides(t *testing.T) {
	// Registry scenario: no-nav + public catalog that opts back into nav
	// links and Sign in/Sign up controls.
	c := resolveChrome("no-nav", &spec.AppChrome{Nav: spec.ChromeMenu, Auth: spec.ChromeLinks})
	if c.Nav != spec.ChromeMenu || c.Auth != spec.ChromeLinks {
		t.Errorf("expected nav=menu auth=links, got %+v", c)
	}
	// Other elements keep the no-nav defaults.
	if c.Footer != spec.ChromeShow || c.Breadcrumbs != spec.ChromeHide {
		t.Errorf("non-overridden elements must keep archetype defaults, got %+v", c)
	}
}

func TestResolveChrome_UnknownValuesFallBackToDefault(t *testing.T) {
	// Lenient at resolve time — strict validation happens at manifest load.
	c := resolveChrome("no-nav", &spec.AppChrome{Nav: "sidebar", Auth: "oauth"})
	if c.Nav != spec.ChromeNone || c.Auth != spec.ChromeNone {
		t.Errorf("unknown chrome values must fall back to archetype defaults, got %+v", c)
	}
}

func TestBuildBundle_ShipsResolvedChrome(t *testing.T) {
	r := NewRegistry()
	b := r.BuildBundle(func() []EntityDescriptor { return nil }, func(string) bool { return true }, AppContext{
		Name: "storefront", RootURL: "/", AppRenderer: "no-nav", Access: "public",
	})
	if b.App.Chrome == nil {
		t.Fatal("bundle.App.Chrome must always be present (never nil)")
	}
	if b.App.Chrome.Nav != spec.ChromeNone || b.App.Chrome.Auth != spec.ChromeNone {
		t.Errorf("no-nav App must ship nav=none auth=none, got %+v", b.App.Chrome)
	}
}
