# Script Runtime — API Penulisan Handler

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Katalog di sini normatif untuk semua
> handler `impl.script`/`impl.script_ref` (Starlark) dan menetapkan kontrak
> resolusi untuk `impl.type: native`. Batas eksekusi sandbox ada di
> [`02-core-extended.md`](02-core-extended.md) §7.1.

Dokumen ini mengkatalogkan **API yang tersedia di dalam script handler** — sesuatu
yang dipakai di hampir setiap contoh script tapi selama ini tersebar. Semua akses
di sini tunduk model permission `uses`/`required_permission`
([`01-core-basic.md`](01-core-basic.md) §5): apa yang disentuh handler wajib
dideklarasikan; akses undeclared diblokir saat resolusi.

## 1. Entrypoint

Setiap script handler mengekspos satu fungsi entrypoint:

```python
def execute(resource, params, ctx):
    # ... logika ...
    return ok({ "status": "done" })
```

| Argumen | Isi |
|---|---|
| `resource` | Record yang menjadi sasaran action — objek Document dengan API di §2. Untuk action `type: service` yang tidak terikat record, `resource` bisa `None`. |
| `params` | Payload masukan action (body request/argumen pemanggil), sudah lolos validasi field-level (L1–L3, [`05-field-types.md`](05-field-types.md) §3) sebelum entrypoint dipanggil. |
| `ctx` | Context runtime — primitive `ctx.*` (§5), identitas (`ctx.user`, `ctx.tenant`), utility (`ctx.now()`, `ctx.next_key()`), config & secret. |

## 2. Objek `resource`

`resource` adalah handle ke satu record Document, bukan dict mentah:

| API | Arti |
|---|---|
| `resource.id` | Primary key (UUID v7, [`01-core-basic.md`](01-core-basic.md) §2). |
| `resource.field(name)` | Membaca nilai satu field. |
| `resource.set(field, value)` | Menyetel nilai field di memori — belum dipersist sampai `save()`. |
| `resource.save()` | Mempersist perubahan **dalam transaksi action** ([`01-core-basic.md`](01-core-basic.md) §3), memicu guard lifecycle dan penulisan outbox event yang berlaku ([`01-core-basic.md`](01-core-basic.md) §7). |
| `resource.new()` | Membuat handle record **baru** (belum tersimpan) untuk entity yang sama; isi lewat `set(...)` lalu `save()`. |

`save()` bukan sekadar UPDATE mentah — ia melewati jalur yang sama dengan mutasi
lewat API (guard, denormalisasi, event), sehingga script tidak bisa menembus
kontrak lifecycle dengan menulis diam-diam.

## 3. Query dari Script

Query builder ([`02-core-extended.md`](02-core-extended.md) §16) diakses dari
script lewat referensi entity:

```python
# Ambil satu record
cust = Customer.query().where(id=params["customer_id"]).first()

# Ambil daftar
overdue = Invoice.query() \
    .where(status="submitted") \
    .where("due_date", "lt", ctx.now()) \
    .all()
```

`<EntityRef>.query()` mengembalikan builder dengan `.where(...)`, agregasi,
`group_by`/`having`, dan `include()` sesuai kontrak §16; terminatornya `.first()`
(satu record atau `None`) atau varian pengembali-daftar (`.all()`). Query lintas
`category` tetap dilarang ([`01-core-basic.md`](01-core-basic.md) §3,
`FORMA.PERSIST.CROSS_CATEGORY`), juga dari script. Referensi entity lintas-module
memakai notasi qualifier `{module}/{entity}` ([`02-core-extended.md`](02-core-extended.md)
§7) dan wajib dideklarasikan di `uses`.

## 4. Akses Lintas-Entity

Memanggil atau memuat record entity **lain** dari dalam script:

| API | Arti |
|---|---|
| `<resource>.load(id)` | Memuat satu record entity lain berdasarkan id, mengembalikan handle §2. |
| `<resource>.call(action, params)` | Memanggil action bernama pada entity lain (mis. membuat journal entry dari handler invoice). |

