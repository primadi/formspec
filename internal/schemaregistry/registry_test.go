package schemaregistry

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"formspec.dev/v1", "v1", false},
		{"formspec.dev/v2", "v2", false},
		{"formspec.dev/v10", "v10", false},
		{"", "", true},
		{"formspec.dev/v1alpha1", "", true},
		{"formspec.dev", "", true},
		{"acme.dev/v1", "", true},
		{"v1", "", true},
	}
	for _, c := range cases {
		got, err := ParseVersion(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseVersion(%q): want error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseVersion(%q): got %q, want %q", c.in, got, c.want)
		}
	}
}

// newTestServer serves a minimal registry layout for a version.
func newTestServer(t *testing.T) (*httptest.Server, *Client) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/formspec.schema.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"$schema":"http://json-schema.org/draft-07/schema#","$defs":{}}`))
	})
	mux.HandleFunc("/v1/kinds/Entity.schema.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"type":"object"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := New(srv.URL)
	c.CacheRoot = t.TempDir()
	return srv, c
}

func TestEnsureFetchesAndCaches(t *testing.T) {
	_, c := newTestServer(t)
	if err := c.Ensure("v1", []string{"Entity"}, false); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	dir, err := c.VersionDir("v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"formspec.schema.json", "kinds/Entity.schema.json"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected %s in cache: %v", rel, err)
		}
	}
}

func TestEnsureSkipsWhenComplete(t *testing.T) {
	srv, c := newTestServer(t)
	dir, _ := c.VersionDir("v1")
	if err := os.MkdirAll(filepath.Join(dir, "kinds"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-seed the cache with sentinel content.
	if err := os.WriteFile(filepath.Join(dir, "formspec.schema.json"), []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kinds", "Entity.schema.json"), []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv.Close() // network gone: Ensure must not fetch when complete.
	if err := c.Ensure("v1", []string{"Entity"}, false); err != nil {
		t.Fatalf("Ensure without network should succeed from cache: %v", err)
	}
}

func TestEnsureFetchesWhenForce(t *testing.T) {
	srv, c := newTestServer(t)
	if err := c.Ensure("v1", []string{"Entity"}, true); err != nil {
		t.Fatalf("Ensure(force): %v", err)
	}
	_ = srv
}

func TestEnsureErrorOnMissingVersion(t *testing.T) {
	_, c := newTestServer(t) // server only serves v1
	err := c.Ensure("v9", []string{"Entity"}, false)
	if err == nil {
		t.Fatal("expected error fetching unknown version")
	}
	if !strings.Contains(err.Error(), "unexpected status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListAndClear(t *testing.T) {
	_, c := newTestServer(t)
	if err := c.Ensure("v1", []string{"Entity"}, false); err != nil {
		t.Fatal(err)
	}
	dir, _ := c.VersionDir("v2")
	if err := os.MkdirAll(filepath.Join(dir, "kinds"), 0o755); err != nil {
		t.Fatal(err)
	}
	versions, err := c.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0] != "v1" || versions[1] != "v2" {
		t.Errorf("List: got %v, want [v1 v2]", versions)
	}
	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	versions, err = c.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 0 {
		t.Errorf("after Clear, got %v, want empty", versions)
	}
}
