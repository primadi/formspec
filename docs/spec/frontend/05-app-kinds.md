# Katalog Kind — Tier App

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Semantik Tier App
App renderer menentukan asumsi bootstrap subtree: chrome penuh (Auth-wrap, menu
persisten, header) vs minimal (publik, tanpa Auth-wrap).

## 2. `sidebar-nav`
Chrome penuh dengan navigasi samping; binding menu App; responsive/mobile.

## 3. `topnav`
Chrome penuh dengan navigasi atas.

## 4. `landing-page`
Bootstrap minimal untuk halaman publik (mis. pendaftaran pasien publik,
pemesanan tiket) — tanpa Auth-wrap, tanpa menu persisten.

## 5. Theme Binding
Relasi App renderer dengan Theme.

## 6. Menambah App Kind Baru
Lewat VisualSpecKind `tier: app`.
