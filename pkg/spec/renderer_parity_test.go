package spec

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"testing"
)

// The renderer keeps a hand-written mirror of these Go contracts at
// renderers/react-shadcn/src/types/manifest.ts. That file used to carry a
// stale `API_VERSION` ("formspec.dev/v1alpha1") and an incomplete kind list
// (no Calendar/ApprovalInbox/NotificationCenter; a phantom `KIND_MIGRATION`
// with no Go counterpart; and Workspace missing from pkg/spec's own
// IsValidKind). These tests cross-check the mirror against the canonical Go
// source so the drift is caught here instead of at runtime.
//
// Same-artifact pins (a closed-set list in a *_test.go) live next to the data
// they pin — the AllKinds pins are in pkg/spec/widget_test.go's neighbours;
// this file is specifically for the *cross-language* copy.

const tsManifestPath = "renderers/react-shadcn/src/types/manifest.ts"

// readTSManifest locates the renderer's manifest.ts from the repo root, which
// is two levels up from pkg/spec.
func readTSManifest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// pkg/spec/<this file> → repo root is ../..
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	path := filepath.Join(root, tsManifestPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("renderer manifest not available: %v", err)
	}
	return string(data)
}

var tsAPIVersionRe = regexp.MustCompile(`export const API_VERSION = "([^"]+)"`)

// TestTSManifest_APIVersionMatchesGo pins the renderer's API_VERSION constant to
// spec.APIVersion. A stale version here is worse than a stale comment: code
// comparing against it would reject manifests the server happily accepts.
func TestTSManifest_APIVersionMatchesGo(t *testing.T) {
	ts := readTSManifest(t)

	m := tsAPIVersionRe.FindStringSubmatch(ts)
	if m == nil {
		t.Fatalf("could not find `export const API_VERSION = \"...\"` in %s", tsManifestPath)
	}
	if m[1] != APIVersion {
		t.Errorf("%s declares API_VERSION = %q, want %q (pkg/spec.APIVersion)", tsManifestPath, m[1], APIVersion)
	}
}

var tsKindRe = regexp.MustCompile(`export const KIND_[A-Z_]+ = "([A-Za-z]+)"`)

// TestTSManifest_KindsCoverGoCatalog checks the renderer's kind constants
// against spec.AllKinds(), both directions: a Go kind the renderer cannot name
// is a rendering gap, and a TS kind with no Go counterpart is a phantom a
// manifest can never legitimately carry.
func TestTSManifest_KindsCoverGoCatalog(t *testing.T) {
	ts := readTSManifest(t)

	tsKinds := make(map[string]bool)
	for _, m := range tsKindRe.FindAllStringSubmatch(ts, -1) {
		tsKinds[m[1]] = true
	}
	if len(tsKinds) == 0 {
		t.Fatalf("no `export const KIND_*` constants found in %s", tsManifestPath)
	}

	goKinds := make(map[string]bool)
	for _, k := range AllKinds() {
		goKinds[k] = true
	}

	var missing, phantom []string
	for k := range goKinds {
		if !tsKinds[k] {
			missing = append(missing, k)
		}
	}
	for k := range tsKinds {
		if !goKinds[k] {
			phantom = append(phantom, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(phantom)

	if len(missing) > 0 {
		t.Errorf("%s is missing kinds declared by spec.AllKinds(): %v", tsManifestPath, missing)
	}
	if len(phantom) > 0 {
		t.Errorf("%s declares kinds with no pkg/spec counterpart: %v", tsManifestPath, phantom)
	}
}
