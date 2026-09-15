# FormSpecExpr

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Peran

Grammar ekspresi client-side (visibility, computed value, validasi ringan) yang
wajib diinterpretasikan identik oleh semua shell. Dipakai oleh `visible_when`,
`readonly_when`, `required_when`, `compute`, dan interpolasi `title` — lihat
pemakaiannya di [`06-page-kinds.md`](06-page-kinds.md) (Form, guard Wizard/
Kanban) dan [`07-component-kinds.md`](07-component-kinds.md).

**Garis bahasa:** ekspresi deklaratif = FormSpecExpr; kode frontend imperatif =
JS/TS lewat component contract
([`07-component-kinds.md`](07-component-kinds.md) §4). Starlark penuh di
browser ditolak — FormSpecExpr sengaja cuma subset ekspresi, bukan bahasa
lengkap.

## 2. Grammar

Subset **ekspresi** Starlark: literal, referensi field (`fields.x`),
perbandingan, `and`/`or`/`not`, aritmetika, `len`, `sum`, list comprehension.
**Tidak ada** definisi fungsi, loop, import, atau akses `ctx`. Implementasi
**wajib** menolak konstruk di luar ekspresi ini saat `formspec validate` — bukan
diam-diam diterima lalu gagal saat runtime.

Diimplementasikan sebagai **AST interpreter kecil di JS** di sisi renderer —
tanpa transpilasi, tanpa build step. Satu grammar dipakai bersama guard
sisi-server (`conditions` di action, [`../backend/01-core-basic.md`](../backend/01-core-basic.md)
§5) supaya model mentalnya satu — sandbox-nya yang berbeda (browser vs
Starlark sandbox server).

## 3. Konteks Evaluasi

Scope data yang tersedia bergantung tempat evaluasi dipanggil:

- **Form field** (`visible_when`/`readonly_when`/`required_when`/`compute`
  pada field) — `fields.*` (state form saat ini, termasuk field yang belum
  disimpan).
- **Judul/interpolasi** (mis. judul Page `"Order {order.number}"`) — record
  yang sedang ditampilkan.
- Guard Wizard/Kanban (§ [`06-page-kinds.md`](06-page-kinds.md)) — `stepData`
  akumulatif (Wizard) atau field record (Kanban).

Identitas/permission caller **tidak** termasuk konteks evaluasi FormSpecExpr —
visibilitas berbasis permission ditentukan katalog permission dari Spec
Resolution API ([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §4),
bukan lewat ekspresi ini. FormSpecExpr murni untuk kondisi _bisnis_, terpisah
dari mekanisme permission.

## 4. Determinisme & Batasan

Dievaluasi di browser — **UX saja, tidak pernah otorisasi dan tidak pernah
validasi final**; keduanya tetap wajib server-side (rules field dari
Entity manifest ditegakkan client-side untuk UX, server-side tetap otoritas
— [`06-page-kinds.md`](06-page-kinds.md) §Form). Tanpa side effect (murni
fungsi dari data yang tersedia di §3 ke nilai) — tidak ada mutasi state,
tidak ada pemanggilan action, tidak ada I/O. Kompleksitas dibatasi oleh
grammar-nya sendiri (§2, tanpa loop/fungsi) — tidak ada batas eksekusi
terpisah yang perlu dideklarasikan seperti sandbox Starlark server
([`../backend/02-core-extended.md`](../backend/02-core-extended.md) belum
membahas ini eksplisit — evaluasi FormSpecExpr cukup ringan by construction).
**Perilaku error evaluasi (normatif).** Referensi ke field yang tidak ada
adalah **error, bukan fail-safe**. Penegakannya dua lapis:

1. **Deploy-time (wajib).** `formspec apply`/`formspec check` melakukan validasi
   statis seluruh FormSpecExpr terhadap skema Entity/Page yang dirujuknya —
   referensi field yang tidak ada, member access yang tidak valid, atau
   identifier di luar konteks §3 adalah **validation error yang menggagalkan
   apply**. Ekspresi yang lolos apply dijamin seluruh referensinya resolvable,
   sehingga error kelas ini tidak mungkin terjadi di runtime.
2. **Runtime (defensive).** Kalau evaluasi tetap gagal di runtime (data
   korup, bug renderer), itu **bug framework** — renderer wajib menampilkan
   error state yang kentara (bukan diam-diam mengevaluasi ke `false`/kosong)
   dan melaporkannya, supaya kegagalan tidak menyaru sebagai perilaku UI yang
   sah.

## 5. Aritmetika & `money` (Normatif)

Nilai `money` adalah objek `{amount, currency}` ([`../backend/05-field-types.md`](../backend/05-field-types.md)
§2), tetapi **operandnya adalah nilai itu sendiri** — tidak ada sintaks
pembungkus. Operand diklasifikasikan menjadi **money** atau **skalar**, dengan
tabel yang identik dengan aturan server (§2.1 dokumen itu):

| Ekspresi                  | Hasil   | Syarat                                         |
| ------------------------- | ------- | ---------------------------------------------- |
| `m + m`, `m - m`          | `money` | mata uang sama; berbeda → error                |
| `m * n`, `n * m`, `m / n` | `money` | `n` skalar; pembagi 0 → error                  |
| `m1 / m2`                 | number  | rasio; mata uang sama                          |
| `-m`                      | `money` |                                                |
| `m1 <op> m2`              | boolean | mata uang sama                                 |
| `m1 == m2`, `m1 != m2`    | boolean | kesetaraan nilai (deep), bukan identitas objek |
| `amount(m)`               | number  | ekstraksi eksplisit komponen jumlah            |
| `currency(m)`             | string  | kode ISO-4217                                  |
| `sum([m…])`               | `money` | semua elemen money & securrency                |

Contoh nyata (aplikasi kafe): `visible_when: "fields.tendered >= fields.amount"`
(kembalian hanya masuk akal bila uang cukup), `compute: "fields.tendered - fields.amount"`.

**Operand tidak sah = error, bukan `0`.** Objek non-money, list, dan string
non-numerik **tidak** dikoersi menjadi `0`; evaluasi menghasilkan warning
sehingga pemanggil (mis. `evalCompute`) memperlakukannya sebagai gagal (§4).
Sebelum aturan ini ditetapkan, `fields.tendered - fields.amount` atas dua objek
money menghasilkan `0` secara diam-diam — kembalian yang salah tanpa jejak.

**Money vs angka mentah.** `money` tidak bisa digabung atau dibandingkan
langsung dengan angka (`fields.total > 100`) — angkanya tidak punya satuan.
Ekstrak dulu: `amount(fields.total) > 100`. Bandingkan money dengan money
(`fields.total > fields.limit`) bila yang dimaksud adalah perbandingan uang.

Agregasi laporan/widget mengikuti aturan yang sama: `sum`/`avg`/`min`/`max` atas
field money menjumlahkan komponen `.amount`-nya, dan field non-numerik ditolak
dengan pesan yang terlihat — bukan total `0`. Gerbang statisnya ada di
`formspec check`.
