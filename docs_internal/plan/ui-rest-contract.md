# Plan — Kontrak REST `/_ui/` terdokumentasi (kafe TODO 2.7 / gap #47)

Sumber: `examples/kafe/gaps_found/TODO.md` 2.7; `12-hasil-verifikasi-runtime.md`
(gap #47).

## Masalah

Surface `/_ui/` adalah kontrak publik — SPA bawaan pun memakainya — tetapi tidak
ada satu halaman pun yang menjelaskannya. Bentuk body, envelope, dan endpoint
aksi hanya ditemukan dengan gagal berkali-kali (`{"data": …}` → `400 unknown
field`; aksi dicoba-coba). Ironis untuk project yang menjanjikan manifest sebagai
satu-satunya sumber kebenaran: permukaan HTTP-nya justru yang paling tidak
terdokumentasi.

## Pendekatan

Dua bagian, dan yang kedua menjaga yang pertama:

1. **Halaman** `docs/runtimes/06-ui-rest-contract.md` — path, tabel method/aksi/
   permission, body flat, envelope respons, query list, catatan `row_scope`, dan
   ringkasan surface publik.
2. **`formspec describe entity <name>` mencetak kontrak per entity**, dengan
   route **digenerate** dari generator yang sama dengan server. Dokumentasi
   tangan untuk route adalah bentuk kegagalan #47; mengambilnya dari
   `pkg/spec`/generator membuatnya tidak bisa berbohong.

## Refactor pendukung

`GenerateUIRoutes`/`GenerateUICustomActionRoutes` dipecah menjadi per-entity
(`UIRoutesForEntity`, `UICustomActionRoutesForEntity`) supaya CLI bisa memakainya
**tanpa database** (registry butuh `db.DB`), dan kedua jalur tetap satu sumber.

## Pengecualian yang wajib benar di kontrak

| Aturan                                                                 | Kenapa mudah salah ditulis                    |
| ---------------------------------------------------------------------- | --------------------------------------------- |
| `{entity}` singular, plural hanya untuk permission/`api/v1`            | mudah tertukar                                |
| aksi `disabled: true` tidak punya route **dan** tidak punya permission | kafe mematikan `delete`/`submit` pada `order` |
| lifecycle-free → tanpa `submit`/`cancel`/`amend`                       | gating transitif                              |
| `summary` → hanya `list`+`find`                                        | proyeksi read-only                            |
| transisi state machine tanpa `impl` **tidak** punya endpoint           | diterapkan lewat `update`                     |
| field `file` → `POST`/`GET {id}/{field}`; segmen lain `404`            | bercampur dengan route aksi                   |

## File

- `docs/runtimes/06-ui-rest-contract.md` (baru) + entri di `docs/runtimes/README.md`.
- `internal/api/generator.go` — dua helper per-entity (refactor).
- `cmd/formspec/get.go` — `describeEntityHTTP()`.
- `internal/api/generator_routes_test.go` — 2 test.

## Bukti

`formspec describe entity order` → 4 route (sesuai spec kafe: `delete`+`submit`
disabled) · `TestUIRoutesForEntity_LifecycleAndDisabled` ·
`TestUICustomActionRoutesForEntity_OnlyActionsWithImpl` · `go test ./...` hijau ·
kafe `validate` 0 problem.

## Sisa

Halaman belum digenerate dari `pkg/spec` (usulan #47 poin 2); Report/Print/
dashboard dan kontrak `api/v1` di luar cakupan.

## Estimasi: **small-medium** (refactor generator + CLI + halaman + 2 test)
