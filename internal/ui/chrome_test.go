package ui

import (
	"reflect"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// resolveChrome applies the archetype default matrix (frontend/05-app-kinds.md
// §4.1) plus the resolved region map (§4.2); these tests pin the matrix and
// the override behavior.

// chromeEqual compares the scalar chrome fields — the resolved Regions map is
// asserted separately (a struct with a map field cannot use ==).
func chromeEqual(got, want ChromeConfig) bool {
	got.Regions = nil
	want.Regions = nil
	return reflect.DeepEqual(got, want)
}

func TestResolveChrome_DefaultsSidebarNav(t *testing.T) {
	c := resolveChrome("sidebar-nav", nil)
	want := ChromeConfig{
		Brand: spec.ChromeShow, Nav: spec.ChromeMenu, Auth: spec.ChromeLinks,
		Footer: spec.ChromeHide, Breadcrumbs: spec.ChromeShow, ThemeSwitcher: spec.ChromeShow,
	}
	if !chromeEqual(*c, want) {
		t.Errorf("sidebar-nav defaults mismatch:\n got %+v\nwant %+v", *c, want)
	}
	// Preset (§4.2): sidebar-nav is no-nav plus a filled sidebar; the topbar
	// fill carries the menu, the footer region is off.
	if c.Regions["sidebar"] != spec.ChromeAuto || c.Regions["topbar"] != spec.ChromeAuto {
		t.Errorf("sidebar-nav must preset sidebar+topbar, got %+v", c.Regions)
	}
	if c.Regions["footer"] != spec.ChromeNone || c.Regions["rightbar"] != spec.ChromeNone ||
		c.Regions["bottombar"] != spec.ChromeNone {
		t.Errorf("sidebar-nav must leave rightbar/bottombar/footer empty, got %+v", c.Regions)
	}
}

func TestResolveChrome_DefaultsTopNav(t *testing.T) {
	c := resolveChrome("topnav", nil)
	if c.Nav != spec.ChromeMenu || c.Auth != spec.ChromeLinks {
		t.Errorf("topnav defaults mismatch: %+v", c)
	}
	// Preset: a filled topbar, no sidebar.
	if c.Regions["topbar"] != spec.ChromeAuto || c.Regions["sidebar"] != spec.ChromeNone {
		t.Errorf("topnav must preset topbar only, got %+v", c.Regions)
	}
}

func TestResolveChrome_DefaultsNoNav(t *testing.T) {
	// no-nav = truly no navigation: no nav links, no auth UI.
	c := resolveChrome("no-nav", nil)
	want := ChromeConfig{
		Brand: spec.ChromeShow, Nav: spec.ChromeNone, Auth: spec.ChromeNone,
		Footer: spec.ChromeShow, Breadcrumbs: spec.ChromeHide, ThemeSwitcher: spec.ChromeHide,
	}
	if !chromeEqual(*c, want) {
		t.Errorf("no-nav defaults mismatch:\n got %+v\nwant %+v", *c, want)
	}
	// Preset (§4.2): chrome EXISTS (a minimal brand bar in the topbar region)
	// but no navigation/sidebar. This is the whole point of §4.2 — no-nav is
	// not "no chrome".
	if c.Regions["sidebar"] != spec.ChromeNone || c.Regions["rightbar"] != spec.ChromeNone ||
		c.Regions["bottombar"] != spec.ChromeNone {
		t.Errorf("no-nav bar regions must be empty, got %+v", c.Regions)
	}
	if c.Regions["topbar"] != spec.ChromeAuto {
		t.Errorf("no-nav must keep a minimal brand topbar, got %+v", c.Regions)
	}
	if c.Regions["footer"] != spec.ChromeAuto {
		t.Errorf("no-nav footer region must be filled, got %+v", c.Regions)
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

func TestResolveChrome_RegionOverrides(t *testing.T) {
	// no-nav + explicitly filled topbar (the "no-nav but chrome exists"
	// scenario §4.2 enables): region content wins, other regions stay empty.
	c := resolveChrome("no-nav", &spec.AppChrome{Regions: map[string]string{"topbar": spec.ChromeAuto}})
	if c.Regions["topbar"] != spec.ChromeAuto {
		t.Errorf("explicit regions.topbar must override the preset, got %+v", c.Regions)
	}
	if c.Regions["sidebar"] != spec.ChromeNone {
		t.Errorf("untouched regions keep the preset, got %+v", c.Regions)
	}

	// A component ref is passed through verbatim.
	c = resolveChrome("sidebar-nav", &spec.AppChrome{Regions: map[string]string{"rightbar": "cafe/components/help"}})
	if c.Regions["rightbar"] != "cafe/components/help" {
		t.Errorf("component ref must pass through, got %+v", c.Regions)
	}

	// `regions.footer` is the region source of truth and mirrors the legacy
	// boolean, so the two can never disagree.
	c = resolveChrome("sidebar-nav", &spec.AppChrome{Footer: spec.ChromeShow})
	if c.Regions["footer"] != spec.ChromeAuto || c.Footer != spec.ChromeShow {
		t.Errorf("legacy footer:show must fill the footer region, got %q / %+v", c.Footer, c.Regions)
	}
	c = resolveChrome("no-nav", &spec.AppChrome{Footer: spec.ChromeHide})
	if c.Regions["footer"] != spec.ChromeNone || c.Footer != spec.ChromeHide {
		t.Errorf("legacy footer:hide must empty the footer region, got %q / %+v", c.Footer, c.Regions)
	}
	// Explicit regions.footer wins over the legacy boolean.
	c = resolveChrome("no-nav", &spec.AppChrome{Footer: spec.ChromeShow, Regions: map[string]string{"footer": spec.ChromeNone}})
	if c.Regions["footer"] != spec.ChromeNone || c.Footer != spec.ChromeHide {
		t.Errorf("explicit regions.footer must win, got %q / %+v", c.Footer, c.Regions)
	}
}

func TestResolveChrome_UnknownValuesFallBackToDefault(t *testing.T) {
	// Lenient at resolve time — strict validation happens at manifest load.
	c := resolveChrome("no-nav", &spec.AppChrome{Nav: "sidebar", Auth: "oauth"})
	if c.Nav != spec.ChromeNone || c.Auth != spec.ChromeNone {
		t.Errorf("unknown chrome values must fall back to archetype defaults, got %+v", c)
	}
	// Unknown region keys / empty values are ignored — never invented.
	c = resolveChrome("no-nav", &spec.AppChrome{Regions: map[string]string{"center": spec.ChromeAuto, "sidebar": ""}})
	if _, ok := c.Regions["center"]; ok {
		t.Errorf("unknown region key must not reach the bundle, got %+v", c.Regions)
	}
	if c.Regions["sidebar"] != spec.ChromeNone {
		t.Errorf("empty region value must keep the preset, got %+v", c.Regions)
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
