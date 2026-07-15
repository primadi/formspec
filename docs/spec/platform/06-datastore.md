# Datastore

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Kind Datastore
Deklarasi koneksi penyimpanan (SQL, KV, objek, antrian) sebagai kind; driver
yang didukung.

## 2. Relasi dengan PersistBackend
Datastore menyediakan koneksi; PersistBackend (spec backend §04) adalah
implementasi kontrak penyimpanan entity yang **mengonsumsi** sebuah Datastore.

## 3. Lifecycle dan Kredensial
Registrasi, rotasi kredensial, health.

## 4. Scoping
Datastore per workspace/app/module; aturan berbagi.
