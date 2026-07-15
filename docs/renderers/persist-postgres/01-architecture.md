# Arsitektur persist-postgres

**Updated:** 2026-07-15 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap dari kode `internal/db/`.

## 1. Hybrid JSONB
Kolom inti relasional + payload JSONB; alasan desain dan trade-off.

## 2. Pemenuhan Kontrak PersistBackend
Pemetaan tiap kemampuan wajib (structural diff, query resolution, next_key,
index, uninstall bersih) ke mekanisme konkretnya.

## 3. Transaksi, Outbox, Audit
Jaminan konsistensi dan event delivery.

## 4. Status Implementasi Hari Ini
Gap terhadap kontrak (termasuk bagian interface yang masih SQL-coupled).
