# Plan: Tutup item todo yang masih terbuka (2026-09-26)

**Pemicu:** permintaan pemilik — "kerjakan semua todo yg masih belum tertutup".
**Status awal:** 83 item `[ ]`/`[⏸️]` di `docs_internal/plan/todo.md` (2 677 baris).

Plan induk ini mengelompokkan item berdasarkan **apa yang menghalanginya**, karena
83 item tidak sama jenisnya:

| Kelompok     | Sifat                                                                          | Tindakan yang benar                                             |
| ------------ | ------------------------------------------------------------------------------ | --------------------------------------------------------------- |
| A. Stale     | Klaim sudah tidak benar (fitur sudah landing / file sudah ada)                 | Verifikasi + tutup dengan bukti                                 |
| B. Small     | Fix jelas, tanpa keputusan kontrak baru                                        | Implementasi                                                    |
| C. Medium    | Fix jelas, butuh kerja beberapa file                                           | Implementasi                                                    |
| D. Terblokir | Butuh keputusan desain, SDK baru, cloud phase, atau verifikasi browser manusia | **Tetap `⏸️`** — perjelas alasan + turunkan ke daftar keputusan |

Kelompok D tidak akan ditutup dengan kode: menutupnya berarti menebak kontrak,
dan `AGENTS.md` §3 melarang menandai `✅` untuk pekerjaan yang hanya sebagian
landing. Yang dilakukan untuk D: memastikan **alasannya jujur** (prasyaratnya
masih benar-benar belum ada) dan buktinya bisa diperiksa.

## Kelompok A — stale (verifikasi dulu, tutup tanpa kode bila benar)

| Item            | Klaim lama                                 | Kenyataan (terverifikasi)                                                                                                          |
| --------------- | ------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| 3.6.3a          | `docs/kind/` belum punya halaman `Seed`    | `docs/kind/data/Seed.md` **ada** (commit `dd3adc6`); `kindGroups` memuat `Seed`; `docs/kind/README.md` total 34 cocok (data/ = 11) |
| 9.4.1a          | kafe belum punya `kind: Seed` bagan akun   | `examples/kafe/spec/modules/gl/seeds/chart-of-accounts.yaml` **ada**                                                               |
| 3.7.6           | `backup create --filter` belum ada         | `matchesFilter` di `cmd/formspec/backup.go:392` + test                                                                             |
| 5.10.18         | (head masih `[ ]` padahal badan item `✅`) | perbaiki checkbox                                                                                                                  |
| 7.8.18, 5.10.24 | dua item melacak flake test yang sama      | gabung ke 5.10.24                                                                                                                  |

## Kelompok B — small

2.1.6 (`core.idempotency_retention` → `IdempotencyTTL`) · 5.10.22 (`generate` union
dari `options`) · 7.8.11 (konsistensi `emit:`↔`emits:`) · 5.10.24 (fix
`waitForJournal` poll **status**, bukan keberadaan baris) · 5.22.8 (docs
`01-architecture.md` §5) · 3.7.6 (tutup, tambah test bila kurang) · 3.7.7
(`restore --map-resource`) · 3.7.8 (`logs` filter lanjutan) · 5.22.6 (`routeExists`
vs `bundle.pages`) · 5.23.3 (`SearchSelect` lewat resolver).

## Kelompok C — medium

4.8.7 (**bug senyap**: `backup`/`restore` menulis workspace `"demo"` hardcoded →
arsip kosong tanpa error) · 3.7.9 (`archive restore-batch`) · 4.2.6 (satu transaksi
untuk `dml`+`ddl`) · 5.12.8 (handler bulk action) · 5.10.17 (wizard memakai
kosakata widget) · 5.21.2 (dialog di sel tabel + kartu katalog) · 5.13.6 (item
`ApprovalInbox` diisi dari step) · 7.7.5 (idempotency di jalur `Dispatch`) ·
5.23.2 (bersihkan 81 `description` kafe) · 5.11.7/5.14.6/5.10.21 (satu fixture
konformansi bersama) · 7.8.13 (`load()` Starlark) · 3.6.8 (`rebuild partial`).

## Kelompok D — tetap ⏸️ (butuh keputusan / besar / cloud)

1.4.12 · 2.6.5 · 3.2.6 · 3.9.2 · 4.4.4 · 4.8.6 · 5.10.16 · 5.10.20 · 5.10.23 ·
5.11.8 · 5.22.7 · 6.2.5 · 6.2.6 · 6.3.3 · 6.3.5 · 6.6.5 · 6.7.1 · 6.7.4 · 7.4.8 ·
7.8.13-nya sebagian · 7.8.17 · 7.9.1–7.9.4 · 7.15.1 · 7.17.3 · 7.18.1–7.18.3 ·
7.19.1–7.19.2 · 8.1.6 · 8.1.7 · 8.2.7 · 8.3.1 · 8.3.2 · 9.1.1 · 9.3.1 ·
10.3.3 · 10.5.1–10.5.4 · 13.3.3 · 13.3.5 · 13.3.9 · 13.5.7 · 17.7 · 2.11.9 ·
2.11.10 · 2.12.7 · 2.12.8 · 5.2.20 · 5.6.7 · 3.9.2 · 5.18.3 · 5.18.5 · 3.8.

Untuk kelompok D, deliverable-nya adalah **kejujuran**: setiap item harus
menyebut (a) prasyarat yang belum ada, (b) bukti bahwa ia belum ada, dan (c)
keputusan siapa/apa yang ditunggu. Item yang ternyata prasyaratnya sudah landing
dipindah ke kelompok A/B/C.

## Urutan kerja

1. Kelompok A dulu (murah, menghapus misinformasi).
2. Kelompok B per item, satu changelog per item (atau satu changelog untuk
   beberapa item serumpun agar tidak meledak jumlahnya).
3. Kelompok C per item.
4. Rapikan checklist `[ ]` → `[⏸️]` untuk D + update header `todo.md`.

## Verifikasi (tiap perubahan)

`go build ./...` · `go test ./...` · `cd renderers/react-shadcn && npx vitest run`
· `npx tsc -b` · `formspec validate --spec examples/kafe/spec --schema schemas` ·
`formspec check --spec examples/kafe/spec`.
