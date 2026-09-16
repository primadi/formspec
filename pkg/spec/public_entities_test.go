package spec

import (
	"strings"
	"testing"
)

// public_entities validation (S3). The allowlist decides what anonymous callers
// can reach, so a malformed grant must fail loudly rather than silently widen
// or silently grant nothing.

func publicApp(decls ...PublicEntityDecl) *AppSpec {
	return &AppSpec{
		RootURL:        "/",
		Access:         AppAccessPublic,
		Modules:        []string{"cafe-master", "cafe-order"},
		PublicEntities: &decls,
	}
}

func TestValidateAppSpec_PublicEntities_Valid(t *testing.T) {
	// Both reference spellings are accepted: dotted (relation.resource style)
	// and slashed (Page ref style).
	a := publicApp(
		PublicEntityDecl{Entity: "cafe-master.menu-item", Actions: []string{"list", "find"}},
		PublicEntityDecl{Entity: "cafe-order/order", Actions: []string{"create"}},
	)
	if err := ValidateAppSpec(a); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateAppSpec_PublicEntities_ExplicitlyEmptyAllowed(t *testing.T) {
	if err := ValidateAppSpec(publicApp()); err != nil {
		t.Errorf("`public_entities: []` (grant nothing) must be valid, got %v", err)
	}
}

func TestValidateAppSpec_PublicEntities_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		app     *AppSpec
		wantSub string
	}{
		{
			name:    "private access",
			app:     &AppSpec{RootURL: "/app", Modules: []string{"m"}, PublicEntities: &[]PublicEntityDecl{{Entity: "m/e", Actions: []string{"list"}}}},
			wantSub: "requires `access: public`",
		},
		{
			name:    "module not mounted",
			app:     publicApp(PublicEntityDecl{Entity: "other-module/thing", Actions: []string{"list"}}),
			wantSub: "is not mounted by this App",
		},
		{
			name:    "unknown action",
			app:     publicApp(PublicEntityDecl{Entity: "cafe-master.menu-item", Actions: []string{"purge"}}),
			wantSub: "unknown action",
		},
		{
			name:    "empty actions",
			app:     publicApp(PublicEntityDecl{Entity: "cafe-master.menu-item"}),
			wantSub: "actions is required",
		},
		{
			name:    "malformed ref",
			app:     publicApp(PublicEntityDecl{Entity: "menu-item", Actions: []string{"list"}}),
			wantSub: "must be",
		},
		{
			name: "duplicate",
			app: publicApp(
				PublicEntityDecl{Entity: "cafe-master.menu-item", Actions: []string{"list"}},
				PublicEntityDecl{Entity: "cafe-master/menu-item", Actions: []string{"find"}},
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

// TestValidateAppSpec_PublicEntities_Scope pins the per-surface row scope on a
// public grant (#45): it exists to make "anonymous may list" mean "may list the
// row carrying the token they present". Two shapes are refused because they
// would look guarded without being it.
func TestValidateAppSpec_PublicEntities_Scope(t *testing.T) {
	ok := publicApp(PublicEntityDecl{
		Entity:  "cafe-order/order",
		Actions: []string{"create", "list"},
		Scope:   []FilterSpec{{Field: "guest_token", Op: "eq", From: "route", Param: "token"}},
	})
	if err := ValidateAppSpec(ok); err != nil {
		t.Errorf("token-scoped public list: expected no error, got %v", err)
	}

	rejects := []struct {
		name    string
		decl    PublicEntityDecl
		wantSub string
	}{
		{
			name: "session scope on a public surface",
			decl: PublicEntityDecl{
				Entity: "cafe-order/order", Actions: []string{"list"},
				Scope: []FilterSpec{{Field: "branch_id", From: "session"}},
			},
			wantSub: "from: route",
		},
		{
			name: "scope without a field",
			decl: PublicEntityDecl{
				Entity: "cafe-order/order", Actions: []string{"list"},
				Scope: []FilterSpec{{From: "route", Param: "token"}},
			},
			wantSub: "field is required",
		},
		{
			// find resolves by id; a scope cannot guard it, so granting both
			// would read as filtered while any id still returns its record.
			name: "find granted together with a scope",
			decl: PublicEntityDecl{
				Entity: "cafe-order/order", Actions: []string{"find", "list"},
				Scope: []FilterSpec{{Field: "guest_token", From: "route", Param: "token"}},
			},
			wantSub: "cannot grant `find` together with `scope`",
		},
	}
	for _, c := range rejects {
		err := ValidateAppSpec(publicApp(c.decl))
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("%s: error %q does not contain %q", c.name, err.Error(), c.wantSub)
		}
	}
}

// TestNormalizeEntityRef pins the ref forms shared by public_entities and the
// router lookup. A dotted module name must not be split at the wrong dot.
func TestNormalizeEntityRef(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"cafe-master.menu-item", "cafe-master/menu-item", true},
		{"cafe-master/menu-item", "cafe-master/menu-item", true},
		{"formspec.core.workspace", "formspec.core/workspace", true},
		{"formspec.core/workspace", "formspec.core/workspace", true},
		{"menu-item", "", false},
		{"", "", false},
		{"/entity", "", false},
		{"module/", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeEntityRef(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeEntityRef(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
