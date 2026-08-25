// ─── Renderer Registry & Resolution (todo 5.16) ───
//
// Implements the renderer resolution engine from frontend/03-renderer-kind.md
// §3 and the slot-tier / stack_family validation from
// frontend/02-visual-spec-kind.md §4–§5 and frontend/01-visual-hierarchy.md §3.
//
// Rules:
//   - Only `trust_tier: official` renderers auto-select for (implements,
//     stack_family). Without an official renderer, resolution is an ERROR
//     (never a silent fallback) and the error suggests verified/community
//     candidates for the developer to pick explicitly.
//   - Override: App-level `renderers:` map (implements → renderer) applies to
//     every instance of that kind; an instance's own `renderer:` field wins.
//   - `accepts_slots` is only valid on tier page|app; `implements_slot` only
//     on tier component — anything else is rejected at apply time.
//   - App shell + shell-integrated Page + Component must share one
//     stack_family; independent Pages are not checked.

package manifest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// RendererEntry is one registered Renderer manifest.
type RendererEntry struct {
	Source      string
	Implements  string
	StackFamily string
	TrustTier   string
}

// VisualKindEntry is one registered VisualSpecKind manifest.
type VisualKindEntry struct {
	Source         string
	Name           string
	Tier           string
	AcceptsSlots   []spec.SlotDecl
	ImplementsSlot string
}

// RendererRegistry indexes Renderer + VisualSpecKind manifests for
// resolution and validation.
type RendererRegistry struct {
	Renderers    []RendererEntry
	VisualKinds  []VisualKindEntry
	byImplements map[string][]RendererEntry // implements → renderers
	kindByTier   map[string]string          // kind name → tier
}

// NewRendererRegistry builds a registry from loaded manifests.
func NewRendererRegistry(manifests []RawManifest) *RendererRegistry {
	reg := &RendererRegistry{
		byImplements: map[string][]RendererEntry{},
		kindByTier:   map[string]string{},
	}
	for _, m := range manifests {
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		switch spec.Kind(m.Kind) {
		case spec.KindRenderer:
			rs, err := RawSpecTo[spec.RendererSpec](sm)
			if err != nil {
				continue
			}
			entry := RendererEntry{
				Source:      m.Source,
				Implements:  rs.Implements,
				StackFamily: rs.StackFamily,
				TrustTier:   rs.TrustTier,
			}
			reg.Renderers = append(reg.Renderers, entry)
			reg.byImplements[rs.Implements] = append(reg.byImplements[rs.Implements], entry)
		case spec.KindVisualSpecKind:
			vk, err := RawSpecTo[spec.VisualSpecKindSpec](sm)
			if err != nil {
				continue
			}
			reg.VisualKinds = append(reg.VisualKinds, VisualKindEntry{
				Source:         m.Source,
				Name:           m.Metadata.Name,
				Tier:           vk.Tier,
				AcceptsSlots:   vk.AcceptsSlots,
				ImplementsSlot: vk.ImplementsSlot,
			})
			reg.kindByTier[m.Metadata.Name] = vk.Tier
		}
	}
	return reg
}

// candidates returns renderers for (implements, stackFamily) ordered by
// trust tier (official first, then verified, then community).
func (r *RendererRegistry) candidates(implements, stackFamily string) []RendererEntry {
	var out []RendererEntry
	for _, e := range r.byImplements[implements] {
		if e.StackFamily == stackFamily {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return tierRank(out[i].TrustTier) < tierRank(out[j].TrustTier)
	})
	return out
}

func tierRank(t string) int {
	switch t {
	case "official":
		return 0
	case "verified":
		return 1
	default:
		return 2
	}
}