Keduanya adalah akses lintas-resource dan **wajib** dideklarasikan di `uses`
([`01-core-basic.md`](01-core-basic.md) §5); pemanggilan cross-boundary tunduk
aturan idempotensi/kompensasi yang sama seperti Integrator
([`02-core-extended.md`](02-core-extended.md) §5). Panggilan same-process
di-dispatch langsung tanpa melewati jaringan ([`01-core-basic.md`](01-core-basic.md)
§8).

## 5. Logging & Primitive `ctx.*`

Katalog lengkap `ctx.*` (db/cache/lock/queue/pubsub/storage/kvstore, identitas,
utility) hidup di [`../../runtimes/02-forma-resource.md`](../../runtimes/02-forma-resource.md)
§4 dan [`04-persist-backend.md`](04-persist-backend.md) §5. Yang relevan untuk
penulisan handler:

- **Logging** — `ctx.log.info(...)`, `ctx.log.warn(...)`, `ctx.log.error(...)`
  meng-emit log terstruktur ([`../platform/09-observability.md`](../platform/09-observability.md)
  §2). Log **tidak pernah** boleh memuat nilai secret ([`02-core-extended.md`](02-core-extended.md)
  §18) atau PII mentah ([`../platform/09-observability.md`](../platform/09-observability.md)
  §2.2).
- **Config & secret** — `ctx.config.get("key")` untuk key non-secret
  ([`01-core-basic.md`](01-core-basic.md) §10); `ctx.secrets` untuk key
  `secret: true`, tunduk `uses: {secrets: [...]}` dan selalu di-audit
  ([`02-core-extended.md`](02-core-extended.md) §18).
- **Async job** — `ctx.job.progress(pct, message)` melaporkan progres dari
  handler async yang di-track ([`02-core-extended.md`](02-core-extended.md) §13).

## 6. Kontrak Return

Handler mengembalikan salah satu dari dua konstruktor hasil, yang memetakan
langsung ke envelope respons HTTP ([`01-core-basic.md`](01-core-basic.md) §8):

| Return | Hasil |
|---|---|
| `ok(data)` | Sukses — `data` menjadi `data` di envelope respons (`2xx`). |
| `fail(message, code?)` | Gagal — membatalkan transaksi action dan mengembalikan envelope error `{error: {code, message, details}, meta}`. Tanpa `code`, `fail` memakai kode error generik; `conditions`/error bisnis SEBAIKNYA membawa `code` bernamespace App (bukan `FORMA.*`, [`01-core-basic.md`](01-core-basic.md) §9). |

`fail()` di dalam handler membatalkan seluruh transaksi (§2 `save()` yang sudah
terjadi ikut rollback) — tidak ada hasil parsial, konsisten dengan jaminan
atomisitas ([`01-core-basic.md`](01-core-basic.md) §3).

## 7. Resolusi `ref` Handler Native

Untuk `impl.type: native`, handler ditulis sebagai method Go di `impl/`
([`../platform/08-project-layout.md`](../platform/08-project-layout.md) §2) dan
dirujuk lewat string `ref: "{Type}.{Method}"`:

```yaml
impl:
  type: native
  ref: "OrderResource.UpdateDiscountRule"
```

Resolusi terjadi dengan **memindai `impl/**/*.go` module** untuk tipe exported
plus method exported yang cocok dengan nama itu (`OrderResource` +
`UpdateDiscountRule`). Aturan normatif:

- **Nama harus unik dalam module.** Bila lebih dari satu `{Type}.{Method}` cocok
  di seluruh `impl/` module, itu **error saat `forma apply`/build** — bukan
  ambiguitas yang dibiarkan sampai runtime.
- Tidak ada match sama sekali juga error build-time — `ref` menggantung ditolak
  sebelum deployment.

Resolusi build-time ini menjaga `ref` selalu menunjuk tepat satu handler yang
ada, sehingga permukaan action yang di-compile tidak pernah menyembunyikan
handler hilang atau dobel.
