# Status meja kafe pakai `state_machine` — revisi plan alur sesi meja

**Tipe:** revisi plan (tanpa perubahan kode). **Revisi 2** dari
`docs_internal/plan/kafe-qr-table-session-flow.md`.

**Pemicu:** dua koreksi pemilik atas rancangan saya, 2026-09-27 —
(1) "tidak perlu timer, begitu pelanggan membayar status meja jadi occupied";
(2) "status meja pakai `state_machine`".

## Koreksi yang diterima

Revisi 1 saya mengusulkan **"occupied = ada sesi terbuka"** karena v1 tidak punya
timer (`Schedule`/`Cron` tidak ada di `KnownKinds`; `durable` sudah dicabut).
Itu **terlalu jauh**: pemicunya bukan timer, melainkan **pembayaran** — jadi
`state_machine` memang tepat, dan usulan proyeksi-sesi saya tidak perlu.

## Yang diverifikasi untuk rancangan baru

| Pertanyaan                            | Hasil                                                                                                                                                                                                                |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `master` boleh punya `state_machine`? | **Ya** — tidak ada aturan yang melarang. Diuji dengan menambal salinan spec (field `table_status` + SM + actions `occupy`/`release`): `85 manifest(s) validated, 0 problem(s)`                                       |
| Transisi tanpa `impl` butuh route?    | **Tidak** — `PATCH` yang mengubah field state memicu transisi (`internal/api/handler.go:1031` `FindTransitionByStates`; `renderers/jsonb-persist/crud.go` `validateStateTransition` mencocokkan `from`+`to` + guard) |
| Clear meja jadi apa?                  | **`PATCH table_status: available`** — tanpa `impl`, tanpa route. Pola sama dengan `shift`                                                                                                                            |
| Bisa menulis meja dari event?         | **Ya** — preseden `sales-to-journal.yaml` → `journalize.star` (menulis `gl.journal-entry` dari `on_paid`)                                                                                                            |
| `resource.upsert` untuk meja?         | **Tidak** — `UpsertProjection` summary-only; `dining-table` = `master` → `find`→`set`→`save`                                                                                                                         |

## Dampak yang paling berharga: serangan tadi melemah di akarnya

Rancangan ini memindahkan sumbu "meja terisi" dari **keberadaan sesi** ke
**field yang digerbangi permission**. Terukur sebelumnya: anonim bisa
`POST table-session` (→ **201**), sehingga `occupied` bisa diklaim tanpa bukti.
Dengan `state_machine`, `occupied` hanya lahir dari jalur ber-permission
(subscription saat bayar, atau staf) — **anonim tidak bisa lagi mengunci meja**,
karena `public_entities` hanya memberi `find` pada `dining-table`.

## Item yang gugur / lahir

- **Gugur (untuk meja):** "clear meja tidak punya route" — tidak perlu route;
  cukup `PATCH` + grant. **10.35a** menyempit ke _sesi_ (menutup `table-session`).
- **Baru:** **10.40** (`dining-table` state machine + actions + grant) dan
  **10.40b** (subscription `on_paid` → `occupied`, `on_cancel` → `available`).
- **Tetap:** 10.34c, 10.36, 10.37, 10.38, 10.39, 10.20.

## Risiko yang dicatat jujur (belum ditutup)

Subscription berjalan **async setelah commit**: bila handler gagal, meja tetap
`available` sementara pesanan sudah lunas. Outbox retry menutup sebagian.
Bukti pengamatan yang disiapkan: matikan handler → bayar → `table_status` tetap
`available`. Bila tidak ditutup di 10.40b, ini harus jadi item `⏸️` tersendiri.

**Plan:** `docs_internal/plan/kafe-qr-table-session-flow.md` (revisi 2)
**Ledger:** kafe 10.34, 10.34c, 10.35, 10.35a (menyempit), 10.36, 10.37, 10.38,
10.39, **10.40**, **10.40b**, 10.20.
