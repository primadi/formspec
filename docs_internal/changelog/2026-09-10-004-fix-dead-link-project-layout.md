# 2026-09-10-004 — Fix dead link di spec/platform/08-project-layout.md

Build Cloudflare Pages (`docs-site`) gagal karena VitePress mendeteksi
1 dead link: `./../registry/05-self-hosting` di
`docs/spec/platform/08-project-layout.md`. Penyebabnya salah depth path
relatif: dari `docs/spec/platform/`, `../registry/` mengarah ke
`docs/spec/registry/` (tidak ada), bukan `docs/registry/`.

Perbaikan: link diubah ke `../../registry/05-self-hosting.md`.
Verifikasi: `npm run build` di `docs-site/` sukses tanpa dead link.

- File terdampak: `docs/spec/platform/08-project-layout.md`
- Referensi: build error Cloudflare Pages 2026-09-10
