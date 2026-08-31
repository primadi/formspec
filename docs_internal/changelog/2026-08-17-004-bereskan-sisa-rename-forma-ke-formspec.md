# 2026-08-17-004 — Bereskan sisa rename forma → formspec

**Apa:** Menuntaskan sisa rename `forma` → `formspec` yang belum selesai di
source docs (sebelumnya hanya toleransi dead-link di docs-site, bukan rename
nyata). Referensi: `docs/plan/rename-formspec.md` (kini status Complete).

## Perubahan

1. **Rename 4 file `docs/ai/`** (git mv, history terjaga):
   - `02-forma-consult.md` → `02-formspec-consult.md`
   - `03-forma-local-mcp.md` → `03-formspec-local-mcp.md`
   - `04-forma-remote-mcp.md` → `04-formspec-remote-mcp.md`
   - `06-forma-skill.md` → `06-formspec-skill.md`
   - README `docs/ai/README.md` sudah mereferensikan nama `formspec` — kini
     link-nya valid.

2. **Rename 8 file `docs/comparison/`** (git mv):
   - `forma-vs-{budibase,custom-app,frappe,laravel,pocketbase,springboot,supabase,vercel}.md`
     → `formspec-vs-{...}.md` (README `docs/comparison/README.md` sudah pakai
     nama `formspec-vs-*`).

3. **Fix referensi stale `08-formaexpr.md` → `08-formspec-expr.md`** di 11 file
   (docs spec frontend, cli-tools, guides, plan, `ai_skills/`, contoh
   `examples/*/.agents/skills/`). File target `08-formspec-expr.md` sudah ada;
   referensi lama menunjuk nama yang tidak ada.

4. **Update `docs-site/.vitepress/config.mts`**:
   - Sidebar `ai/` + `comparison/` memakai nama baru.
   - Hapus entri dead-link tolerance yang kini ter-resolve
     (`02/03/04/06-formspec-*`, `formspec-vs-*`); tersisa `08-formaexpr` (masih
     ada referensi di docs source yang belum dibereskan).

5. **Update `docs/architecture/09-domain-map.md`** — referensi
   `docs/ai/04-forma-remote-mcp.md` → `04-formspec-remote-mcp.md`.

## File terdampak

- `docs/ai/*` (4 rename), `docs/comparison/*` (8 rename)
- `docs-site/.vitepress/config.mts`
- `docs/architecture/09-domain-map.md`
- `docs/spec/frontend/{README,06-page-kinds,07-component-kinds}.md`
- `docs/cli-tools/02-formspec-cli.md`, `docs/guides/{authoring-a-shell,authoring-a-page-renderer}.md`
- `docs/plan/{todo,rename-formspec}.md`
- `ai_skills/formspec-spec-structure/SKILL.md` + 3 salinan di `examples/*/.agents/skills/`

## Verifikasi

- `docs-site` build hijau (VitePress, 123 halaman).
- `make build` hijau (binary di-rebuild setelah sed tak sengaja menyentuh
  artifact `bin/formspec` & `formspec` yang gitignored).
- Residual `forma` hanya di changelog historis (sengaja dibiarkan).
