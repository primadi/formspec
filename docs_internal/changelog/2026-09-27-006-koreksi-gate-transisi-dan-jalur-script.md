# Mengapa ada transisi bergerbang dan tanpa gerbang? (koreksi + 10.46)

**Tipe:** koreksi desain + temuan (ada perubahan kecil pada manifest).
**Ledger:** kafe **10.46 ⏸️** (baru), 10.45 ⏸️ (diperluas), 10.44 ✅ (tak berubah)
**Pemicu:** pertanyaan pemilik — "mengapa ada transisi dengan
`require_permission`, ada yang tanpa? apakah karena dipicu event lain dan tidak
bisa dipanggil manual?"

## Jawaban singkat

**Bukan.** Semua transisi di meja **bisa** dipanggil manual — dari CLI,
dasbor, atau tombol. `require_permission` tidak menyatakan "hanya otomatis";

ia menyatakan **"siapa (permission apa) yang boleh menjalankannya"**.

**Premis "tanpa gate = tidak bisa dipanggil manual" salah, dan saya buktikan.**
Sebelum perbaikan ini, kasir menjalankan **semua** transisi tanpa gate → 200:

| Transisi (tanpa gate)                         | hasil   |
| --------------------------------------------- | ------- |
| `available → occupied` (via `occupy`)         | **200** |
| `served → occupied`                           | **200** |
| `occupied → served` (via `mark-table-served`) | **200** |
| `occupied → available` (via `release`)        | **200** |

Pembeda sebenarnya bukan "otomatis vs manual", melainkan:

|                                 | Arti                                                                                         |
| ------------------------------- | -------------------------------------------------------------------------------------------- |
| **tanpa** `require_permission`  | "siapa pun yang punya `update` + transisinya sah di izinkan" — **termasuk** dipanggil manual |
| **dengan** `require_permission` | "hanya pemegang permission itu" — juga termasuk dipanggil manual **oleh orang yang tepat**   |

## Koreksi yang saya terapkan

Implikasi dari jawaban itu: `mark-table-served` dan `release` seharusnya
**juga** bergerbang, karena keduanya adalah aksi peran (pelayan menyajikan,
kasir menutup). Sebelumnya keduanya terbuka untuk semua pemegang `update`.

Sekarang (terukur, dev server):

| Transisi                                   | kasir   | pelayan | manajer |
| ------------------------------------------ | ------- | ------- | ------- |
| `occupied → served` (gate pelayan)         | **403** | **200** | —       |
| `occupied/served → available` (gate kasir) | **200** | **403** | **403** |
| `available → not_available` (gate admin)   | **403** | **403** | **200** |
| `available → occupied` (**tanpa gate**)    | **200** | —       | —       |

Manajer **403** pada `release` sekarang — bukan bug: ia tidak diberi grant
`release` (kasir yang menutup meja). Kalau Anda ingin manajer bisa mengambil
alih, cukup tambahkan `release` ke grant `dining-table-page` miliknya.

## Temuan baru: 10.46 — jalur script tidak menegakkan gate

Bukti kode, bukan dugaan:

- Gate hidup di **jalur HTTP** (`internal/api/handler.go` →
  `spec.TransitionPermission`).
- `resource.save()` menulis lewat `resource/formspec.go` `SetSaveHandler` →
  `store.Update(...)` → `jsonb-persist` `validateStateTransition`.
- Handler itu memanggil `runBeforeWriteHooks` (jadi **hook** tetap jalan) tetapi
  **tidak pernah membaca `require_permission`** — `grep TransitionPermission
resource/formspec.go` → **0 hasil**.

Jadi `store.Update` memvalidasi **legalitas** transisi (dari/to terdaftar),
**bukan siapa** yang menjalankannya. Konsekuensi:

- `gl/scripts/journal_post.star` dan `order.void-order` menempuh jalur ini
  (`resource.set("status", …)` + `save()`), sehingga gate transisi tidak menahan
  script.
- Untuk **meja** belum eksploitatif (tidak ada script seperti itu), tetapi
  **10.40b** (subscription `on_paid` → `occupied`) adalah penulis script pada
  entity yang sama.
- **UI juga belum menghormati gate**: `PATCH` dari dasbor mengirim **seluruh**
  field, sehingga field status yang dikendalikan state machine bisa ikut
  ter-PATCH tanpa maksud pengguna.

Ini kelas yang sama dengan **10.43** dan **10.45**: gerbang yang hidup di satu
lapisan (HTTP/UI) bukan batas keamanan sampai ditegakkan di lapisan tulis.

## Perubahan file

`examples/kafe/spec/modules/cafe-master/master/dining-table/entity.yaml`
(`require_permission` + `mark-table-served` + `release`),
`examples/kafe/spec/modules/formspec.core/seeds/roles.yaml` (kasir `+release`,
pelayan `+mark-table-served`).

Kafe `validate` 85 manifest 0 problem · `gofmt` bersih · `go test ./...` hijau.
