# 2026-08-17-035-extension-read-write.md

## Extension read/write (4.3.1/4.3.2)

Melengkapi jalur baca/tulis runtime untuk entity extension yang sebelumnya
hanya DDL + registry (4.3.3–4.3.5). Sekarang data extension benar-benar
dipisahkan ke kolom `ext_{namespace}` dan digabungkan kembali saat baca.

**Apa yang diubah:**

- `renderers/jsonb-persist/crud.go`:
  - `validateKnownFields` menerima key namespace extension sebagai input
    write yang sah (sebelumnya ditolak sebagai unknown field).
  - `Insert` menulis payload namespace ke kolom `ext_{namespace}` setelah
    insert baris (sebelumnya `splitExtensions` dipanggil tapi hasilnya tidak
    pernah ditulis).
  - `Update` memanggil `splitExtensions` dan menulis payload ke kolom
    `ext_{namespace}` dalam transaksi yang sama.
  - `mergeExtensions` (baca) sudah ada dan tetap dipakai di
    `hydrateAndCompute`.
- `internal/entity/registry.go`: `GetEntityStore` me-wire `SetExtensions`
  dengan memindai semua entity `ExtendStorage` yang menarget entity ini
  (namespace → `ext_{namespace}`), sehingga jalur baca/tulis aktif di alur
  nyata.
- `renderers/jsonb-persist/extension_test.go` (baru): `TestEntityStore_ExtensionReadWrite`
  memverifikasi insert → base data tidak terpolusi, kolom ext terisi, baca
  menggabungkan kembali, dan update memperbarui kolom ext.

**Kenapa:** menutup gap bahwa extension column dibuat tapi tidak pernah
dibaca/ditulis runtime, sehingga fitur extension tidak berfungsi end-to-end.

**Referensi:** `docs/plan/todo.md` 4.3.1/4.3.2, `docs/spec/backend/03-entity-extension.md`.
