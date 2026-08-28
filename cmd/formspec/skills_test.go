package main

import (
	"strings"
	"testing"
)

// TestRelevantSkillsFor proves the deterministic skill re-check in
// propose_spec_file (todo 10.6.4, docs/ai/06 §3): skills whose
// applies_to_kind covers the draft kind are returned; skills with an empty
// applies_to_kind always apply.
func TestRelevantSkillsFor(t *testing.T) {
	entityDraft := "apiVersion: formspec.dev/v1\nkind: Entity\nmetadata:\n  name: x\n  module: m\nspec: {}\n"
	formDraft := "apiVersion: formspec.dev/v1\nkind: Form\nmetadata:\n  name: x\n  module: m\nspec: {}\n"

	entitySkills := relevantSkillsFor(entityDraft)
	names := map[string]bool{}
	for _, s := range entitySkills {
		names[s.Name] = true
	}
	if !names["entity-authoring"] {
		t.Errorf("entity draft should surface entity-authoring, got %v", names)
	}
	if !names["entity-extension-authoring"] {
		t.Errorf("entity draft should surface entity-extension-authoring, got %v", names)
	}
	if names["form-layout"] {
		t.Errorf("entity draft must not surface form-layout, got %v", names)
	}

	formSkills := relevantSkillsFor(formDraft)
	found := false
	for _, s := range formSkills {
		if s.Name == "form-layout" {
			found = true
		}
	}
	if !found {
		t.Error("form draft should surface form-layout")
	}

	// Skills with empty applies_to_kind (e.g. formspec-app-workflow) always
	// appear regardless of kind.
	any := relevantSkillsFor("kind: Whatever\n")
	if len(any) == 0 {
		t.Error("empty applies_to_kind skills must always apply")
	}
}

// TestSkillFrontmatterFields proves the 06 §2 fields are parsed
// (applies_to_kind, min_core_spec_version).
func TestSkillFrontmatterFields(t *testing.T) {
	metas, _, err := embeddedSkills()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]skillMeta{}
	for _, m := range metas {
		byName[m.Name] = m
	}
	for _, want := range []string{"entity-authoring", "form-layout", "entity-extension-authoring", "module-vendoring"} {
		m, ok := byName[want]
		if !ok {
			t.Errorf("skill %q missing from embedded catalog", want)
			continue
		}
		if len(m.AppliesTo) == 0 {
			t.Errorf("skill %q missing applies_to_kind", want)
		}
		if m.MinCoreVer == "" {
			t.Errorf("skill %q missing min_core_spec_version", want)
		}
	}
	if m := byName["entity-authoring"]; !containsFold(m.AppliesTo, "Entity") {
		t.Errorf("entity-authoring applies_to_kind = %v, want Entity", m.AppliesTo)
	}
}

func containsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}
