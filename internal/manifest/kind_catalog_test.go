package manifest

import (
	"sort"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestIsValidKind_MatchesKnownKinds pins pkg/spec.IsValidKind to the catalog the
// engine actually gates on. It was a dead-code copy that had silently drifted:
// `Workspace` was accepted by this loader (KindWorkspace exists and workspaces
// load fine) but IsValidKind said no. Any kind added to KnownKinds without
// updating pkg/spec now fails here.
func TestIsValidKind_MatchesKnownKinds(t *testing.T) {
	for name := range KnownKinds {
		if !spec.IsValidKind(spec.Kind(name)) {
			t.Errorf("KnownKinds has %q but spec.IsValidKind(%q) = false", name, name)
		}
	}

	declared := spec.AllKinds()
	if len(declared) != len(KnownKinds) {
		t.Errorf("kind count mismatch: spec.AllKinds() has %d (%v), KnownKinds has %d",
			len(declared), declared, len(KnownKinds))
	}
	for _, name := range declared {
		if !KnownKinds[name] {
			t.Errorf("spec.AllKinds() declares %q but internal/manifest.KnownKinds does not accept it", name)
		}
	}

	// Every catalog entry must have an AllKinds counterpart too — otherwise a
	// kind would be loadable but invisible to the parity check above.
	declaredSet := make(map[string]bool, len(declared))
	for _, n := range declared {
		declaredSet[n] = true
	}
	var missing []string
	for name := range KnownKinds {
		if !declaredSet[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("KnownKinds entries absent from spec.AllKinds(): %v", missing)
	}
}

// TestKnownKinds_ContainsWorkspace is the regression this fixes: the platform
// Workspace kind loads through the engine but was missing from pkg/spec's
// IsValidKind.
func TestKnownKinds_ContainsWorkspace(t *testing.T) {
	if !KnownKinds["Workspace"] {
		t.Fatal("loader must accept kind: Workspace")
	}
	if !spec.IsValidKind(spec.KindWorkspace) {
		t.Error("spec.IsValidKind(KindWorkspace) = false, want true")
	}
}
