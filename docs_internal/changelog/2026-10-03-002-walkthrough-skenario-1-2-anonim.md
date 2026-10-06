# 2026-10-03-002 — Walkthrough 9.4 skenario 1–2 sebagai tamu ANONIM; 3 temuan (1 major)

**Apa yang dilakukan.** Menjalankan rantai pelanggan dari nol **sebagai anonim
murni** (tanpa token, lewat permukaan `kafe-qr`) pada dev server `:8080` dengan
DB hasil `make seed-kafe`, lalu melengkapinya di sisi kasir/KDS. Tujuannya
memverifikasi jalur yang **benar-benar dilalui pelanggan**, bukan jalur yang
dibuat oleh test:

1. resolve meja dari token QR → 2. katalog + harga cabang → 3. buka sesi meja →
2. kirim pesanan → 5. kasir catat pembayaran + tandai lunas → 6. KDS → 7. jurnal.

Tidak ada perubahan kode; yang dicatat adalah **bukti**, **tiga temuan**, dan
**satu koreksi cakupan klaim lama**.

**Hasil (terukur).**

| Langkah                                                                              | Hasil                                                                             |
| ------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| `GET dining-table/JKT-A01-DEMO` (anon)                                               | **200** — natural key 10.34b terbukti hidup                                       |
| token tak dikenal                                                                    | **404**                                                                           |
| katalog `menu-category`/`menu-item` (anon)                                           | **200** (4 kategori, 9 menu)                                                      |
| `menu-item-price` TANPA `branch_id`                                                  | **403** (scope, bukan kebocoran)                                                  |
| `POST table-session` (anon)                                                          | **201**                                                                           |
| sesi kedua di meja sama                                                              | **409** `CONFLICT` (10.34c)                                                       |
| `POST order` (anon, **tanpa angka uang**)                                            | **201**; `subtotal` 50000 & `line_total` dihitung server; `total_amount` **null** |
| `list` order anonim tanpa token                                                      | **403**                                                                           |
| `list` dengan token sendiri / token orang lain                                       | **1 baris** / **0 baris**                                                         |
| `PATCH` status order oleh tamu                                                       | **401**                                                                           |
| entity di luar allowlist (`shift`, `cash-movement`, `payment`, `member`, `employee`) | **401** (semua)                                                                   |
| `POST payment` QRIS `settled` (kasir)                                                | **201**                                                                           |
| `PATCH order awaiting_payment → paid`                                                | **200**                                                                           |
| meja `available → occupied`                                                          | **~2 s** (subscription `table-occupancy`, 10.40b)                                 |
| filter KDS `status[in]=paid,in_kitchen,ready`                                        | **2 baris**; `status[in]=ready` → 1 baris                                         |
| barista `paid → in_kitchen → ready`                                                  | **200 / 200**                                                                     |

Jalur anonim dan scoping token tamu berperilaku persis seperti yang
dideklarasikan. **Satu koreksi metode:** percobaan pertama saya membaca filter
KDS sebagai kosong dan hampir melaporkannya sebagai bug — penyebabnya kutipan
`[` di shell yang termakan, bukan server. Diuji ulang lewat skrip: filter
bekerja benar. Dicatat supaya tidak terulang.

**Temuan baru → item bernomor di `examples/kafe/gaps_found/TODO.md`.**

- **10.65 ⏸️ (MAJOR) — pesanan dari jalur QR nyata tidak pernah menghasilkan
  jurnal GL.** `order.total_amount` tidak pernah terisi (form QR tidak mengirim
  angka uang — memang desainnya; rantai diskon/pajak belum diturunkan), sedangkan
  `gl/journalize.star` menuntutnya. Terukur: `ORD-2026-00001` → outbox
  `deliver failed … FORMSPEC.GL.NO_AMOUNT` → retry 6× → `failed` → **nol jurnal**.
  **Kausal dibuktikan dengan kontrol:** order kedua dari sesi yang sama, hanya
  menambahkan `total_amount` → `JRN-2026-000001` **posted**, 2 baris seimbang
  (Kas debit 25000 = Omzet kredit 25000), 2 `gl-balance` terisi.
  **Koreksi cakupan klaim:** "skenario 8 ✅" (2026-09-21) benar untuk order yang
  diuji, tetapi order itu ditulis oleh **test** yang menyuplai `total_amount`
  (`o2c_e2e_test.go:170`, `kafe_table_lifecycle_e2e_test.go`). Cakupan yang diuji
  ≠ cakupan yang diklaim — dampaknya jalur penjualan utama aplikasi tidak punya
  jurnal sama sekali.
- **10.66 ⏸️ — `default` tidak diterapkan pada field CHILD, sementara `computed`
  diterapkan.** Terukur: `lines[].line_status` (`default: queued`) **absen** dari
  baris tersimpan. Bukti kode: `applyDefaults` hanya mengiterasi `s.fields`
  (induk); `evaluateComputed` langkah 1 **memang** mengiterasi `f.Child.Fields`.
  Dua perlakuan berbeda tanpa alasan berbeda.
- **10.67 ⏸️ — aturan bisnis #1 ditegakkan kolom Kanban, bukan API.** Terukur:
  barista (`kafe-kds`, hanya `list`+`view`) dapat `find`/`list` order `draft`
  (`ORD-2026-00003`) — lengkap dengan `guest_token` (kunci akses tamu),
  `dining_table_id`, `guest_note`. Kelas yang sama dengan GAP-08; bedanya aturan
  #1 ditulis "tanpa pengecualian" di `docs/overview.md`.

**Dampak.** `examples/kafe/gaps_found/TODO.md` — baris 9.4 skenario 1 & 2
diperbarui dengan re-pengukuran anonim; catatan ⚠️ baru di bawah tabel 9.4
(termasuk koreksi cakupan skenario 8); tiga item `[⏸️]` baru **10.65/10.66/10.67**.
Tidak ada berkas kode yang berubah.

**Referensi.** Walkthrough 9.4; item terkait: 10.11 ⏸️ (menu vs grant),
10.46 ⏸️ (gate tidak berlaku di jalur script), GAP-08 (penyaring cabang KDS).
