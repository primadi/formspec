# Menulis PersistBackend Baru

**Status:** Draft

> Panduan praktis — bukan kontrak. Definisi normatif ada di
> `docs/spec/backend/04-persist-backend.md`, dirujuk di tiap langkah.

## 1. Prasyarat
Baca [`../spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md)
penuh dulu — PersistBackend levelnya **setara Shell** di sisi visual (§1):
satu implementasi resmi (jsonb-persist) dipakai lama, tapi seluruh
framework wajib bicara ke interface ini, tidak boleh ada shortcut ke satu
backend tertentu di kode inti. Baca juga bagian storage-agnostic Core Basic
([`../spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md)
§1–§4) — itu kontrak yang backend-mu harus penuhi, terlepas mekanisme
internalnya.

## 2. Interface Wajib
Lima kemampuan minimal
([`04-persist-backend.md`](../spec/backend/04-persist-backend.md) §2):

1. **Structural diff apply** — terima diff skema dari framework
   (`renamed_from` untuk rename, dua tahap untuk field removal), terjemahkan
   ke storage-mu sendiri.
2. **Query resolution** — seluruh filter operator kontrak (`eq neq gt gte lt
   lte between in nin like ilike null notnull`,
   [`../spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md)
   §6) identik hasilnya dengan backend resmi.
3. **`ctx.next_key`** — sequence gap-free per `natural_key_rule`: atomik,
   gap-free (kecuali retry), duplicate-free, dilarang derive lewat scan
   `MAX()`.
4. **Index generation** — penuhi `persist.indexes`.
5. **Uninstall extension bersih** — tanpa sisa data/kolom yang tertinggal.

**Catatan dari implementasi resmi:** jsonb-persist sendiri belum 100%
memenuhi kelima ini hari ini (lihat
[`../renderers/jsonb-persist/01-architecture.md`](../renderers/jsonb-persist/01-architecture.md)
§4 dan [`04-query-and-keys.md`](../renderers/jsonb-persist/04-query-and-keys.md)
§1–§2 — cuma 9 dari 12 operator terimplementasi, mutasi+outbox tidak
atomik, uninstall extension belum ada implementasinya) — jangan jadikan
kode jsonb-persist sebagai acuan "sudah pasti benar", jadikan
`04-persist-backend.md` sebagai acuan, dan `renderers/jsonb-persist/*` §
Status Implementasi sebagai daftar apa yang **belum** boleh ditiru.

## 3. Menjaga Jaminan
Backend-mu wajib mempertahankan (§3): gap-free sequence, transaksionalitas,
idempotensi, **backup & restore** (format backup adalah bagian normatif
spec ini, bukan detail implementasi tersembunyi — full+incremental,
filterable, restore dengan mode `skip`/`overwrite`/`remap`, `--dry-run`
compatibility report), dan operasi baca/ekspor yang **tidak boleh**
license-gated di implementasi manapun.

## 4. Yang Tidak Perlu Dipenuhi
- Dialek `ctx.db` milik backend lain — `ctx.db` sengaja backend-coupled
  (§5); resource yang memakainya sudah menerima keterkuncian ke satu
  PersistBackend, backend-mu tidak perlu meniru dialek SQL backend lain.
- Detail strategi skema backend resmi (hybrid JSONB — kolom generated,
  extension column per-namespace,
  [`../renderers/jsonb-persist/02-schema-strategies.md`](../renderers/jsonb-persist/02-schema-strategies.md))
  — itu detail implementasi jsonb-persist, bukan kontrak. Backend-mu bebas
  strategi lain (mis. fully-relational — tiap field jadi kolom nyata) selama
  §2/§3 terpenuhi.
- Bentuk data yang diserahkan ke Shell (§6) — itu kontrak Spec Resolution
  API ([`../spec/frontend/04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md)
  §3), dipenuhi lapisan engine, bukan tanggung jawab langsung PersistBackend
  — tapi kamu tetap wajib **tidak membocorkan** detail fisik (nama kolom,
  path storage) ke lapisan itu.

## 5. Konformansi dan Registrasi
**Open** — mekanisme verifikasi konformansi PersistBackend baru (validasi
statis vs test-suite, [`04-persist-backend.md`](../spec/backend/04-persist-backend.md)
§7) belum dirumuskan, sama seperti gap konformansi Renderer
([`../spec/frontend/03-renderer-kind.md`](../spec/frontend/03-renderer-kind.md)
§5). Sampai itu final:
1. Implementasikan seluruh §2/§3 di atas.
2. Daftarkan sebagai `kind: PersistBackend` dengan `trust_tier` (`official
   | verified | community`, seragam dengan Renderer/Module).
3. Distribusikan lewat marketplace
   ([`../spec/platform/07-marketplace.md`](../spec/platform/07-marketplace.md)).
4. Dokumentasikan secara eksplisit di listing-mu bagian kontrak mana yang
   belum kamu penuhi — jangan diam-diam anggap selesai (pola yang sama
   dipakai dokumen ini sendiri untuk jsonb-persist, §2 di atas).
