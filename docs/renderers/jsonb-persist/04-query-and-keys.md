# Query & Keys

**Updated:** 2026-07-19 · Status: Outline

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
sendiri) — bukan lewat scan `MAX()`. Pada jalur auto-generate saat `create`
(`EntityStore.Insert`), UPSERT ini sekarang jalan **di transaksi yang sama**
dengan `INSERT` baris Document (`InTx`, `renderers/jsonbpersist/tx.go`) —
kalau insert gagal setelah counter terlanjur increment, rollback membatalkan
keduanya, menutup gap yang sebelumnya dicatat di sini (lihat
[`01-architecture.md`](01-architecture.md) §3 untuk cakupan penuh mutasi
atomik ini, termasuk apa yang masih belum tercakup). Mode gap-free penuh
("angka tidak pernah bolong sama sekali") tetap belum berarti *lock ditahan
lintas request bersamaan* — ini "counter dan insert commit/rollback
bersama", bukan serialisasi antar request konkuren, yang cukup untuk
kontrak ini (gap-free per unit transaksi, bukan zero-contention). `scope_field`
opsional memetakan nilai field lain (mis. `branch_id`) jadi komponen `scope`,
sehingga satu sequence independen per nilai itu alih-alih satu sequence
tenant-wide. Pemanggilan eksplisit `ctx.next_key(field)` dari script saat ini
selalu pakai scope tenant-wide (jalur itu tidak punya data resource untuk
resolve nilai `scope_field`) — menyamakan perilakunya dengan jalur
auto-generate saat `create` adalah follow-up yang belum dikerjakan.

`natural_key_rule.strategy: custom` (kontrak §2: `sequence | custom`) berarti
framework **tidak** auto-generate nilai ini — pemanggil (hook/script/import)
wajib mengisinya sendiri; `generateNaturalKeys` melewati field semacam itu
tanpa menyentuh counter, dan validasi required-field jadi pengaman kalau
field itu ternyata tidak diisi siapa pun.

## 3. Idempotency Store
`(tenant, action, key) → pending | completed | failed + response tersimpan`
([`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md)
§5) — entry tidak pernah dihapus saat commit, kedaluwarsa via `CleanupExpired`
lewat retention (`core.idempotency_retention`, default 24 jam,
`IdempotencyStore.WithTTL`). `resource.App` (`resource/forma.go`) sekarang
membuat satu `IdempotencyStore` per App dengan TTL dari `Config.IdempotencyTTL`
dan mengeksposnya lewat `App.Idempotency()` — TTL-nya nyata dipakai, bukan
field yang dihitung lalu dibuang. Resolusi TTL dari manifest `kind: Config`
(`core.idempotency_retention` sebagai key config, bukan field Go) menunggu
runtime Config-kind (belum ada registry-nya — lihat Fase 7.2 di
`docs/plan/todo.md`); sampai saat itu, `Config.IdempotencyTTL` adalah seam
konfigurasi yang setara, sama seperti `JWTSecret` dkk. Jalur HTTP
prepare-flow (`POST /{resource}/{action}/prepare`) yang benar-benar memakai
store ini saat request masuk belum ada (Fase 2.7) — bagian itu tetap gap.

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

## 6. Translasi Operator Tree

Kontrak operator hierarki untuk field `relation` ber-`tree: true`
([`../../spec/backend/05-field-types.md`](../../spec/backend/05-field-types.md)
§4) — `descendant_of`, `ancestor_of`, `child_of`, `root` — dipenuhi backend ini
**tanpa recursive CTE**, memakai prefix-match terhadap kolom materialized path
(`_tpath_<field>`, lihat [`02-schema-strategies.md`](02-schema-strategies.md)
§4).

### 6.1 `descendant_of`

Mencari semua turunan rekursif dari node X:

```sql
-- X bukan root
SELECT * FROM gl_accounts
WHERE tenant_id = $1
  AND _tpath_parent_id LIKE '<path(X)>.<X>.%'
  AND deleted_at IS NULL;

-- X adalah root (path kosong)
SELECT * FROM gl_accounts
WHERE tenant_id = $1
  AND _tpath_parent_id LIKE '<X>.%'
  AND deleted_at IS NULL;
```

Karena pola `LIKE` dimulai dengan prefix tetap (bukan wildcard di awal),
B-tree index pada `(tenant_id, _tpath_parent_id)` digunakan penuh — **tidak
ada sequential scan** dan **tidak ada recursive CTE**.

### 6.2 `ancestor_of`

Mencari semua leluhur rekursif dari node X:

1. Ambil path node X dari `data->>'__tpath.parent_id'` (atau dari kolom
   `_tpath_parent_id`)
2. Split path dengan separator `.` → dapatkan daftar ID ancestor
3. Query: `WHERE id IN (<daftar_id>) AND tenant_id = $1`

Karena lookup berdasarkan PK (`id`), ini adalah **O(depth)** query — depth
tree bisnis praktis jarang melebihi 10 level.

### 6.3 `child_of`

Anak langsung (satu tingkat) dari node X:

```sql
SELECT * FROM gl_accounts
WHERE tenant_id = $1
  AND data->>'parent_id' = '<X>'
  AND deleted_at IS NULL;
```

Ini adalah query field `relation` biasa — bukan operator path. Indeks pada
FK `parent_id` (jika ada generated column untuk field tersebut) mencukupi.

### 6.4 `root`

Node akar (tanpa parent):

```sql
SELECT * FROM gl_accounts
WHERE tenant_id = $1
  AND data->>'parent_id' IS NULL
  AND deleted_at IS NULL;
```

Atau ekuivalen: `_tpath_parent_id = ''`.

### 6.5 Siklus

Integritas tree (§4 kontrak) dicegah **sebelum** path ditulis: pada
`create`/`update`/reparent, framework memeriksa apakah kandidat parent
adalah turunan dari node yang sedang dimutasi — pemeriksaan pakai path
node kandidat parent (query `ancestor_of`). Siklus yang terdeteksi
menghasilkan `VALIDATION_ERROR` (422) sebelum transaksi commit. Backend ini
tidak mengandalkan recursive CTE untuk deteksi siklus — hanya prefix-match
string.
