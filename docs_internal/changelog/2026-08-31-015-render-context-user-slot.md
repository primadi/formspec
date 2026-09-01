# 2026-08-31-015 — Render context standard: standard slot `user` + interpolasi blocks

## Apa

Phase 1 dari plan `docs_internal/plan/render-context-standard.md`:
standarisasi context render Page/Form dengan **standard slot `user`**
(identitas session dari `/_meta/me`) yang di-inject renderer ke context
blocks, plus interpolasi token `{user.*}` di section text.

- `renderers/react-shadcn/src/components/sections/SectionBlocks.tsx`:
  helper `t()` + prop `context` di semua block (hero, feature_grid, card,
  carousel, cta, alert) — title/subtitle/item.title/item.text kini
  mendukung token `{dotted.path}`.
- `renderers/react-shadcn/src/kinds/page/PageRenderer.tsx`: `userCtx =
{ user: me }` dari session store, di-pass ke `PageBlockRenderer` →
  `SectionBlockRenderer`; page title juga di-interpolasi dengan `userCtx`.
- `registry/spec/modules/portal/pages/profile.yaml`: migrasi dari
  `mode: custom` asset ke **pure YAML blocks** memakai `{user.username}`,
  `{user.user_id}`, `{user.workspace}`, `{user.roles}`, `{user.permissions}`.
  `assets/profile.js` dipertahankan sebagai contoh escape hatch.

## Kenapa

Halaman profile sebelumnya terpaksa custom asset karena blocks tidak bisa
render identitas session. Dengan standard slot `user` (keputusan desain:
hardcode karena zero-cost, data caller sendiri, backward compat), halaman
account bisa pure YAML — selaras prinsip Convention over Configuration.

## Verifikasi

- `tsc --noEmit` bersih; `vitest run` 166 pass, 0 fail.
- `formspec validate -spec registry/spec -schema schemas` — 13 manifest,
  0 problem.
- Browser: login TestUser → Profile → `/default/portal/profile` render
  penuh: "TestUser", "User ID: <uuid> · Workspace: default", Roles,
  Permissions. Tab title "Profil Saya · FormSpec Registry".

## File terdampak

- `renderers/react-shadcn/src/components/sections/SectionBlocks.tsx`
- `renderers/react-shadcn/src/kinds/page/PageRenderer.tsx`
- `registry/spec/modules/portal/pages/profile.yaml`
- `docs_internal/plan/render-context-standard.md` (status Phase 1 ✅)

## Lanjutan

Phase 2 (plan): `spec.context` declaration — closed source set
`session | entity | api | const | expr`, permission ceiling untuk
entity/api, loading/error/fallback states untuk async sources.
