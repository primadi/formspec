# `docs/renderers/shadcn-shell/05-routing.md` — dokumen routing otoritatif

**Tanggal**: 2026-09-25 · **Plan**: `docs_internal/plan/routing-docs-and-menu-visibility.md`
**Todo**: 5.22.5 · **Changelog terkait**: `2026-09-25-001..004`

## Apa yang diubah

Dokumen baru `docs/renderers/shadcn-shell/05-routing.md` (10 bagian), terdaftar
di `docs/renderers/shadcn-shell/README.md` dan sidebar VitePress
(`docs-site/.vitepress/config.mts`).

Isinya: komposisi URL (workspace → `root_url` → surface → segmen → query,
termasuk jebakan `buildRoutes basePath` vs `mountPrefix`); empat jenis route
A/B/C/D; cara membedakannya (4 sinyal + pohon keputusan + tabel contoh promo);
cara resolve (route→komponen, Form→spec, menu `view`→route); lapisan menu
(termasuk jawaban "paling depan bukan menu"); tabel dua sumbu + lima lapis
visibilitas; gate deploy-time; divergensi & sisa terbuka; resep diagnosis
"URL ini jalur apa?"; cross-ref.

Plus test `src/shell/router.shapes.test.tsx` (13 test) yang mengunci bentuk path
tiap jalur.

## Kenapa

Pertanyaan yang memicu seluruh pekerjaan ini — _"route `/cafe-master/promos`
dihandle entity `promo`; karena ada `promo-form`, form itu yang dipakai?"_ dan
_"kalau ada lebih dari 1 Form, apakah router bisa memilih?"_ — tidak punya satu
pun dokumen yang menjawabnya. Jawabannya tersebar di empat berkas kode, dan
menelusurinya menemukan dua belas fakta yang tidak tertulis di mana pun,
termasuk yang berlawanan dengan dugaan:

- pemilihan Form **bukan** di router: jalur A/B/D lewat `form.ref` eksplisit,
  jalur C lewat konvensi `{entity}-create/-edit/-form`;
- jalur D mengambil Form **pertama secara alfabetis**, bukan yang paling cocok —
  kafe punya contoh nyata (entity `order`, dua Form, tombol New memakai
  `order-form-pos` yang ber-layout mode _edit_);
- `spec.mode` pada Form **tidak** memilih mode runtime (hanya perm derived Page +
  footprint grant);
- jalur B selalu mode `view` karena block-nya dibuat tanpa `mode`;
- menu adalah **konsumen** route, bukan pangkalnya.

Dokumen ini juga tempat menulis keputusan yang tidak boleh hilang: **`when` bukan
gerbang otorisasi**, dengan tabel lima lapis yang menjelaskan apa yang benar-benar
menahan bypass.

## Bukti

- `npx vitest run src/shell/router.shapes.test.tsx` → 13 lulus (satu test per
  jalur + keluarga kind navigasi + satu test yang mengunci ketiadaan route untuk
  overlay).
- `cd docs-site && npm run build` — diperiksa pada langkah verifikasi plan.
- Suite vitest penuh 444 lulus, `tsc -b` bersih.
