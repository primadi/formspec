# Plan — Fase 4 Kafe: Stok, HPP & Pembelian

**Sumber:** `examples/kafe/gaps_found/TODO.md` Fase 4 (`4.1`–`4.8`)
**Referensi spec:** `docs/spec/backend/01-core-basic.md` (§7 hooks, §10 Config),
`docs/spec/backend/02-core-extended.md` (§6.1 summary),
`docs/spec/backend/05-field-types.md` (§1.6 unit, §2.1 money),
`docs/reference/primitives.md` (dialek Starlark)
**Tanggal:** 2026-09-18

## Tujuan

Menutup 8 gap Fase 4 sehingga aplikasi kafe dapat menghitung HPP (moving
average), menjaga invarian `summary`, dan menulis guard tanpa raw SQL.

## Urutan & dependensi

```
4.2 (hooks/conditions pada summary)  ─┐
4.3 (find-by-field API)              ─┼─→ 4.4 (guard keunikan atomik) ─→ 4.1 (valuasi inventory)
4.5 (ctx.db di transaksi, SQLite)    ─┘
4.6 (konversi satuan) ─────────────────→ 4.1 (ledakan resep)
4.7 (HookDecl.uses)   — independen
4.8 (verifikasi money di laporan stok) — independen, verifikasi
```

**Prasyarat yang sudah selesai:** 1.3 (money), 1.6 (partial index), 1.8
(`maintained_by`/`invariants`/`unit`), 3.5 (row_scope), 3.6 (natural key scope).

## Rincian per item

### 4.2 — hooks/conditions benar-benar dipanggil pada `summary` (S14 + #33)

- **Masalah:** penulisan `summary` tidak lewat action pipeline → `hooks:` dan
  `conditions:` yang dipasang di entity `summary` **tidak pernah dipanggil**,
  sehingga manifest terlihat terlindungi padahal tidak.
- **File:** `internal/entity/registry.go` (jalur tulis summary),
  `internal/action/hooks.go` (`RunBeforePhase`), `internal/summary/`,
  `renderers/jsonb-persist/crud.go`.
- **Pendekatan:** tentukan jalur tulis summary yang sah (script `maintained_by`
  + internal), lalu panggil `before_*`/`after_*` hook di jalur itu — atau, bila
  summary memang tidak boleh punya hook, **tolak** deklarasinya di validator
  dengan pesan jelas (menghapus kondisi "terlihat terpasang tapi tidak jalan").
- **Effort:** medium. **Accept:** memasang `hooks:` di `summary` → entah
  benar-benar jalan, atau `formspec validate` menolaknya.

### 4.3 — API Starlark find-by-field (#31)

- **Masalah:** tidak ada API "find by field value" → guard keunikan terpaksa raw
  SQL, melanggar konvensi "never raw SQL".
- **File:** `internal/starlark/primitive.go` (builtin baru),
  `internal/starlark/context.go`, `docs/reference/primitives.md`.
- **Pendekatan:** tambah `ctx.db().find(entity, field, value)` (atau
  `ctx.entity(...).find(...)`) yang mengembalikan record/`None`, memakai
  resolver entity yang sudah ada. Harus menghormati `row_scope`/tenant.
- **Effort:** medium. **Accept:** guard keunikan kafe ditulis tanpa SQL mentah.

### 4.4 — guard keunikan atomik (#32)

- **Masalah:** guard keunikan di script tidak atomik → butuh `ctx.lock`, jadi
  reimplementasi UNIQUE yang lebih rapuh dari constraint-nya.
- **File:** `internal/starlark/primitive.go` (`ctx.lock`), atau arahkan ke
  `indexes[].where` (1.6) sebagai jawaban kanonik.
- **Pendekatan:** setelah 1.6, aturan keunikan **deklaratif** sudah jadi jawaban
  kanonik. Item ini menutup sisa: guard script yang masih perlu atomisitas
  (mis. cek-lalu-tulis lintas baris) harus punya primitif yang benar, atau
  didokumentasikan bahwa `indexes:` adalah satu-satunya cara yang didukung.
- **Effort:** small–medium. **Accept:** tidak ada guard keunikan kafe yang
  memakai pola cek-lalu-tulis tanpa penopang DB.

### 4.5 — `ctx.db()` di dalam transaksi aksi tidak deadlock di SQLite (#30)

- **Masalah:** `ctx.db()` query di dalam transaksi aksi deadlock di SQLite
  (koneksi tunggal) → guard tidak bisa diuji di dev.
- **File:** `renderers/jsonb-persist/txscope.go`, `internal/starlark/primitive.go`
  (`Querier`), `internal/action/`.
- **Pendekatan:** teruskan `TxScope` ke `ctx.db()` sehingga query memakai
  koneksi transaksi yang sama (bukan membuka koneksi kedua). Sudah ada
  `crud_txscope_test.go` untuk resolusi relasi — perluas ke `ctx.db()`.
- **Effort:** medium. **Accept:** guard yang memanggil `ctx.db()` di dalam
  transaksi aksi **selesai**, bukan hang, di SQLite.

### 4.6 — satuan & konversi (S12)

- **Masalah:** deklarasi `unit: {base, convertible}` ada (1.8), tetapi konversi
  belum dihitung engine → ledakan resep butuh konversi manual di script.
- **File:** `pkg/spec/entity.go` (`UnitDecl`), `internal/starlark/` (fungsi
  konversi), `renderers/jsonb-persist/` (bila perlu).
- **Pendekatan:** sediakan fungsi konversi (mis. `ctx.unit.convert(value, from,
  to)`) yang membaca deklarasi `unit` entity; satuan di luar grup → error.
- **Effort:** medium. **Accept:** resep gram + pembelian kg → ledakan resep
  benar tanpa konversi manual.

### 4.7 — `HookDecl.uses` (#34)

- **Masalah:** `HookDecl` tidak punya `uses` → akses script hook tidak terlihat
  di consent footprint.
- **File:** `pkg/spec/entity.go` (`HookDecl`), `internal/genjsonschema`,
  `cmd/formspec/` (honesty scan), `docs/`.
- **Pendekatan:** tambah `uses: {read: [...], write: [...]}` seperti
  `UsesConfigDecl`; validator memeriksa konsistensi dengan script.
- **Effort:** small. **Accept:** `formspec validate` melaporkan akses script
  hook; consent footprint memuatnya.

### 4.8 — verifikasi aritmetika/agregasi `money` di laporan stok (#28)

- **Masalah:** 1.3 menutup aritmetika/agregasi money, tetapi belum diverifikasi
  di **laporan stok** kafe.
- **File:** `examples/kafe/spec/modules/cafe-report/`, `cafe-stock/`.
- **Pendekatan:** jalankan laporan stok kafe, verifikasi `sum`/`avg` atas money
  benar (bukan 0, bukan teks).
- **Effort:** small (verifikasi). **Accept:** laporan stok menampilkan total
  money yang benar.

## Estimasi total

| Item | Effort |
| ---- | ------ |
| 4.2 | medium |
| 4.3 | medium |
| 4.4 | small–medium |
| 4.5 | medium |
| 4.6 | medium |
| 4.7 | small |
| 4.8 | small |
| 4.1 | large (bergantung 4.2–4.6) |

## Aturan

- Spec kafe **tidak di-degradasi** — gap ditutup di engine/bahasa spec.
- Setiap item: changelog + update TODO + bukti yang bisa gagal.
- `formspec validate --schema schemas` harus tetap **0 problem**.
