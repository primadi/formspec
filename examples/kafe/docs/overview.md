# Overview — Aplikasi Kafe

> Dokumen ini ditulis dalam bahasa bisnis (tanpa istilah teknis) dan menjadi
> dasar seluruh perancangan aplikasi. Silakan dibaca dan dikoreksi oleh pemilik
> bisnis sebelum kita lanjut ke tahap perancangan teknis.

## Apa Itu Aplikasi Kafe

Aplikasi ini adalah **sistem operasional kafe multi-cabang**. Ia menangani dua
sisi sekaligus:

1. **Sisi pelanggan** — pelanggan duduk di meja, memindai QR code yang tertempel
   di meja, melihat menu, memesan, dan membayar. Tidak perlu antre, tidak perlu
   mengunduh aplikasi.
2. **Sisi kafe** — kasir, barista, dan manajer menjalankan operasional harian:
   menerima pesanan, membuat minuman/makanan, mengelola menu, stok bahan baku,
   kas, promo, dan laporan.

Karena kafe punya beberapa cabang, semua data (menu, stok, penjualan, kas)
dipisahkan per outlet namun tetap dapat dilihat dan dibandingkan oleh pemilik
dari satu tempat.

## Tujuan Bisnis

1. **Mempercepat pelayanan** — pelanggan memesan sendiri dari meja lewat QR code,
   sehingga tidak perlu menyetop pelayan atau antre di kasir.
2. **Menjamin pesanan sudah dibayar sebelum dibuat** — dapur hanya menerima
   pesanan yang sudah lunas, sehingga tidak ada pesanan "nyangkut" yang tidak
   terbayar.
3. **Mengontrol stok bahan baku secara otomatis** — setiap menu punya resep;
   saat terjual, pemakaian bahan tercatat dan stok berkurang sendiri.
4. **Menjaga uang kas tetap akurat** — setiap kasir bekerja dalam shift dengan
   kas awal, hitungan fisik saat tutup shift, dan pencatatan selisih.
5. **Mengendalikan diskon dan pembatalan** — promo diatur sebagai aturan resmi,
   sedangkan pembatalan transaksi harus disetujui supervisor.
6. **Mengenal pelanggan** — pelanggan yang mendaftar mengumpulkan poin dan
   mendapat keuntungan khusus, sehingga mereka kembali lagi.
7. **Memberi pemilik visibilitas penuh** — penjualan, menu terlaris, margin,
   pemakaian bahan, kas, dan loyalitas pelanggan dalam satu dashboard.

## Pengguna dan Perannya

| Peran | Siapa | Yang bisa dilakukan |
| --- | --- | --- |
| **Pemilik** | Pemilik kafe | Melihat data & laporan seluruh outlet, mengatur menu, harga, promo, dan pengguna |
| **Supervisor / Manajer Outlet** | Kepala outlet | Menyetujui pembatalan transaksi & diskon di luar batas, menutup shift, mengelola stok & pembelian |
| **Kasir** | Staf kasir | Membuat dan membayar pesanan, menerima pembayaran tunai, buka/tutup shift, kas masuk/keluar |
| **Barista / Dapur** | Staf produksi | Melihat daftar pesanan yang sudah lunas, menandai sedang dibuat dan siap disajikan |
| **Pelanggan** | Pengunjung kafe | Memindai QR meja, melihat menu, memesan, membayar, melihat status pesanan |
| **Pelanggan Member** | Pengunjung terdaftar | Seperti pelanggan biasa, ditambah akumulasi poin dan penukaran poin |

## Alur Bisnis Utama

### 1. Pelanggan memesan dari meja (QR code)

```
Pelanggan duduk di meja
   │
   ├─ Scan QR code meja → halaman menu kafe (tanpa login)
   │
   ├─ Pilih menu → keranjang → isi jumlah
   │
   ├─ Isi nama (opsional). Kalau mau dapat poin, masukkan nomor HP → jadi member
   │
   └─ Pilih cara bayar:
         ├─ Bayar sekarang (QRIS / e-wallet / kartu) → lunas otomatis
         └─ Bayar di kasir (tunai) → pesanan berstatus "menunggu pembayaran"
   │
   ▼
Hanya pesanan yang SUDAH LUNAS yang otomatis muncul di layar dapur
   │
   ▼
Barista menandai "sedang dibuat" → "siap" → pelayan mengantar → "selesai"
```

**Aturan penting:** pesanan tidak pernah sampai ke dapur sebelum lunas.
Untuk pesanan "bayar di kasir", pelanggan menyebutkan nomor meja atau kode
pesanan kepada kasir, kasir menerima uang tunai, dan pesanan baru diteruskan
ke dapur setelah kasir menekan "Lunas".

### 2. Pelanggan membayar di kasir (tunai)

