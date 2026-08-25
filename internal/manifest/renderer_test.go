package manifest

import (
	"strings"
	"testing"
)

// TestRendererRegistry_Resolution verifies 5.16.1: only official renderers
// auto-select; without official → error with candidate suggestions.
func TestRendererRegistry_Resolution(t *testing.T) {
	manifests := []RawManifest{
		{Kind: "Renderer", Metadata: RawMetadata{Name: "form-page", Module: "formspec"}, Spec: map[string]any{
			"implements": "formspec/visual.form-page", "stack_family": "react-shadcn", "trust_tier": "official",
		}},
		{Kind: "Renderer", Metadata: RawMetadata{Name: "super-kanban", Module: "community"}, Spec: map[string]any{
			"implements": "kanban", "stack_family": "react-shadcn", "trust_tier": "community",
		}},
	}
	reg := NewRendererRegistry(manifests)

	// Official renderer auto-selects.
	name, err := reg.ResolveRenderer("formspec/visual.form-page", "react-shadcn", "")
	if err != nil || name != "formspec/visual.form-page" {
		t.Fatalf("official auto-select failed: name=%q err=%v", name, err)
	}

	// No official → error with candidate suggestion.
	_, err = reg.ResolveRenderer("kanban", "react-shadcn", "")
	if err == nil {
		t.Fatal("expected error when no official renderer")
	}
	if !strings.Contains(err.Error(), "community") {
		t.Fatalf("expected candidate suggestion, got: %v", err)
	}

	// Explicit override wins.
	name, err = reg.ResolveRenderer("kanban", "react-shadcn", "community/super-kanban")
	if err != nil || name != "community/super-kanban" {
		t.Fatalf("explicit override failed: name=%q err=%v", name, err)
	}

	// No renderer at all → error.
	_, err = reg.ResolveRenderer("unknown-kind", "react-shadcn", "")
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

// TestRendererRegistry_SlotTiers verifies 5.16.2: accepts_slots only on
// tier page|app; implements_slot only on tier component.
func TestRendererRegistry_SlotTiers(t *testing.T) {
	manifests := []RawManifest{
		{Kind: "VisualSpecKind", Metadata: RawMetadata{Name: "bad-page", Module: "m"}, Spec: map[string]any{
			"tier": "component", "accepts_slots": []any{map[string]any{"name": "x"}},
		}},
		{Kind: "VisualSpecKind", Metadata: RawMetadata{Name: "bad-component", Module: "m"}, Spec: map[string]any{
			"tier": "page", "implements_slot": "widget",
		}},
		{Kind: "VisualSpecKind", Metadata: RawMetadata{Name: "good-page", Module: "m"}, Spec: map[string]any{
			"tier": "page", "accepts_slots": []any{map[string]any{"name": "x"}},
		}},
		{Kind: "VisualSpecKind", Metadata: RawMetadata{Name: "good-component", Module: "m"}, Spec: map[string]any{
			"tier": "component", "implements_slot": "widget",
		}},
	}
	reg := NewRendererRegistry(manifests)
	errs := reg.ValidateSlotTiers()
	if len(errs) != 2 {
		t.Fatalf("expected 2 slot-tier errors, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "bad-page") || !strings.Contains(errs[1], "bad-component") {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

// TestRendererRegistry_ResolutionValidation verifies App renderers: map and
// Page renderer: field resolve to registered renderers (5.16.1).
func TestRendererRegistry_ResolutionValidation(t *testing.T) {
	manifests := []RawManifest{
		{Kind: "Renderer", Metadata: RawMetadata{Name: "form-page", Module: "formspec"}, Spec: map[string]any{
			"implements": "formspec/visual.form-page", "stack_family": "react-shadcn", "trust_tier": "official",
		}},
		{Kind: "App", Metadata: RawMetadata{Name: "shop", Module: "m"}, Spec: map[string]any{
			"renderers": map[string]any{"kanban": "community/super-kanban"},
		}},
		{Kind: "Page", Metadata: RawMetadata{Name: "orders", Module: "m"}, Spec: map[string]any{
			"route": "/orders", "renderer": "community/super-kanban",
		}},
	}
	reg := NewRendererRegistry(manifests)
	var apps, pages []RawManifest
	for _, m := range manifests {
		if m.Kind == "App" {
			apps = append(apps, m)
		}
		if m.Kind == "Page" {
			pages = append(pages, m)
		}
	}
	errs := reg.ValidateRendererResolution(apps, pages)
	// Both reference "community/super-kanban" which is NOT registered → 2 errors.
	if len(errs) != 2 {
		t.Fatalf("expected 2 resolution errors, got %d: %v", len(errs), errs)
	}
}

// TestRendererRegistry_StackFamily verifies 5.16.3: shell-integrated Page
// renderer must match the App stack_family.
func TestRendererRegistry_StackFamily(t *testing.T) {
	manifests := []RawManifest{
		{Kind: "App", Metadata: RawMetadata{Name: "shop", Module: "m"}, Spec: map[string]any{
			"stack_family": "react-shadcn",
		}},
		{Kind: "Page", Metadata: RawMetadata{Name: "orders", Module: "m"}, Spec: map[string]any{
			"route": "/orders", "renderer": "vue/orders-page",
		}},
	}
	reg := NewRendererRegistry(manifests)
	var apps, pages []RawManifest
	for _, m := range manifests {
		if m.Kind == "App" {
			apps = append(apps, m)
		}
		if m.Kind == "Page" {
			pages = append(pages, m)
		}
	}
	errs := reg.ValidateStackFamily(apps, pages)
	if len(errs) != 1 {
		t.Fatalf("expected 1 stack_family error, got %d: %v", len(errs), errs)
	}
}