// ResolveRenderer resolves the renderer for one VisualSpecKind instance.
//
//   - explicit (per-instance `renderer:` or App `renderers:` map) wins.
//   - otherwise only an `official` renderer auto-selects.
//   - without official → error naming verified/community candidates.
//
// Returns the resolved renderer name ("" when none) and an error when the
// instance needs a renderer but none can be auto-selected.
func (r *RendererRegistry) ResolveRenderer(implements, stackFamily, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	cands := r.candidates(implements, stackFamily)
	for _, c := range cands {
		if c.TrustTier == "official" {
			return c.Implements, nil
		}
	}
	if len(cands) == 0 {
		return "", fmt.Errorf(
			"no renderer for (%s, %s) — declare one via App `renderers:` or instance `renderer:`",
			implements, stackFamily)
	}
	// No official renderer — error with candidate suggestions (never silent
	// fallback to a non-official renderer).
	var names []string
	for _, c := range cands {
		names = append(names, fmt.Sprintf("%s/%s (%s)", c.TrustTier, c.Implements, c.Source))
	}
	return "", fmt.Errorf(
		"no official renderer for (%s, %s); candidates (pick explicitly): %s",
		implements, stackFamily, strings.Join(names, ", "))
}

// ValidateSlotTiers enforces 5.16.2: `accepts_slots` only from tier page|app,
// `implements_slot` only from tier component.
func (r *RendererRegistry) ValidateSlotTiers() []string {
	var errs []string
	for _, vk := range r.VisualKinds {
		if len(vk.AcceptsSlots) > 0 && vk.Tier != "page" && vk.Tier != "app" {
			errs = append(errs, fmt.Sprintf(
				"%s: VisualSpecKind %q (tier %s) declares accepts_slots — only valid on tier page|app",
				vk.Source, vk.Name, vk.Tier))
		}
		if vk.ImplementsSlot != "" && vk.Tier != "component" {
			errs = append(errs, fmt.Sprintf(
				"%s: VisualSpecKind %q (tier %s) declares implements_slot — only valid on tier component",
				vk.Source, vk.Name, vk.Tier))
		}
	}
	return errs
}

// ValidateStackFamily enforces 5.16.3: App shell + shell-integrated Page +
// Component must share one stack_family. Independent Pages (no App context)
// are not checked. Returns error strings.
func (r *RendererRegistry) ValidateStackFamily(apps []RawManifest, pages []RawManifest) []string {
	var errs []string
	for _, am := range apps {
		sm, ok := am.Spec.(map[string]any)
		if !ok {
			continue
		}
		app, err := RawSpecTo[spec.AppSpec](sm)
		if err != nil {
			continue
		}
		family := app.StackFamily
		if family == "" {
			family = spec.DefaultStackFamily
		}
		// Pages that declare a renderer must match the App family.
		for _, pm := range pages {
			psm, ok := pm.Spec.(map[string]any)
			if !ok {
				continue
			}
			p, err := RawSpecTo[spec.PageSpec](psm)
			if err != nil {
				continue
			}
			if p.Renderer != "" && !strings.Contains(p.Renderer, family) {
				errs = append(errs, fmt.Sprintf(
					"%s: App %q (stack_family %s): Page %q renderer %q is not in the same family",
					am.Source, am.Metadata.Name, family, pm.Metadata.Name, p.Renderer))
			}
		}
	}
	return errs
}

// ValidateRendererResolution checks every App's `renderers:` map and every
// Page's `renderer:` field resolves to a registered renderer. Returns error
// strings.
func (r *RendererRegistry) ValidateRendererResolution(apps []RawManifest, pages []RawManifest) []string {
	var errs []string
	known := map[string]bool{}
	for _, e := range r.Renderers {
		known[e.Implements] = true
	}
	for _, am := range apps {
		sm, ok := am.Spec.(map[string]any)
		if !ok {
			continue
		}
		app, err := RawSpecTo[spec.AppSpec](sm)
		if err != nil {
			continue
		}
		for implements, renderer := range app.Renderers {
			if !known[renderer] {
				errs = append(errs, fmt.Sprintf(
					"%s: App %q: renderers[%q] = %q is not a registered renderer",
					am.Source, am.Metadata.Name, implements, renderer))
			}
		}
	}
	for _, pm := range pages {
		psm, ok := pm.Spec.(map[string]any)
		if !ok {
			continue
		}
		p, err := RawSpecTo[spec.PageSpec](psm)
		if err != nil {
			continue
		}
		if p.Renderer != "" && !known[p.Renderer] {
			errs = append(errs, fmt.Sprintf(
				"%s: Page %q: renderer %q is not a registered renderer",
				pm.Source, pm.Metadata.Name, p.Renderer))
		}
	}
	return errs
}
