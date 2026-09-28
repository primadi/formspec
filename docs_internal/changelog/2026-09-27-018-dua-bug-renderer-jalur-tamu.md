# 2026-09-27-018 — Dua bug renderer yang menghalangi alur tamu (ditemukan E2E)

**Apa:** Harness E2E baru (`2026-09-27-019`) menemukan **dua bug renderer nyata**
yang tidak terlihat oleh 40 test jsdom maupun `go test`, keduanya di jalur tamu
anonim. Keduanya diperbaiki di sini. **Yang penting: sebelum perbaikan ini,
"tamu memesan dari QR" tidak mungkin dijalankan di browser sama sekali.**

## Bug 1 — tombol Create tamu hilang (public surface)

`canDoEntityAction` memeriksa identitas (`me`) **sebelum** `authorized_actions`:

```ts
if (!me) return false
if (entity.authorized_actions)
  return entity.authorized_actions.includes(resource)
```

Permukaan `access: public` **boot anonim secara konstruksi** dan — sesuai desain
`session.ts` — **tidak** memalsukan identitas: `me.user_id === "anonymous"`
dibersihkan menjadi `me: null` (kalau dipalsukan, guard otorisasi bisa dilewati).
Akibatnya `!me` selalu benar untuk tamu, dan **setiap tombol create
disembunyikan** — termasuk tombol submit halaman pemesanan QR.

Terukur di Chromium lewat `/kafe/t/JKT-A01-DEMO`: form hanya merender dua field
dan **"Cancel"**; tombol submit tidak ada di DOM. `authorized_actions`-nya sendiri
benar (`['find','create']`), jadi keputusan yang salah ada di urutan pemeriksaan.

**Perbaikan:** `authorized_actions` dipercaya lebih dulu, karena ia sudah
diresolusi server **untuk pemanggil ini** — dan justru itulah satu-satunya cara
mengekspresikan grant App publik. Identitas tetap dipakai sebagai fallback (server
lama tanpa flag itu), dan **di jalur fallback itulah "fail closed tanpa identitas"
tetap berlaku** — jaminan lama tidak dihapus, hanya dipersempit ke kasus yang
memang membutuhkannya. Dua test baru mengunci keduanya
(`permissions.test.ts`: "trusts authorized_actions WITHOUT an identity" +
"applies resourceAction() before consulting authorized_actions").

## Bug 2 — `default_from` mengirim placeholder mentah (race context async)

`default_from` di-seed sekali, dengan `ctx` **sengaja** di luar dependency:

```ts
// `ctx` is intentionally excluded: it is a fresh object each render, and
// re-seeding on every resolution pass would fight the user's edits.
}, [seedKey, form])
```

Alasan itu benar untuk kasus biasa (ctx sudah siap di render pertama), tetapi
salah untuk form yang **`spec.context`-nya sendiri** mengambil record
(`source: entity`) — resolusi itu **asinkron**. Render pertama berjalan saat
`table` masih pending, `interpolateTokens` menemukan token yang belum bisa
di-resolve, dan `picker.ts` **memang sengaja membiarkannya verbatim** ("payload
menampilkan `{session.branch_id}` alih-alih blank lebih baik daripada 422
misterius"). Nilai mentah itu lalu dikirim sebagai field:

> `table-session insert: field validation failed: relation branch_id points to
cafe-master.branch[{table.branch_id}], which does not exist` — **422**

Efeknya efek yang sama seperti bug 1: form tampak benar, tombol ada, klik
berhasil, dan tidak terjadi apa-apa. Karena `ctx` tidak ada di dependency, efek
seed **tidak pernah jalan lagi** setelah context resolve.

**Perbaikan:** `ctx` masuk dependency, dan penulisannya dibatasi nilai yang
**belum terisi ATAU masih berupa token yang belum resolve** (`/^\{[\w.]+\}$/`).
Nilai yang sudah resolve tidak pernah ditulis ulang — itu yang menjaga janji lama
("tidak melawan editan user") tetap berlaku. Sebuah nilai yang tersangkut
placeholder justru **tidak** bisa dianggap "editan user".

## Temuan ketiga (spec, bukan kode) — confirm App bocor ke halaman masuk

Halaman masuk tamu mewarisi `confirm.create: "Kirim pesanan?"` milik App
`kafe-qr`, jadi tamu yang **baru memindai QR** ditanya "Kirim pesanan?" padahal
belum ada pesanan apa pun. Diperbaiki di spec: `confirm: { create: "" }` pada
`table-open-form` ("" = eksplisit off, mekanisme yang sudah ada).

**Kenapa ketiganya luput sampai sekarang:** 40 test jsdom menguji komponen
secara terpisah dengan fixture yang `me`-nya sudah ada, dan `go test` menguji
HTTP tanpa renderer. Bug 1 hanya muncul bila identitas benar-benar `null`
(kombinasi public-surface + anonymous boot), dan bug 2 hanya muncul bila
`spec.context` **dan** `default_from` dipakai bersamaan pada satu form — pola
yang justru baru dibuat oleh halaman masuk QR ini.

**File:** `renderers/react-shadcn/src/engine/permissions.ts`,
`src/engine/permissions.test.ts`, `src/kinds/form/FormRenderer.tsx`,
`examples/kafe/spec/modules/cafe-order/forms/table-open-form.yaml`
