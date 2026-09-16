# Plan — `read_all`: pengecualian `row_scope` yang eksplisit (keputusan untuk 3.5)

Sumber: pertanyaan desain yang menghambat `examples/kafe/gaps_found/TODO.md` **3.5**.
Jawaban pemilik proyek: **opsi C — permission eksplisit** (bukan bypass `*`
implisit, bukan wildcard di atribut).

## Masalah

`row_scope` memfilter pembacaan dari nilai sesi dan **fail closed**: kalau
seorang principal tidak punya nilai dimensi (tidak ada baris `employee` yang
`username`-nya cocok), setiap pembacaan 403. Itu benar untuk kasir cabang — tapi
salah untuk:

| Pemanggil                         | Kenapa tidak punya dimensi                                |
| --------------------------------- | --------------------------------------------------------- |
| Pemilik workspace                 | justru tidak bertugas di satu cabang; ingin melihat semua |
| Super-admin / identitas dev (`*`) | identitas teknis, tanpa baris `employee`                  |
| Auditor lintas cabang             | tugasnya memang membandingkan antar cabang                |

Tanpa pengecualian, menyalakan 3.5 mematikan aplikasi bagi orang yang paling
butuh melihat semuanya — termasuk walkthrough `formspec dev`.

## Konstruk

`{module}.{plural}.read_all` — permission **kebijakan**, bukan aksi:

```yaml
permissions:
  - cafe-order.orders.read_all
```

Aturan:

| Aturan                                                                                                     | Alasan                                                                                                                     |
| ---------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Nama ikut bentuk kanonik `{module}.{plural}.{action}` (D5), plural dari `spec.plural` (fallback `<name>s`) | konsisten dengan registry & route generator; bisa digabung dengan grant lain tanpa aturan khusus                           |
| Pemegangnya **dilewati** dari `row_scope` entity itu                                                       | "lihat semua" berarti tidak ada filter; atribut yang tak bisa diselesaikan bukan lagi error                                |
| **Didaftarkan** bersama permission standar                                                                 | agar bisa diberikan & terlihat di audit — pertanyaan "siapa boleh baca lintas cabang" dijawab daftar grant, bukan wildcard |
| Tidak punya route sendiri                                                                                  | ia tidak menjalankan operasi apa pun; ia mengecualikan                                                                     |
| `*` tetap memenuhi                                                                                         | identitas dev/super-admin tidak boleh mati; pemeriksaannya memakai `HasPermission` yang sama di mana-mana                  |
| Cakupan **per entity**                                                                                     | `read_all` pada `order` tidak memberi akses lintas cabang pada `shift`                                                     |

## File

- `internal/api/scope.go` — `ReadAllPermission()` + pemeriksaan di awal
  `applyRowScope` (signature bertambah `module, entity`).
- `internal/api/handler.go` — call site `HandleList`.
- `internal/entity/registry.go` — registrasi `read_all` (dengan komentar bahwa ia
  kebijakan, bukan route).
- `internal/api/scope_test.go` — 2 test baru + penyesuaian signature.
- `docs/spec/backend/01-core-basic.md` §1.7 (pengecualian) + §8.6 (permission
  kebijakan).

## Bukti

`TestApplyRowScope_ReadAllPermissionExempts` (pemegang `read_all` tanpa atribut
tidak 403 dan tidak difilter; kasir tanpa `read_all` pada entity yang sama tetap
fail closed) · `TestReadAllPermission` (bentuk nama, termasuk fallback plural) ·
`go test ./...` hijau · kafe `validate` 0 problem.

## Yang membuka apa

Ini menghapus penghambat **3.5** (menyalakan penyaringan otomatis per cabang):
spec kafe bisa memasang `row_scope` tanpa mematikan aplikasi bagi pemilik dan
`formspec dev`, dan role pemilik cukup diberi `read_all` per entity yang
dimaksud.

## Estimasi: **small** (satu pemeriksaan + registrasi + 2 test + dokumen)
