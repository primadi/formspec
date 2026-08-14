# 2026-08-14-006 — Logo baru "Spec Stack" di semua surfaces

**Apa:** mengganti logo monogram huruf "M" dengan mark baru **"Spec Stack"** —
tiga bar horizontal menurun di atas kotak rounded bergradien indigo `#6366f1` →
emerald `#10b981`.

**Kenapa:** logo lama hanya monogram satu huruf dan tidak menceritakan apa yang
FormSpec lakukan (spec-first, declarative). Mark baru menggabungkan "Form" (baris
= field input) dan "Spec" (lapisan = struktur deklaratif bersarang).

**File terdampak:**

- `site/public/favicon.svg`, `docs-site/public/favicon.svg`,
  `renderers/react-shadcn/public/favicon.svg`, `cmd/formspec/dist/favicon.svg`
  (favicon, termasuk yang di-embed ke CLI via `go:embed`)
- `site/src/components/Nav.tsx` (mark inline di navbar + footer)
- `renderers/react-shadcn/src/shell/Sidebar.tsx` (komponen `LogoMark` di
  sidebar mobile + desktop, mengganti "F" saat collapsed)
- `scripts/publish-schemas.sh` + `schemas/dist/index.html` (brand registry
  schema: `.dot` → mark SVG; + favicon `<link>` dan staging `favicon.svg`)
- `docs-site/dist/favicon.svg` (hasil build VitePress — source `public/` sudah
  update, dist disinkronkan)
- `schemas/dist/favicon.svg` (favicon registry, baru — sebelumnya tidak ada)
- `brand/` (aset proposal: 3 konsep + `preview.html`)

**Referensi:** proposal di `brand/README.md`. Catatan: bundle admin SPA yang
sudah ter-build (`cmd/formspec/dist/assets/*`) perlu di-rebuild (`make build`)
agar perubahan `Sidebar.tsx` ikut ter-embed ke binary.
