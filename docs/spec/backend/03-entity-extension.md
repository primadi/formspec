# Entity Extension

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Model Extension
Module lain menambah field/perilaku ke entity yang sudah ada tanpa memodifikasi
module pemilik.

## 2. Kontrak Uninstall Bersih
Extension wajib bisa di-uninstall tanpa sisa. Ini kontrak; *cara* mencapainya
(mis. strategi kolom) adalah urusan masing-masing PersistBackend.

## 3. Konflik & Presedensi
Aturan saat dua extension menyentuh entity yang sama.

## 4. Extension dan Permission
Permission field hasil extension.
