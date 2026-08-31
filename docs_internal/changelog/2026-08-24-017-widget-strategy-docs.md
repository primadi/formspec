# 2026-08-24-017 — Widget Strategy Docs (todo.md sync + plan)

## Apa yang diubah

Menetapkan strategi widget FormSpec dan menyinkronkan `docs/plan/todo.md` dengan state renderer
terkini. Keputusan inti: **tidak semua komponen shadcn di-mapping ke field widget** — registry
widget dasar tetap closed set yang dikurasi (`07-component-kinds.md` §1), dengan tiga jalur
"UI rich": (1) field widget set tertutup, (2) chrome struktural via `formspec.ui`/
`formspec.components`/`formspec.files` untuk component `asset`, (3) block presentasi deklaratif
di Page (banner/alert/notice).

## Kenapa

Pertanyaan desain "apakah perlu mapping semua komponen shadcn ke widget" dijawab dengan
keputusan arsitektur yang konsisten dengan prinsip closed set. Selain itu, beberapa item
`todo.md` §5.10 sudah usang vs widget existing di renderer (DateInput/JsonInput/ChildTable
sudah ada), sehingga perlu di-refine agar tidak dikerjakan ulang.

## File terdampak

- `docs/plan/todo.md` — refine §5.10 (tandai 5.10.1–5.10.3 ✅, reframe 5.10.6/5.10.7/5.10.8),
  tambah §5.2.7 (banner/alert block) + §5.10a (kurasi 5.10.10–5.10.14), cross-link §7.17.1 ↔
  §5.10.5, note strategi di §5.9 & §5.10, update status header.
- `docs/plan/widget-strategy.md` — dokumen strategi baru (keputusan, konteks, 3 jalur, aturan
  kurasi, mapping track→todo, relevant files, verification).

## Referensi

- `docs/plan/widget-strategy.md`
- `docs/spec/frontend/07-component-kinds.md` §1–§4
- `docs/plan/todo.md` §5.2.7, §5.9, §5.10, §7.17
