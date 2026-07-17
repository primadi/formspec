# Query & Keys

**Updated:** 2026-07-16 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap.

## 1. Translasi Filter Operator
**Cakupan hari ini lebih sempit dari kontrak.** Operator kontrak
([`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md)
§6 mendaftar `eq neq gt gte lt lte between in nin like ilike null notnull`)
— yang benar-benar diterjemahkan ke SQL hari ini cuma **sembilan**: `eq neq
gt gte lt lte like in nin`. `between`, `ilike`, `null`, `notnull` **belum
diimplementasikan sama sekali**.

Filter/sort juga **cuma bisa menyasar field yang punya `index`/`unique`/
`natural_key`** — tidak ada fallback query ke path JSONB mentah
(`data->>'field'`) untuk field yang tidak ter-index seperti sempat
didesain; field non-indexed sederhananya tidak bisa difilter/disortir sama
sekali hari ini, bukan "bisa tapi lebih lambat". Klaim "hasil query identik
terlepas jalur mana yang dipakai" di kontrak karena itu belum bisa diuji
sepenuhnya — cuma ada satu jalur (generated column), bukan dua jalur yang
perilakunya perlu disamakan.

## 2. Natural Key Counter
`ctx.next_key` ([`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md)
§2, [`../../spec/backend/04-persist-backend.md`](../../spec/backend/04-persist-backend.md)
§2) diimplementasikan lewat tabel counter:

```sql
-- PK = tenant/resource/field/scope/period
forma_natural_key_counters (tenant_id, resource, field, scope, period, seq)
```

`NaturalKeyCounter.NextSequence` meng-UPSERT baris counter secara atomik
(satu statement, gap-free dan duplicate-free untuk nilai counter itu
sendiri) — bukan lewat scan `MAX()`. **Tapi** UPSERT counter ini **tidak**
dibungkus transaksi bersama insert/update Document-nya (`BeginTx` tidak
pernah dipanggil di jalur ini, sama seperti gap outbox di
[`01-architecture.md`](01-architecture.md) §3) — kalau insert gagal setelah
counter terlanjur increment, angkanya tetap "terpakai": gap selalu mungkin
terjadi hari ini, bukan cuma "kecuali mode gap-free dideklarasikan" seperti
niat kontrak (mode gap-free itu sendiri butuh lock ditahan sampai commit
transaksi bersama — belum ada mekanismenya). `scope_field` opsional
memetakan nilai field lain (mis. `branch_id`) jadi komponen `scope`, sehingga
satu sequence independen per nilai itu alih-alih satu sequence tenant-wide.
Pemanggilan eksplisit `ctx.next_key(field)` dari script saat ini selalu pakai
scope tenant-wide (jalur itu tidak punya data resource untuk resolve nilai
`scope_field`) — menyamakan perilakunya dengan jalur auto-generate saat
`create` adalah follow-up yang belum dikerjakan.

## 3. Idempotency Store
`(tenant, action, key) → pending | completed | failed + response tersimpan`
([`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md)
§5) — entry tidak pernah dihapus saat commit, kedaluwarsa via `CleanupExpired`
lewat retention (`core.idempotency_retention`, default 24 jam). Ini bagian
yang implementasinya sudah sesuai kontrak, tidak seperti §1/§2 di atas.

## 4. Summary Multi-Source
Kontrak "gabungkan sources by join_key"
([`../../spec/backend/02-core-extended.md`](../../spec/backend/02-core-extended.md)
§6) dijawab lewat SQL join biasa antar tabel yang di-generate framework ini —
detail konkretnya (bentuk join, strategi refresh) mengikuti bagaimana Summary
itu dipopulasikan dari event durable (rebuild via replay event stream, bukan
query on-demand terhadap sources-nya).

## 5. Dialek `ctx.db`
Resource yang memilih `ctx.db` ([`../../spec/backend/04-persist-backend.md`](../../spec/backend/04-persist-backend.md)
§5) mendapat SQL mentah sesuai driver aktif (`DriverName()` — `sqlite` untuk
dev, `postgres` untuk produksi) lewat `internal/db.DB`. Dialek SQL antar
kedua driver **tidak** dijamin identik (mis. sintaks upsert, fungsi tanggal)
— resource yang memakai `ctx.db` bertanggung jawab sendiri menulis SQL yang
kompatibel driver yang ia targetkan, atau menerima keterkuncian ke satu
driver tertentu.
