# Query & Keys

**Updated:** 2026-07-15 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap.

## 1. Translasi Filter Operator
Pemetaan operator kontrak (`eq`, `gt`, `between`, …) ke SQL/JSONB path.

## 2. Natural Key Counter
Implementasi `ctx.next_key` gap-free (tabel counter, locking).

## 3. Idempotency Store
Mekanisme idempotensi action.

## 4. Summary Multi-Source
Bagaimana kontrak "gabungkan sources by join_key" dijawab dengan SQL join.

## 5. Dialek `ctx.db`
SQL mentah yang tersedia bagi resource yang memilih forfeit portability.
