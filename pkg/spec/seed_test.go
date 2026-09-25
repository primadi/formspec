package spec

import "testing"

// TestValidateSeedSpec covers the manifest-level shape of a kind: Seed — the
// gate that was missing entirely: `formspec seed` ran these files while
// `formspec validate` rejected the kind, so no seed could ever be validated.
func TestValidateSeedSpec(t *testing.T) {
	t.Run("valid seed with two entities", func(t *testing.T) {
		s := &SeedSpec{Entities: []SeedEntity{
			{Entity: "branch", Records: []map[string]any{{"code": "KFE-01"}}},
			{Entity: "menu-item", Records: []map[string]any{{"code": "LAT-001"}}},
		}}
		if err := ValidateSeedSpec(s); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
	})

	t.Run("nil spec is rejected", func(t *testing.T) {
		if err := ValidateSeedSpec(nil); err == nil {
			t.Fatal("expected error for nil spec")
		}
	})

	t.Run("no entities is rejected", func(t *testing.T) {
		if err := ValidateSeedSpec(&SeedSpec{}); err == nil {
			t.Fatal("expected error for empty entities")
		}
	})

	t.Run("entity name is required", func(t *testing.T) {
		s := &SeedSpec{Entities: []SeedEntity{
			{Records: []map[string]any{{"code": "A"}}},
		}}
		if err := ValidateSeedSpec(s); err == nil {
			t.Fatal("expected error for missing entity name")
		}
	})

	t.Run("empty records is rejected", func(t *testing.T) {
		s := &SeedSpec{Entities: []SeedEntity{{Entity: "branch"}}}
		if err := ValidateSeedSpec(s); err == nil {
			t.Fatal("expected error for empty records")
		}
	})

	t.Run("duplicate entity block is rejected", func(t *testing.T) {
		s := &SeedSpec{Entities: []SeedEntity{
			{Entity: "branch", Records: []map[string]any{{"code": "A"}}},
			{Entity: "branch", Records: []map[string]any{{"code": "B"}}},
		}}
		if err := ValidateSeedSpec(s); err == nil {
			t.Fatal("expected error for duplicate entity block")
		}
	})
}

// TestSeedIsRegisteredKind pins the fix for the gap that made every seed file
// fail `formspec validate`: the kind must be in both catalogs, and the two
// catalogs must agree (TestIsValidKind_MatchesKnownKinds pins the pair).
func TestSeedIsRegisteredKind(t *testing.T) {
	if !IsValidKind(KindSeed) {
		t.Fatal("KindSeed must be a valid kind")
	}
	found := false
	for _, k := range AllKinds() {
		if k == string(KindSeed) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("KindSeed must appear in AllKinds()")
	}
}
