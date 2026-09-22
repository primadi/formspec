package spec

import (
	"regexp"
	"testing"
)

// TestAPIVersionIsStable pins the value of the exported APIVersion constant to
// the shape the loader accepts. It used to read "formspec.dev/v1alpha1" — a
// value `internal/schemaregistry.ParseVersion` explicitly rejects — so any
// consumer wiring the constant into a manifest produced one that would not
// validate (gap #20).
func TestAPIVersionIsStable(t *testing.T) {
	stable := regexp.MustCompile(`^formspec\.dev/v[0-9]+$`)
	if !stable.MatchString(APIVersion) {
		t.Errorf("APIVersion = %q, want a stable version matching `formspec.dev/v<digits>` "+
			"(pre-stable versions such as v1alpha1 are rejected by the loader)", APIVersion)
	}
}
