# Plan: Theme Switcher + Theme Binding untuk Registry Portal

**Tanggal**: 2026-08-31
**Status**: ✅ complete
**Referensi spec**: `docs/spec/frontend/05-app-kinds.md` §4.1 (chrome matrix) & §6 (Theme Binding)

## Latar Belakang

Portal registry (`formspec-registry`, App `no-nav`) tampil tanpa theme switcher
dan background mengikuti mode sistem (default `system` — OS user dark → hitam).
Dua gap:

1. `NoNavShell` tidak merender `ThemeSwitcher` sama sekali (SideNav/TopNav sudah).
2. `theme_ref` (§6 Theme Binding) dideklarasikan di `pkg/spec` tapi tidak
   pernah di-wire ke bundle/frontend — App tidak bisa menetapkan tema default.

## Perubahan

| File                                                      | Perubahan                                                                                 | LOE   |
| --------------------------------------------------------- | ----------------------------------------------------------------------------------------- | ----- |
| `registry/spec/apps/registry.yaml`                        | `chrome.theme_switcher: show` (opt-in; no-nav default hide) + `theme_ref: registry-theme` | small |
| `registry/spec/modules/portal/themes/registry-theme.yaml` | **baru** — kind Theme, token indigo + radius                                              | small |
| `renderers/react-shadcn/src/shell/NoNavShell.tsx`         | Render `ThemeSwitcher` saat `chrome.theme_switcher === "show"`                            | small |
| `internal/ui/meta.go`                                     | `AppContext.ThemeRef` + `AppSummary.Theme` (json `theme`) + wiring `BuildBundle`          | small |
| `internal/api/meta.go`                                    | `resolveAppContext` mengisi `ThemeRef` dari `App.spec.theme_ref`                          | small |
| `renderers/react-shadcn/src/types/manifest.ts`            | `AppSummary.theme?: string`                                                               | small |
| `renderers/react-shadcn/src/stores/prefs.ts`              | `themeTouched` flag — `setActiveTheme`/`setColorPreset` menandai user sudah memilih       | small |
| `renderers/react-shadcn/src/App.tsx`                      | Auto-apply `bundle.app.theme` selama `!themeTouched`                                      | small |
| `renderers/react-shadcn/src/shell/NoNavShell.test.tsx`    | Test opt-in `theme_switcher: show`                                                        | small |

## Keputusan Desain

- **Tidak mengubah default no-nav** `theme_switcher: hide` — sesuai §4.1,
  portal opt-in eksplisit di manifest (spec-first).
- **`themeTouched`** hanya di-set oleh pemilihan theme/color preset, BUKAN oleh
  toggle mode light/dark/system — ganti mode tidak boleh membatalkan tema App.
- Auto-apply hanya jika theme dengan nama `theme_ref` ada di bundle; kalau
  tidak, fallback ke index.css default (tidak error).
- Token theme di-inject sebagai `<style> :root` oleh `ThemeRenderer` — cascade
  light/dark/system toggle tetap berfungsi (§10 D35).

## Verifikasi

- `go test ./internal/ui ./internal/api` — 143 passed.
- `vitest run src/shell/NoNavShell.test.tsx` — 6 passed.
- `tsc --noEmit` — no errors.
- `formspec validate -spec registry/spec` — `registry-theme.yaml` OK (fail
  lain pre-existing schema drift: `chrome`/`logo`/`title`/`cache`/`title_visible`).
