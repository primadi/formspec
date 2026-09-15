# Baseline Validasi — Aplikasi Kafe

> Dijanjikan oleh `docs/architecture.md` §0 (Aturan 3) tapi belum pernah dibuat.
> Dibuat 2026-09-14 sebagai bagian dari **Fase 0** (`TODO.md`).

## Aturan pakai

`formspec validate` harus **selalu hijau** (0 problem) pada `spec/` aplikasi kafe.

Karena spec ini adalah **spec ideal** (mengandalkan gap diselesaikan), hijau-nya
validate **bukan** berarti aplikasi bisa jalan. Setiap problem yang muncul di
luar daftar ini adalah **bug nyata** (regresi), bukan "gap yang diharapkan".

## Baseline saat ini

**Dengan schema lokal (repositori ini)** — diharapkan **0 problem**:

```console
$ cd examples/kafe
$ ../../formspec validate --spec spec --schema ../../schemas
...
69 manifest(s) validated, 0 problem(s) found
```

**Dengan schema registry (default)** — diharapkan **1 problem** sejak 2026-09-14:

```console
$ ../../formspec validate --spec spec
...
69 manifest(s) validated, 1 problem(s) found
# spec/apps/kafe-qr.yaml#0
#   schema: /spec: additional properties 'public_entities' not allowed
```

**Mengapa ini diharapkan, bukan regresi.** `kafe-qr` memakai konstruksi baru
`spec.public_entities` (S3, gap #6). Schema-nya sudah digenerate di
`schemas/` repositori ini, tetapi **schema yang dipublikasikan di registry
(`https://schemas.formspec.dev`) belum di-refresh** — jadi `validate` tanpa
`--schema` memakai salinan lama dan tidak mengenal properti itu. Setelah registry
di-refresh (siklus rilis), problem ini hilang dan kedua mode kembali **0**.

> Aturan tetap berlaku: problem **di luar** dua baris di atas adalah regresi nyata.
> Sebelum memakai `scope:`/`public_entities` di cabang/klon baru, jalankan
> `formspec schema refresh` atau pakai `--schema <repo>/schemas`.

## Peringatan penting: hijau ≠ bisa dijalankan

Validator **tidak menangkap** kelas kesalahan yang justru mematikan aplikasi ini.
Terbukti pada Fase 0:

| Yang tidak ditangkap                   | Bukti                                                                                                                                    |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| **Script Starlark gagal kompilasi**    | 3 guard script di `spec/modules/*/scripts/*.star` tidak bisa dikompilasi, tetapi `validate` melaporkan **0 problem**. Lihat gap #49/#50. |
| **`indexes:` di level spec diabaikan** | Diverifikasi lewat `formspec migrate plan` — DDL tanpa index. Lihat gap #22.                                                             |
| **`PrintOutput.format: thermal`**      | Schema menerima, tapi tidak ada handler. Lihat gap #10.                                                                                  |
| **Salah ketik nama widget**            | `widget` bertipe `string` bebas. Lihat gap S10.                                                                                          |
| **Referensi menggantung**              | `App.spec.modules`, menu `view:`, `impl.ref`. Lihat gap #21.                                                                             |
| **Tipe Postgres bocor ke SQLite**      | `_transaction_date timestamptz` muncul di DDL SQLite. Lihat gap #27.                                                                     |

## Perintah yang dipakai untuk baseline

```console
# Validasi spec (gerbang utama — pakai schema lokal repo ini)
formspec validate --spec spec --schema ../../schemas    # → 0 problem (baseline)

# Validasi lewat registry (default) — 1 problem DIHARAPKAN sampai schema
di registry di-refresh (lihat bagian di atas)
formspec validate --spec spec

# DDL & index (mendeteksi gap #22/#23/#27)
formspec migrate plan --spec spec --dsn sqlite:.formspec/tmp.db

# Runtime (mendeteksi gap #44/#45/#46/#49)
formspec dev --spec spec --dsn sqlite:.formspec/tmp.db --addr :18123 --workspace-id kafe
curl -s -X POST http://localhost:18123/kafe/_ui/entity/<module>/<entity> -d '{...}'
```

## Riwayat

| Tanggal    | Problem | Catatan                                                                                             |
| ---------- | ------- | --------------------------------------------------------------------------------------------------- |
| 2026-09-14 | 0       | Baseline awal (Fase 0). 69 manifest.                                                                |
| 2026-09-14 | 0 / 1   | `kafe-qr` mulai memakai `public_entities` (S3). Schema lokal **0**; registry cache **1** (belum di-refresh). |