```
Pelanggan memesan dari QR → memilih "bayar di kasir"
   │
   ▼
Pelanggan ke kasir, sebut nomor meja / kode pesanan
   │
   ▼
Kasir buka pesanan → terima uang → hitung kembalian → tekan "Lunas"
   │
   ▼
Pesanan otomatis masuk ke dapur
```

### 3. Kasir melayani pesanan langsung (walk-in)

Pelanggan yang memesan langsung di depan kasir (tanpa QR) tetap dilayani:
kasir memilih menu, membuat pesanan, memilih meja atau bungkus/takeaway, dan
langsung memproses pembayaran. Alurnya sama setelah itu.

### 4. Operasional harian kasir (shift)

```
Buka shift: kasir memasukkan kas awal (jumlah uang di laci)
   │
   ▼
Transaksi berjalan sepanjang shift (tunai, kartu, QRIS)
   │
   ├─ Kas masuk / kas keluar (mis. beli es batu, setor ke bank) dicatat
   │
   ▼
Tutup shift: kasir menghitung fisik uang di laci dan memasukkannya
   │
   ▼
Sistem menampilkan selisih (seharusnya vs kenyataan) → supervisor menutup shift
```

### 5. Pembatalan transaksi (void)

Kasir **tidak bisa** membatalkan transaksi sendirian. Kasir mengajukan
pembatalan dengan alasan, lalu supervisor menyetujui. Setelah disetujui,
transaksi ditandai batal dan dicatat siapa yang membatalkan dan mengapa.
Bila pesanan sudah dibuat, bahan yang terpakai tetap dicatat sebagai kerugian.

### 6. Stok bahan baku

```
Bahan baku diterima dari supplier → catat pembelian → stok bertambah
   │
   ▼
Penjualan terjadi → sistem mengurangi stok bahan sesuai RESEP menu
   │
   ▼
Stok harian dihitung ulang (stock opname) → selisih dicatat
   │
   ▼
Stok menipis → muncul peringatan untuk segera dibeli
```

### 7. Promo dan loyalitas

- Promo dibuat sebagai **aturan resmi** oleh pemilik/supervisor, contoh:
  diskon happy hour 20% setiap Senin–Jumat pukul 14:00–17:00, atau beli 2
  gratis 1 untuk menu tertentu. Aturan ini otomatis berlaku saat pelanggan
  memesan.
- Setiap pembelian oleh **pelanggan member** menghasilkan poin. Poin dapat
  ditukar menjadi potongan harga pada pembelian berikutnya.

## Aturan Bisnis (Business Rules)

1. **Pesanan masuk dapur hanya setelah lunas.** Tidak ada pengecualian.
2. **QR code unik per meja.** Satu meja satu kode; pelanggan otomatis
   terhubung ke meja tersebut saat memindai.
3. **Pelanggan tidak wajib punya akun.** Namun tanpa nomor HP terdaftar,
   pembelian tidak menghasilkan poin.
4. **Satu pelanggan boleh membuat lebih dari satu pesanan** dalam satu
   kunjungan (mis. pesan makanan dulu, minuman kemudian).
5. **Pesanan tidak dapat diubah setelah dibayar.** Penambahan dilakukan
   dengan membuat pesanan baru.
6. **Harga dan pajak boleh berbeda antar outlet.** Pemilik mengatur harga
   per outlet, atau menetapkan harga berlaku untuk semua outlet.
7. **Promo tidak dapat digabung bebas.** Maksimal satu promo otomatis per
   pesanan, ditambah satu penukaran poin.
8. **Diskon manual di luar batas wajib disetujui supervisor.**
9. **Stok dihitung per outlet.** Bahan di cabang A tidak mengurangi bahan
   di cabang B.
10. **Satu kasir hanya boleh punya satu shift terbuka** pada satu waktu,
    dan satu outlet hanya boleh punya satu shift terbuka per kasir.
11. **Pembatalan transaksi setelah shift ditutup tidak dimungkinkan.**
    Selisihnya diperlakukan sebagai penyesuaian pada hari berikutnya.
12. **Poin diberikan setelah transaksi lunas**, bukan saat pesanan dibuat.
13. **Nomor HP member harus unik** — satu nomor HP = satu keanggotaan.
14. **Data penjualan tidak pernah dihapus permanen.** Koreksi dilakukan
    dengan pembatalan, bukan penghapusan.

## Laporan yang Dibutuhkan

| Laporan | Isi | Untuk siapa |
| --- | --- | --- |
| **Penjualan per periode** | Omzet harian/mingguan/bulanan, per jam, per outlet, per metode bayar | Pemilik, Supervisor |
| **Menu terlaris & profitabilitas** | Jumlah terjual, omzet, biaya bahan, margin per menu | Pemilik |
| **Pemakaian & stok bahan** | Bahan terpakai, stok tersisa, item kritis, selisih opname | Supervisor |
| **Rekap kas & shift** | Kas awal, penjualan per metode, kas masuk/keluar, selisih per shift | Supervisor, Pemilik |
| **Loyalitas & pelanggan** | Pelanggan baru vs kembali, poin terkumpul/terpakai, pelanggan teratas | Pemilik |
| **Pembelian & supplier** | Pembelian per supplier, harga bahan, hutang pembelian | Supervisor |

