# Temuan intake anonim kafe (`/kafe`) — 10.34/10.35/10.36

**Tipe:** pencatatan temuan (tanpa perubahan kode).
**Pemicu:** pertanyaan pemilik saat menyelidiki `/kafe` — (1) guna
`public_entities` + `modules` di `kafe-qr` dan kaitannya ke landing page;
(2) pemilih meja: salah pilih meja terpakai → melihat pesanan orang lain, dan
orang iseng memesan di semua meja.

**Plan/catatan:** `docs_internal/plan/kafe-intake-anonim-temuan.md`
**Ledger:** `examples/kafe/gaps_found/TODO.md` **10.34 ⏸️, 10.35 ⏸️, 10.36 ⏸️**
(bagian Fase 10). **Terkait:** 10.20 ⏸️ (landing/`DefaultRedirect`), 2.15 ⏸️
(halaman masuk token), 10.5a ⏸️.

## Ringkas

Tidak ada kode yang berubah. Tiga item baru dicatat karena temuan berikut
**tidak** tertutup oleh gap yang sudah ada:

1. **`public_entities` memang menggerbangi dengan benar, tetapi tidak ada
   kaitannya dengan landing page.** Terukur: `list dining-table` / `list
table-session` anonim → **401** (granted hanya `find`/`find+create`),
   sedangkan `list menu-item` → 200. Yang membuat `/kafe` 404 adalah
   `DefaultRedirect` memilih entity pertama **tanpa memeriksa
   `authorized_actions`** → jatuh ke `cafe-master/dining-tables` yang tidak
   punya rute turunan bagi anonim. Itu **10.20 ⏸️** (bagian kedua).
2. **`POST table-session` anonim dijawab 422, bukan 403** — request lolos
   otorisasi; hanya validasi relasi yang menghentikannya. Tanpa halaman masuk
   token, satu request bisa mengklaim meja siapa pun (**10.34**).
3. **Kapabilitas penahan sudah ada tetapi tidak dipakai:** `EntitySpec.
RateLimit`/`Action.RateLimit` (0 manifest kafe memakainya → **10.36**),
   `IndexDecl.Where` parsial (dipakai `shift`, **tidak** dipakai
   `table-session` → 10.34c), dan `public_entities[].scope` (sudah dipakai
   `order`, menolak `list` anonim tanpa `guest_token` → 403).

## Sisa (item bernomor)

- 10.34 ⏸️ — klaim meja tanpa token + token tidak bisa di-`find` + tidak ada
  keunikan sesi per meja. Effort: small (reda oleh 10.35).
- 10.35 ⏸️ — halaman masuk token → sesi. Effort: medium.
- 10.36 ⏸️ — `rate_limit` intake anonim. Effort: small.
- 10.20 ⏸️ (lama) — landing page; bentuknya masih menunggu keputusan
  (`home_page` per-App vs module khusus).
