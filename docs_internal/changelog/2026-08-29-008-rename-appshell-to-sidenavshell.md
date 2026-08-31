# 2026-08-29-008 — Rename AppShell → SideNavShell

## Apa

Rename komponen shell `AppShell` menjadi `SideNavShell` agar namanya konsisten
dengan shell archetype `sidebar-nav` dan sejajar dengan `TopNavShell` /
`NoNavShell`.

- `renderers/react-shadcn/src/shell/AppShell.tsx` → `SideNavShell.tsx`
- Fungsi `AppShell` → `SideNavShell`; export di `shell/index.ts` tanpa alias
- Update import & registry `APP_SHELLS` di `src/App.tsx`
- Update komentar di `TopNavShell.tsx`, `useResolvedMenu.ts`
- Update referensi di docs (`docs/renderers/shadcn-shell/01-architecture.md`,
  `03-kind-renderers.md`, `docs/plan/chrome-composition-spec.md`,
  `docs/plan/landing-page.md`, `docs/plan/todo.md`),
  `examples/storefront/README.md`, dan `.github/skills/forma-frontend/SKILL.md`

## Kenapa

Nama `AppShell` tidak deskriptif — semua shell (TopNav, NoNav) sama-sama
"app shell". Nama `SideNavShell` menyatakan archetype-nya secara eksplisit.

## Verifikasi

`tsc --noEmit` di `renderers/react-shadcn` — no errors.

## Referensi

- `docs/plan/chrome-composition-spec.md`
