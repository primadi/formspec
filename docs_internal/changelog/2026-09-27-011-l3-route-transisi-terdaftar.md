# 2026-09-27-011 — L3 tuntas: transisi ber-`impl` benar-benar terdaftar sebagai route

**Apa:** Menutup celah terakhir L3 (`docs_internal/plan/via-sebagai-action-penuh.md`).
`GenerateCustomActionRoutes` dan `UICustomActionRoutesForEntity` sudah lama
menghasilkan `RouteDescriptor` untuk transisi ber-`impl`, tetapi **registrasi
handler**-nya belum ikut: `registerRouteWithPattern` (`internal/api/router.go`)
dan `generatePrepareRoutes` (`internal/api/generator.go`) masih memindai
`EntitySpec.Actions` langsung. Karena `ActionSources()` **mensintesis** `via`
tanpa menuliskannya kembali ke `es.Actions`, penelusuran itu mengembalikan nil →
registrasi dilewati → permintaan jatuh ke handler file generik.

**Kenapa:** Tanpa ini, migrasi L4 (menghapus deklarasi ganda) akan **mematikan
route** alih-alih merapikan manifest. Ini prasyarat keras L4.

**Bukti (server hidup, `POST /kafe/_ui/entity/gl/journal-entry/1/{action}`):**

| action                              | sebelum                                      | sesudah                                              |
| ----------------------------------- | -------------------------------------------- | ---------------------------------------------------- |
| `post` (hanya transisi)             | **404** `no such file field or action: post` | **403** `missing permission: gl.journal-entrys.post` |
| `reverse` (hanya transisi)          | **404**                                      | **403** `…journal-entrys.reverse`                    |
| `submit` (kontrol, deklarasi ganda) | 401 anonim / 403 auth                        | tidak berubah                                        |

Petunjuk kuncinya: pesan 404 menyebut **field FILE**, artinya route dimiliki
handler file. `403` pada `post` setara `submit` membuktikan transisi itu kini
punya route **dengan gate transisinya**.

**Dampak:** `grep "EntitySpec.Actions" internal/api/router.go` = kosong. Tiga
situs kini memakai `b.registry.GetActionSpec(...)` (union) — dua cabang `prepare`
(`/api/v1` dan `/_ui`) dan satu cabang `custom` di `registerRouteWithPattern`.

**File:** `internal/api/router.go`, `internal/api/generator.go`,
`internal/api/transition_route_registration_test.go` (baru),
`docs_internal/plan/via-sebagai-action-penuh.md`.

**Kalibrasi:** guard menguji **status code** (404 vs 403/401), bukan "bukan 200";
saat cabang `custom` dikembalikan ke penelusuran langsung, test gagal dengan
404 yang persis sama dengan bug live. `go test ./internal/... ./pkg/...` hijau.

**Pengukuran untuk L4:** 83 deklarasi ganda (`via` yang juga punya entri
`actions[]`) dan 2 transisi ber-`impl` tanpa entri `actions[]` (keduanya di
`examples/kafe/spec/modules/gl/entities/journal-entry.yaml`). Setelah L3,
kategori kedua ini sudah punya route.

→ L4 (validator anti-duplikat + migrasi 83 deklarasi) sekarang bisa jalan.
