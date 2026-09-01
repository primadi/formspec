# 2026-08-31-008 — Buat Form Signup Vendor menjadi Private

## Apa

Mengubah halaman dan formulir pendaftaran vendor di Registry dari bersifat publik (dapat diakses anonim) menjadi _private_ (wajib login).

Menambahkan `public: false` ke blok `spec:` pada _manifest_ berikut:

1. `registry/spec/modules/portal/pages/vendor-signup.yaml`
2. `registry/spec/modules/portal/forms/vendor-signup.yaml`

## Kenapa

Sesuai rancangan _secure by default_, pendaftaran vendor (meng-klaim _public key_ yang akan digunakan pada setiap publikasi _module_) harus dihubungkan dengan user/akun yang mendaftarkannya (otentikasi).

Meskipun saat ini kolom pengisian nama akun (`owner_username`) masih harus diketik manual oleh _user_, mengunci halamannya ke `public: false` mencegah pendaftaran dari spammer anonim dan menuntun langkah _user_ secara logis (_harus bikin akun dulu, baru daftar identity_). File navigasi `AuthArea.tsx` dan `shell` FormSpec akan secara otomatis me-redirect pengunjung `vendor-signup` yang belum login ke form Sign-in dan Sign-up.

## File terdampak

- `registry/spec/modules/portal/pages/vendor-signup.yaml`
- `registry/spec/modules/portal/forms/vendor-signup.yaml`

## Rencana Selanjutnya (Deferred)

Implementasi _auto-fill_ atau penguncian kolom Username / identity dengan Data Store dari Backend (agar `owner_username` otomatis mengambil nilai `_meta/me` dari sisi UI maupun Backend Engine FormSpec).
