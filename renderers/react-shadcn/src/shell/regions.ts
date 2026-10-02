// ─── Chrome Regions ───
//
// The declarative chrome model (frontend/05-app-kinds.md §4.2). An App shell is
// a fixed set of REGIONS — topbar, sidebar, rightbar, bottombar, footer — each
// filled with `none`, `auto` (the archetype's predefined fill) or a component
// reference. `content` is implicit (the page Outlet).
//
// Archetypes are presets over this one map: `sidebar-nav` is `no-nav` plus a
// filled sidebar, `topnav` is `no-nav` plus a filled topbar. That is the whole
// point — `no-nav` is not "no chrome", it is "no DEFAULT fill", so an App can
// attach its own topbar/sidebar/rightbar/bottombar (kafe-qr needs a chrome to
// host a session control).
//
// The backend resolves the map (`internal/ui.resolveChrome`) and ships it on
// `bundle.app.chrome.regions`; this module only fills the gaps for bundles
// that predate the field (older server / test fixtures) by deriving the same
// preset from the legacy booleans.

import type { ChromeConfig, ChromeRegion } from "@/types/manifest"

export const CHROME_REGIONS: ChromeRegion[] = [
  "topbar",
  "sidebar",
  "rightbar",
  "bottombar",
  "footer",
]

const NONE = "none"
const AUTO = "auto"

/** The archetype's predefined region map — mirrors `chromeRegionPreset` in
 *  internal/ui/meta.go. Keep the two in sync. */
export function presetFor(archetype: string): Record<ChromeRegion, string> {
  switch (archetype) {
    case "no-nav":
      // Chrome EXISTS (a minimal brand bar) but no default navigation.
      return {
        topbar: AUTO,
        sidebar: NONE,
        rightbar: NONE,
        bottombar: NONE,
        footer: AUTO,
      }
    case "topnav":
      return {
        topbar: AUTO,
        sidebar: NONE,
        rightbar: NONE,
        bottombar: NONE,
        footer: NONE,
      }
    default: // sidebar-nav
      return {
        topbar: AUTO,
        sidebar: AUTO,
        rightbar: NONE,
        bottombar: NONE,
        footer: NONE,
      }
  }
}

/**
 * The effective region map: the resolved `chrome.regions` when present,
 * otherwise the preset derived from the legacy chrome booleans (older bundle).
 */
export function effectiveRegions(
  chrome: ChromeConfig | undefined,
  archetype: string,
): Record<ChromeRegion, string> {
  const preset = presetFor(archetype)
  if (chrome?.regions) {
    for (const region of CHROME_REGIONS) {
      const v = chrome.regions[region]
      if (typeof v === "string" && v !== "") preset[region] = v
    }
    return preset
  }
  // Legacy bundle: no `regions`. Reconstruct the footer from the flag so
  // behaviour is unchanged (an older server never shipped the map); the other
  // regions already come from the archetype preset, which is exactly what the
  // legacy shells rendered.
  if (chrome) {
    const footerOn =
      chrome.footer === "show"
        ? true
        : chrome.footer === "hide"
          ? false
          : archetype === "no-nav"
    preset.footer = footerOn ? AUTO : NONE
  }
  return preset
}

/** A region is "filled" when its content is anything other than `none`. */
export function isRegionFilled(
  regions: Record<ChromeRegion, string>,
  region: ChromeRegion,
): boolean {
  return regions[region] !== NONE
}

/**
 * Whether a region holds a component reference (i.e. is neither `none` nor
 * `auto`) — the caller must render an AssetRenderer for it.
 */
export function regionComponent(
  regions: Record<ChromeRegion, string>,
  region: ChromeRegion,
): string | undefined {
  const v = regions[region]
  return v && v !== NONE && v !== AUTO ? v : undefined
}

/** Whether a region should render its predefined (auto) fill. */
export function regionIsAuto(
  regions: Record<ChromeRegion, string>,
  region: ChromeRegion,
): boolean {
  return regions[region] === AUTO
}
