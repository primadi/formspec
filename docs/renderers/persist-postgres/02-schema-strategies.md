# Strategi Skema

**Updated:** 2026-07-15 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap.

## 1. Strategi Sebagai Titik Ekstensi
Cara table, field, dan index dibuat adalah strategi yang bisa diganti di dalam
backend ini — hybrid JSONB adalah default, bukan satu-satunya.

## 2. Hybrid JSONB (default)
Layout kolom, tipe, path payload.

## 3. Fully-Relational (potensial)
Tiap field jadi kolom nyata; kapan masuk akal; konsekuensi migrasi.

## 4. Extension Column
Mekanisme kolom per-extension untuk memenuhi kontrak uninstall bersih
(`ALTER TABLE DROP COLUMN`).

## 5. Index Generation
Penerjemahan `persist.indexes` ke DDL.
