package spec

import "testing"

// public_entities used to be a manifest field validated by ValidateAppSpec.
// It is gone (plan docs_internal/plan/implicit-public-grants.md): the allowlist
// is now DERIVED from the App's surface, so there is nothing for the manifest
// validator to check.
//
// What remains here is the ref normalization the derivation and the router
// lookup share. A dotted module name must not be split at the wrong dot.

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
