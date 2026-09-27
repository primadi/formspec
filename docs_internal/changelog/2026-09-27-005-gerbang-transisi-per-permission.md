# `state_machine`: gerbang transisi per-permission (10.44)

**Plan:** `docs_internal/plan/kafe-qr-table-session-flow.md`
**Ledger:** kafe **10.44 ✅** · sisa → **10.45 ⏸️**, 10.42, 10.43, 10.40b, 10.41

## Masalah

Permintaan pemilik: `available ↔ not_available` oleh **admin**,
`available ↔ reserved` oleh **kasir**. Ini **tidak mungkin** sebelum perubahan
ini, dan bukan karena kurang deklarasi:

- `TransitionDecl` (`pkg/spec/entity.go`) = `{from, to, via, guard, emit}` —
  tidak ada tempat menaruh gate.
- Jalur penerap transisi adalah `PATCH`, yang diperiksa
  `{module}.{plural}.update` — **satu** permission untuk SEMUA transisi.
- `guard.expression` tidak bisa menggantikan: FormSpecExpr **melarang**
  identitas/permission (`docs/spec/frontend/08-formspec-expr.md` §3), jadi
  `user.roles`/`has(...)` tidak dapat dievaluasi.

**Terukur sebelum fix:** kasir berhasil `available → not_available` → **200**.

## Perubahan

| File                                         | Isi                                                                                                            |
| -------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `pkg/spec/entity.go`                         | `TransitionDecl.RequirePermission` (field baru) + `TransitionPermission()` + `ValidateTransitionPermissions()` |
| `internal/api/handler.go`                    | Pemeriksaan gate di jalur `PATCH`, sebelum workflow interception                                               |
| `pkg/spec/transition_permission_test.go`     | **baru** — 8 test                                                                                              |
| `examples/kafe/.../dining-table/entity.yaml` | `require_permission` pada 4 transisi                                                                           |
| `examples/kafe/.../seeds/roles.yaml`         | kasir `+reserve`; manajer `+mark-not-available`, `+mark-available`                                             |
| `schemas/**`, `docs/kind/**`                 | hasil `make generate-schema` + `generate-kind-docs`                                                            |

Kontraknya **additive dan fail-safe**: kosong = perilaku lama (`update`), jadi
nol manifest lama berubah. Validator menolak `require_permission` yang tidak
merujuk permission apa pun yang didaftarkan entity — typo tidak boleh dibuang
diam-diam, karena permission yang tidak pernah diberikan berarti transisi
terkunci selamanya.

## Bukti (dev server, terukur)

| Transisi                            | kasir   | pelayan | manajer (admin) |
| ----------------------------------- | ------- | ------- | --------------- |
| `available → not_available`         | **403** | **403** | **200**         |
| `not_available → available`         | **403** | —       | **200**         |
| `available → reserved`              | **200** | —       | **403**         |
| `available → occupied` (tanpa gate) | **200** | —       | —               |
| `occupied → served` (tanpa gate)    | —       | **200** | —               |

Pesan 403 menyebut gate + transisinya:
`missing permission: cafe-master.dining-tables.not-available (required for
transition available -> not_available)`.

`go test ./...` 39 paket hijau · `go build ./...` · `gofmt` bersih · kafe
`validate` 85 manifest 0 problem.

## Temuan penting: field baru hilang di `UnmarshalYAML`

Fix pertama **tidak berefek**. Penyebabnya:

`TransitionDecl.UnmarshalYAML` memakai **struct lokal** yang harus menyebut
setiap field secara eksplisit. `require_permission` tidak ada di sana, jadi
nilainya **dibuang saat load** — sementara `formspec validate` membaca **file**
dan tetap hijau. Kombinasi terburuk: manifest benar, validasi benar, engine
kosong.

Ini **persis** bug yang pernah menghilangkan `emit` (komentar di kode itu
mendokumentasikannya: durable `on_paid` tidak pernah sampai ke outbox).

**Test pengunci:** `TestTransitionDecl_UnmarshalKeepsRequirePermission` —
**dibuktikan gagal** saat field dihapus dari struct lokal
(`want "dining-tables.not-available", got ""`), hijau sesudahnya.

**Pelajaran:** setiap field baru pada `TransitionDecl` (dan tipe lain yang punya
`UnmarshalYAML` kustom) harus ditambahkan di **dua** tempat. Kelas bug ini mahal
karena validasi dan runtime membaca dua sumber berbeda.

## Sisa

- **10.45 ⏸️** — gate bekerja pada jalur transisi, tetapi belum diuji sebagai
  batas keamanan: `PATCH` mentah yang menyentuh field status lewat jalur
  non-transisi (script/hook `save()`) belum dipastikan tertutup.
- **10.42 ⏸️** — backfill field enum baru (baris lama → 500 `initial state must
be ...`). **Terukur lagi hari ini:** A-01/T02 kosong → 500 sampai
  diinisialisasi.
- **10.43 ⏸️** — semantik `required_permission` pada action impl-less.
- **10.40b ⏸️** (pemicu otomatis saat bayar), **10.41 ⏸️** (agregat "semua
  disajikan"), **10.34c/10.36/10.37/10.38/10.39/10.20**.
