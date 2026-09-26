# Natural key: tiga mode entry, required sebagai pilihan author

Melanjutkan `2026-09-25-009`. Kontrak natural key kini:

1. **`unique` implisit** — `natural_key: true` sudah berarti unik; `unique: false` eksplisit ditolak (butuh presence flag `Field.uniqueSet`, karena `bool` polos tidak bisa membedakan "absen" dari "false").
2. **`required` bukan implikasi engine** — keputusan author. Percobaan pertama sempat mengimplikasikannya dan mematahkan 12 manifest yang sah (seed/`required` tidak pernah diisi user).
3. **`natural_key_entry`** (`auto_generated` | `user_entry` | `auto_generated_if_empty`) — properti baru yang menyatakan siapa pemasok nilai, karena dua situasi itu menuntut UI yang berlawanan: nomor yang di-generate tidak boleh ditanyakan, sedangkan kode yang diisi user tidak boleh ditimpa diam-diam. Absen diresolusi konvensi (`ResolveNaturalKeyEntry`): tanpa rule → `user_entry`; `strategy: custom` → `user_entry`; `strategy: sequence` → `auto_generated_if_empty`. Konvensi ini yang membuat 41 deklarasi `natural_key` yang sudah ada tetap sah tanpa edit, termasuk `order.number` dan `purchase-order.number` yang memang mengizinkan kasir memakai nomor kertas.

Perilaku engine per mode (uji: `natural_key_entry_test.go`): `auto_generated` **mengabaikan** nilai dari caller; `auto_generated_if_empty` memakai nilai caller dan hanya meng-generate bila kosong; `user_entry` tidak pernah meng-generate.

Pada key yang **opsional** (`required` tidak diset), nilai kosong berarti "belum diisi" — bukan sebuah nilai. Index uniknya karena itu melewatkan baris kosong (`_code IS NOT NULL AND _code != ''`), karena tanpa itu dua record tanpa kode ditolak `UNIQUE constraint failed` dan mode opsional jadi tidak bisa dipakai. Bukti terukur ada di `natural_key_optional_test.go`.

Dipakai di manifest: `gl/account.code` → `user_entry` (dipasok seed, boleh diisi user); `cafe-order/order.number` dan `cafe-stock/purchase-order.number` → `auto_generated` (keduanya tidak pernah jadi input — form POS mengunci `order.number`, dan `purchase-order-form` tidak menyediakannya sama sekali; `purchase_receive.star` hanya MEMBACA nomor PO). Uji kontrak pada entity nyata: `resource/order_number_entry_e2e_test.go`.

Sekalian dirapikan: `unique: true` yang redundan pada deklarasi `natural_key` di contoh kafe (6 file) dihapus, karena `natural_key` sudah mengimplikasikannya. Sisa 19 deklarasi serupa di `verticals/`, `examples/Clinic-UI-Showcase/`, dan `cmd/formspec-registry/` belum disentuh (tidak memblokir, hanya duplikasi).

Dampak lanjutan yang ikut ditutup:

- `lib/field-presence.ts` (frontend) — `natural_key_entry` dibaca dengan konvensi yang sama seperti validator Go, sehingga `auto_generated` **tidak muncul sebagai input** di form (nilainya akan dibuang server) sedangkan dua mode lain tetap input.
- `internal/api/handler.go` — jalur find-or-create reference entity tidak lagi membuat record ketika `id` kosong, dan test-nya menyetel path value `{id}` seperti route produksi (sebelumnya membuat record dengan natural key kosong).

File: `pkg/spec/entity.go`, `pkg/spec/entity_test.go`, `renderers/jsonb-persist/{crud.go,ddl.go}` + 2 test, `renderers/react-shadcn/src/lib/field-presence.ts` + test, `derive.ts`, `zod-schema.ts`, `FormRenderer.tsx`, `headless-form.ts`, `internal/api/handler.go`, `handler_settings_seed_test.go`, `resource/order_number_entry_e2e_test.go`. Referensi plan: `docs_internal/plan/entity-natural-key-routing.md`.
