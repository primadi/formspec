package spec

import "testing"

// TestSurfaceURL pins the workspace-prefixed shape of an App surface URL. The
// bug this guards is not subtle arithmetic but a banner that printed the bare
// origin ("http://localhost:8080") as "where the UI is" — every surface lives
// under the workspace slug (D50), so that URL is always a 404.
func TestSurfaceURL(t *testing.T) {
	cases := []struct {
		name    string
		slug    string
		rootURL string
		want    string
	}{
		{"root app keeps the mount slash", "default", "/", "/default/"},
		{"empty slug falls back to default", "", "/", "/default/"},
		{"empty root_url behaves as root", "kafe", "", "/kafe/"},
		{"free-form mount has no trailing slash", "default", "/barbershop", "/default/barbershop"},
		{"nested mount", "kafe", "/app/pos", "/kafe/app/pos"},
		{"trailing slash on root_url is trimmed", "default", "/barbershop/", "/default/barbershop"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SurfaceURL(tc.slug, tc.rootURL); got != tc.want {
				t.Errorf("SurfaceURL(%q, %q) = %q, want %q", tc.slug, tc.rootURL, got, tc.want)
			}
		})
	}
}
