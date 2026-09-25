package formspec

import (
	"path/filepath"
	"testing"
)

// The state directory holds the filesystem storage fallback and the persisted
// dev JWT secret. `StateDirFromDSN` only understands SQLite DSNs and returns a
// bare ".formspec" otherwise, which the process then resolves against its CWD —
// so running `formspec dev` from the repo root vs. from `examples/kafe` pointed
// the storage and the secret at two different directories (gap 10.17).
func TestStateDirFor_AnchorsToProjectRoot(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "spec")

	// A non-SQLite DSN has no path to derive a directory from, so the fallback
	// must land under the project root — not under the caller's CWD.
	got := StateDirFor("postgres://user:pw@localhost:5432/db", specPath)
	want := filepath.Join(root, ".formspec")
	if got != want {
		t.Errorf("postgres DSN: got %q, want %q (CWD-relative would be .formspec)", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("anchored state dir must be absolute, got %q", got)
	}

	// The SQLite DSN arrives already anchored (cmd/formspec resolveDSN keeps it
	// absolute). Anchoring again would produce .../<root>/<root>/.formspec.
	anchoredDSN := "sqlite:" + filepath.Join(root, ".formspec", "data.db")
	got = StateDirFor(anchoredDSN, specPath)
	if want := filepath.Join(root, ".formspec"); got != want {
		t.Errorf("anchored sqlite DSN: got %q, want %q (double-anchoring?)", got, want)
	}
}

func TestStateDirFor_LeavesAbsoluteAndEmptyAlone(t *testing.T) {
	// An absolute DSN path is already deterministic.
	abs := filepath.Join(t.TempDir(), ".formspec")
	got := StateDirFor("sqlite:"+filepath.Join(abs, "data.db"), "/some/spec")
	if got != abs {
		t.Errorf("absolute sqlite DSN: got %q, want %q", got, abs)
	}

	// No spec path to anchor to: return the historical value rather than
	// inventing a directory, so callers without a spec keep working.
	if got := StateDirFor("postgres://h/db", ""); got != ".formspec" {
		t.Errorf("no spec path: got %q, want the bare default", got)
	}
}
