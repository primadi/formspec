# 4.2 — `summary` menolak `hooks:`/`conditions:` (GAP-33)

**Tanggal:** 2026-09-18 · **Plan:** `docs_internal/plan/fase-4-kafe-stok-hpp.md`
(4.2) · **TODO:** `examples/kafe/gaps_found/TODO.md` 4.2

## Apa yang diubah

`ValidateEntitySpec` (`pkg/spec/entity.go`) kini **menolak** `hooks:` dan
`conditions:` yang dipasang pada entity `characteristic: summary`, dengan pesan
yang menunjuk ke `maintained_by` + `invariants` (Core Extended §6.1).

## Kenapa

Entity `summary` tidak punya action pipeline — `create`/`update`/`delete`
permanen nonaktif, dan penulisannya lewat script `maintained_by`. Akibatnya
`hooks:`/`conditions:` yang dipasang di sana **tidak pernah dipanggil**, tetapi
manifest-nya **terlihat** terlindungi. Itu kelas kegagalan yang sama dengan
#52/#53/GAP-33: "terlihat terpasang tapi tidak jalan".

Dua pilihan: (a) benar-benar memanggil hook di jalur tulis summary, atau
(b) menolak deklarasinya. Dipilih **(b)** karena jalur tulis summary adalah
script pemelihara — memanggil hook di sana berarti menambah pipeline kedua yang
harus dijaga sinkron dengan pipeline aksi, padahal kontrak yang sudah ada
(`maintained_by` + `invariants`, ditopang unique index) sudah menjawab
pertanyaan yang sama dengan penopang **database**, bukan disiplin script.

## File yang terkena dampak

- `pkg/spec/entity.go` — penolakan di blok `CharSummary` `ValidateEntitySpec`.
- `pkg/spec/entity_test.go` — `TestValidateEntitySpec_SummaryRejectsHooks`.
- `docs/spec/backend/02-core-extended.md` §6.1 — validator menolak, bukan
  sekadar "tidak dipanggil".
- `examples/kafe/spec/modules/cafe-stock/summary/stock-level/entity.yaml` —
  marker GAP-33 ditutup.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./pkg/spec/ -run TestValidateEntitySpec_Summary` | 3 PASS |
| spec uji (summary + hooks) `formspec validate` | `[FAIL] … summary entity declares hooks … use maintained_by + invariants instead` |
| `formspec validate --spec examples/kafe/spec --schema schemas` | **0 problem** (69 manifest) |
| `go test ./...` | hijau |
