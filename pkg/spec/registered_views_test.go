package spec

import (
	"strings"
	"testing"
)

// registered_views validation (S4). The field curates an App's reachable
// surface, so a malformed entry must fail loudly rather than silently grant
// nothing (a view that never resolves) — see plan registered-views.md.

func registeredApp(decls ...RegisteredViewDecl) *AppSpec {
	return &AppSpec{
		RootURL:         "/",
		Modules:         []string{"cafe-master", "cafe-order"},
		RegisteredViews: decls,
	}
}

func TestValidateAppSpec_RegisteredViews_Valid(t *testing.T) {
	// Both reference spellings are accepted for `entity`, and `view` accepts
	// one navigable name per entry.
	a := registeredApp(
		RegisteredViewDecl{Entity: "cafe-master.menu-item"},
		RegisteredViewDecl{Entity: "cafe-order/order"},
		RegisteredViewDecl{View: "cafe-order/menu-catalog"},
	)
	if err := ValidateAppSpec(a); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateAppSpec_RegisteredViews_AbsentAndEmptyAllowed(t *testing.T) {
	if err := ValidateAppSpec(&AppSpec{RootURL: "/", Modules: []string{"m"}}); err != nil {
		t.Errorf("absent registered_views must be valid, got %v", err)
	}
	if err := ValidateAppSpec(registeredApp()); err != nil {
		t.Errorf("`registered_views: []` must be valid, got %v", err)
	}
}

func TestValidateAppSpec_RegisteredViews_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		app     *AppSpec
		wantSub string
	}{
		{
			name:    "neither entity nor view",
			app:     registeredApp(RegisteredViewDecl{}),
			wantSub: "exactly one of",
		},
		{
			name:    "both entity and view",
			app:     registeredApp(RegisteredViewDecl{Entity: "cafe-master/menu-item", View: "cafe-order/menu-catalog"}),
			wantSub: "exactly one of",
		},
		{
			name:    "entity module not mounted",
			app:     registeredApp(RegisteredViewDecl{Entity: "other-module/thing"}),
			wantSub: "is not mounted",
		},
		{
			name:    "view module not mounted",
			app:     registeredApp(RegisteredViewDecl{View: "other-module/page"}),
			wantSub: "is not mounted",
		},
		{
			name:    "malformed entity ref",
			app:     registeredApp(RegisteredViewDecl{Entity: "menu-item"}),
			wantSub: "must be",
		},
		{
			name:    "malformed view ref",
			app:     registeredApp(RegisteredViewDecl{View: "menu-catalog"}),
			wantSub: "must be a module/name reference",
		},
		{
			name: "duplicate entity",
			app: registeredApp(
				RegisteredViewDecl{Entity: "cafe-master.menu-item"},
				RegisteredViewDecl{Entity: "cafe-master/menu-item"},
			),
			wantSub: "more than once",
		},
		{
			name: "duplicate view",
			app: registeredApp(
				RegisteredViewDecl{View: "cafe-order/menu-catalog"},
				RegisteredViewDecl{View: "cafe-order/menu-catalog"},
			),
			wantSub: "more than once",
		},
	}
	for _, c := range cases {
		err := ValidateAppSpec(c.app)
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("%s: error %q does not contain %q", c.name, err.Error(), c.wantSub)
		}
	}
}
