# Entity Natural-Key Routing

## Tujuan

Gunakan `natural_key` sebagai identifier publik utama untuk URL entity dan breadcrumb. Gunakan UUID primary key sebagai fallback bila entity tidak memiliki natural key atau nilai natural key pada record kosong. Field `unique` biasa tidak dipilih otomatis.

## File dan dependensi

1. Tambahkan helper frontend untuk memilih dan meng-encode route identifier dari `EntitySchema` + record.
2. Ubah `TableRenderer` dan `DetailPage` agar seluruh URL/link/action entity menggunakan identifier yang sama.
3. Tambahkan route-identity store ringan agar shell dapat mengganti breadcrumb UUID dengan natural key setelah detail record selesai dimuat tanpa request kedua.
4. Ubah `SideNavShell` dan `TopNavShell` agar memakai resolver breadcrumb bersama.
5. Tambahkan validasi backend bahwa `natural_key` **implisit unique** (validator men-set nilainya, bukan mewajibkan penulisan `unique: true`), dan `required` tidak diimplikasikan karena nilai boleh dibuat generator atau dipasok seed. Beserta unit test.
6. Tambahkan test frontend dan persistence/API untuk natural-key URL, UUID fallback, URL encoding, dan backward compatibility UUID.
7. Perbarui dokumentasi routing dan todo/changelog.

## Keputusan kontrak

- Maksimal satu field `natural_key` per entity.
- `unique` tetap constraint, bukan selector URL.
- `natural_key` mengimplikasikan `unique`; `unique: false` eksplisit ditolak.
- `natural_key_entry` (`auto_generated` | `user_entry` | `auto_generated_if_empty`) menyatakan siapa pemasok nilai; absen diresolusi konvensi agar deklarasi lama tetap bermakna.
- `auto_generated` mengabaikan nilai dari caller dan tidak muncul sebagai input di form — dipakai `cafe-order/order.number` dan `cafe-stock/purchase-order.number`.
- `required` adalah keputusan author, bukan implikasi engine.
- Pada key opsional, nilai kosong berarti "belum diisi" sehingga index unik melewatkannya.
- UUID URL lama tetap valid.
- Nilai natural key harus di-encode sebagai satu path segment.
- Natural key yang dipakai sebagai URL sebaiknya immutable; perubahan mutable memerlukan kebijakan redirect/alias terpisah.

## Urutan verifikasi

1. Test helper identifier.
2. Test route identity store/resolver dan shell breadcrumb.
3. Test persistence/API lookup UUID dan natural key.
4. `npm run build` pada renderer.
5. Validasi manifest Kafe dan regresi route terkait.

Effort: medium. Referensi: `docs/spec/01-overview.md`, `docs/spec/02-core-basic.md`, dan `docs/renderers/shadcn-shell/05-routing.md`.
