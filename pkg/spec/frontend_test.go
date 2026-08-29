package spec

import (
	"strings"
	"testing"
)

func TestValidateAppSpec_DefaultRenderer(t *testing.T) {
	// Empty app_renderer is valid (defaults to sidebar-nav at resolve time).
	a := &AppSpec{RootURL: "/app"}
	if err := ValidateAppSpec(a); err != nil {
		t.Errorf("expected no error for empty app_renderer, got %v", err)
	}
}

func TestValidateAppSpec_KnownRenderers(t *testing.T) {
	for _, r := range []string{"sidebar-nav", "topnav", "no-nav"} {
		a := &AppSpec{RootURL: "/app", AppRenderer: r}
		if err := ValidateAppSpec(a); err != nil {
			t.Errorf("expected no error for app_renderer %q, got %v", r, err)
		}
	}
}

func TestValidateAppSpec_UnknownRenderer(t *testing.T) {
	a := &AppSpec{RootURL: "/app", AppRenderer: "hamburger-nav"}
	err := ValidateAppSpec(a)
	if err == nil {
		t.Fatal("expected error for unknown app_renderer")
	}
	if !strings.Contains(err.Error(), "not a known App renderer") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAppSpec_Access(t *testing.T) {
	for _, acc := range []AppAccess{AppAccessPrivate, AppAccessPublic, ""} {
		a := &AppSpec{RootURL: "/app", Access: acc}
		if err := ValidateAppSpec(a); err != nil {
			t.Errorf("expected no error for access %q, got %v", acc, err)
		}
	}
	if err := ValidateAppSpec(&AppSpec{RootURL: "/app", Access: AppAccess("sso")}); err == nil {
		t.Error("expected error for invalid access")
	}
}

func TestValidateAppSpec_PersistBackend(t *testing.T) {
	// Default + installed backend OK.
	if err := ValidateAppSpec(&AppSpec{RootURL: "/app"}); err != nil {
		t.Errorf("expected no error for default persist_backend, got %v", err)
	}
	if err := ValidateAppSpec(&AppSpec{RootURL: "/app", PersistBackend: DefaultPersistBackend}); err != nil {
		t.Errorf("expected no error for installed persist_backend, got %v", err)
	}
	// Uninstalled backend → hard error (incompatible swap).
	err := ValidateAppSpec(&AppSpec{RootURL: "/app", PersistBackend: "postgres-persist"})
	if err == nil {
		t.Fatal("expected error for uninstalled persist_backend")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAppSpec_Chrome(t *testing.T) {
	// Valid values pass.
	valid := &AppSpec{RootURL: "/app", Chrome: &AppChrome{
		Brand: ChromeShow, Nav: ChromeMenu, Auth: ChromeLinks,
		Footer: ChromeHide, Breadcrumbs: ChromeAuto, ThemeSwitcher: ChromeHide,
	}}
	if err := ValidateAppSpec(valid); err != nil {
		t.Errorf("expected no error for valid chrome, got %v", err)
	}
	// Empty chrome / nil chrome pass.
	if err := ValidateAppSpec(&AppSpec{RootURL: "/app", Chrome: &AppChrome{}}); err != nil {
		t.Errorf("expected no error for empty chrome, got %v", err)
	}
	// Invalid values fail with the field name in the message.
	for _, tc := range []struct{ field, val string }{
		{"brand", "maybe"}, {"nav", "sidebar"}, {"auth", "oauth"},
		{"footer", "yes"}, {"breadcrumbs", "true"}, {"theme_switcher", "on"},
	} {
		c := &AppChrome{}
		switch tc.field {
		case "brand":
			c.Brand = tc.val
		case "nav":
			c.Nav = tc.val
		case "auth":
			c.Auth = tc.val
		case "footer":
			c.Footer = tc.val
		case "breadcrumbs":
			c.Breadcrumbs = tc.val
		case "theme_switcher":
			c.ThemeSwitcher = tc.val
		}
		err := ValidateAppSpec(&AppSpec{RootURL: "/app", Chrome: c})
		if err == nil {
			t.Errorf("expected error for chrome.%s = %q", tc.field, tc.val)
			continue
		}
		if !strings.Contains(err.Error(), "chrome."+tc.field) {
			t.Errorf("error for chrome.%s should name the field, got: %v", tc.field, err)
		}
	}
}

func TestValidatePageSpec_BlocksAndTabsMutuallyExclusive(t *testing.T) {
	p := &PageSpec{
		Route:  "/x",
		Title:  "X",
		Blocks: []PageBlock{{Form: &BlockRef{Ref: "f"}}},
		Tabs:   []PageTab{{Label: "T", Form: &BlockRef{Ref: "f"}}},
	}
	err := ValidatePageSpec(p)
	if err == nil {
		t.Fatal("expected error for blocks+tabs")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePageSpec_SectionBlockTypes(t *testing.T) {
	for _, typ := range []string{"hero", "feature_grid", "card", "carousel", "cta"} {
		p := &PageSpec{
			Route:  "/home",
			Title:  "Home",
			Blocks: []PageBlock{{Section: &SectionBlock{Type: typ}}},
		}
		if err := ValidatePageSpec(p); err != nil {
			t.Errorf("expected no error for section type %q, got %v", typ, err)
		}
	}
}

func TestValidatePageSpec_UnknownSectionType(t *testing.T) {
	p := &PageSpec{
		Route:  "/home",
		Title:  "Home",
		Blocks: []PageBlock{{Section: &SectionBlock{Type: "video"}}},
	}
	err := ValidatePageSpec(p)
	if err == nil {
		t.Fatal("expected error for unknown section type")
	}
	if !strings.Contains(err.Error(), "not a known section block") {
		t.Errorf("unexpected error: %v", err)
	}
}