## Catatan Teknis (untuk tim)

- Dibangun dengan **FormSpec** — seluruh definisi aplikasi (API, tampilan, izin,
  alur status, event) dinyatakan sebagai spec YAML sebagai satu-satunya sumber
  kebenaran.
- Database: **PostgreSQL** untuk produksi, **SQLite** untuk pengembangan.
- Satu kafe, banyak outlet: dipisahkan sebagai tenant/outlet di dalam satu
  aplikasi, bukan aplikasi terpisah per cabang.
- **Tiga permukaan aplikasi** yang berbagi data yang sama:
  - **Aplikasi publik (QR meja)** — menu & pemesanan untuk pelanggan,
    bisa diakses tanpa login.
  - **Aplikasi privat (back-office/POS)** — kasir, supervisor, pemilik,
    master data, stok, laporan (wajib login).
  - **Layar dapur (KDS)** — layar penuh khusus tablet dapur, tanpa navigasi,
    hanya menampilkan antrean pesanan yang sudah lunas.
    *(Perlu konfirmasi: apakah layar dapur dipisah sebagai aplikasi sendiri,
    atau cukup satu halaman di dalam aplikasi privat.)*

## Keputusan yang Sudah Disetujui

1. **Harga boleh berbeda antar outlet.** Pemilik dapat menetapkan harga per
   outlet; bisa juga menyalin harga satu outlet ke semua outlet.
2. **Pajak & service charge dapat diatur dari konfigurasi** (per outlet),
   bukan ditanam di harga. Struk menampilkan keduanya terpisah bila diaktifkan.
3. **Struk digital diperlukan** — pelanggan dapat melihat rincian pesanan dan
   struk dari halaman QR-nya sendiri.
4. **Tidak perlu reservasi meja.** Meja hanya untuk pelanggan yang sudah datang.
5. **Aturan poin harus bisa diatur dari panel admin** — laju perolehan poin dan
   nilai tukarnya diubah tanpa mengubah spec.
6. **Ruang lingkup diperluas** dari draf awal:
   - **Struk cetak (thermal) masuk cakupan**, bukan hanya struk digital.
   - **Akuntansi penuh masuk cakupan** — memakai modul vertikal akuntansi yang
     sudah tersedia (jurnal, buku besar, neraca saldo), bukan hanya rekap kas.
   - **Supplier & pembelian bahan masuk cakupan.**
   - **Resep/BOM** menjadi dasar pemotongan stok bahan dan perhitungan biaya.

## Batasan & Hal yang Belum Termasuk (Fase Ini)

1. **Payment gateway otomatis penuh.** Status pembayaran QRIS/e-wallet/kartu
   diperbarui oleh kasir atau melalui integrasi terpisah, belum ada settlement
   otomatis dari penyedia. (Di luar: settlement, refund otomatis ke penyedia.)
2. **Aplikasi mobile native.** Pelanggan memakai browser (web) via QR.
3. **Integrasi perangkat keras fisik** — mesin EDC dan printer thermal
   terhubung ke perangkat kasir, bukan dikendalikan langsung oleh aplikasi
   (struk dihasilkan lalu dicetak lewat perangkat).
4. **Pesanan antar (delivery) via pihak ketiga** (GoFood, GrabFood).
5. **Reservasi/booking meja di muka.**
6. **Multi-mata-uang.** Satu mata uang per workspace.
7. **Pajak berlapis/lintas negara** — hanya satu pajak penjualan + satu
   service charge per outlet.

## Prinsip Penyusunan Spec (disetujui)

Aplikasi ini adalah **test case FormSpec untuk menyelesaikan masalah bisnis
nyata**. Karena itu spec ditulis sebagai **spec ideal** — menggambarkan
bagaimana aplikasi kafe *seharusnya* dinyatakan, dengan asumsi seluruh
keterbatasan mesin yang ditemukan di `gaps_found/` **sudah diselesaikan**.

Konsekuensinya:

- Spec ini adalah **kontrak target**, bukan deskripsi kemampuan FormSpec hari
  ini. Sebagian manifest mungkin belum bisa dijalankan sekarang.
- Setiap bagian yang bergantung pada keterbatasan mesin ditandai eksplisit
  dengan komentar `# GAP-nn:` sehingga bisa ditelusuri ke `gaps_found/`.
- `gaps_found/` berperan ganda: **daftar temuan** dan **daftar pekerjaan
  mesin** yang dibutuhkan agar spec ini berjalan.
