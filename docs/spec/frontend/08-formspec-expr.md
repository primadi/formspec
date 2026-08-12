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
bukan lewat ekspresi ini. FormSpecExpr murni untuk kondisi *bisnis*, terpisah
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
