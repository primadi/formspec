# PersistBackend

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. §8 mencatat status implementasi
> hari ini terhadap kontrak ini — bagian itu boleh berubah tanpa mengubah
> kontrak.

## 1. Kedudukan
PersistBackend adalah seam penyimpanan **setara Shell di sisi visual**: satu
implementasi resmi (jsonb-persist, hybrid JSONB, Postgres/SQLite) dipakai
lama, tapi seluruh framework wajib bicara ke interface ini — tidak ada
shortcut ke Postgres di kode inti. Kind `PersistBackend` dideklarasikan
formal, satu per deployment scope.

Prinsip yang mengikat: kalau PersistBackend kedua suatu saat ingin
dimungkinkan (mis. strategi fully-relational — tiap field jadi kolom nyata,
lihat [`../../renderers/jsonb-persist/02-schema-strategies.md`](../../renderers/jsonb-persist/02-schema-strategies.md)),
*seam*-nya harus sudah ada sejak PersistBackend pertama dibangun — bukan
ditambal belakangan. Retrofit setelah kode inti terlanjur mengasumsikan satu
implementasi berarti membongkar migration engine, query resolution, dan
`ctx.next_key` sekaligus, bukan menambah modul baru.

## 2. Interface Wajib
Kemampuan minimal setiap backend, dirumuskan sebagai kontrak (bukan signature
Go tertentu — tiap PersistBackend bebas menerjemahkannya ke mekanisme
internalnya):

- **Structural diff apply.** Framework menghasilkan diff skema dari
  perbandingan Entity manifest versi lama vs baru (field ditambah/dihapus/
  di-`renamed_from`, index berubah); PersistBackend menerima diff itu dan
  menerjemahkannya ke storage-nya sendiri. Field rename **wajib** dideklarasi
  lewat `renamed_from` — tanpa itu, diff membacanya sebagai drop+add. Field
  removal butuh dua tahap (deprecate lalu remove) lintas dua versi ter-apply.
  Migrasi data (backfill) adalah tipe terpisah — script data-migration
  ber-versi, run/rollback manual — bukan bagian structural diff.
- **Query resolution.** Memenuhi seluruh filter operator kontrak (`eq`, `gt`,
  `between`, dst. — [`01-core-basic.md`](01-core-basic.md) §6) identik antar
  backend; hasilnya tidak boleh berbeda perilaku tergantung backend yang
  dipakai.
- **`ctx.next_key`.** Sequence gap-free per `natural_key_rule`: increment wajib
  atomik, gap-free, duplicate-free — dilarang derive lewat `MAX()` scan.
  Alokasi terjadi di bawah lock yang sama dengan transaksi insert/update; kalau
  transaksi itu gagal optimistic-concurrency check dan di-retry, gap **boleh**
  terjadi kecuali document mendeklarasikan mode gap-free (lock ditahan sampai
  commit). `scope_field` (opsional) membuat sequence terpisah per nilai field
  itu (mis. satu sequence per `branch_id`) alih-alih satu sequence per
  tenant/resource/field/period.
- **Index generation** — memenuhi `persist.indexes`.
- **Uninstall extension bersih** — tanpa sisa (lihat §6 soal mekanisme
  konkretnya sebagai detail implementasi, bukan kontrak).

## 3. Jaminan yang Dipertahankan

Gap-free sequence, transaksionalitas, idempotensi — dirumuskan generik tanpa
kehilangan garansi yang ada.

**Backup & restore (credible exit guarantee).** Format backup adalah bagian
normatif dari spesifikasi terbuka ini — bukan detail implementasi yang boleh
disembunyikan operator atau vendor PersistBackend tertentu. Setiap
PersistBackend WAJIB mendukung: backup penuh maupun incremental, filterable;
file storage ikut ter-backup (summary/agregat tidak, karena bisa dihitung
ulang); restore dengan mode konflik `skip`/`overwrite`/`remap` (UUID dan FK
di-remap konsisten) serta laporan kompatibilitas `--dry-run` sebelum eksekusi.

Operasi baca/ekspor (`list`, `find`, `export`, `backup`) **tidak boleh**
license-gated, tanpa kedaluwarsa, di PersistBackend manapun — ini yang membuat
implementasi Forma manapun bisa direstore oleh implementasi lain yang konform,
memberi pemilik workspace jalan keluar yang kredibel dari satu operator/vendor.

## 4. Konvensi Query & Format API

Lihat kontrak lengkap di [`01-core-basic.md`](01-core-basic.md) §6 (filter
operator) dan §8 (response envelope, kode error) — PersistBackend manapun
wajib menjawab query resolution dengan hasil yang identik terhadap kontrak
tersebut, terlepas mekanisme internalnya (SQL, dokumen, dll).

## 5. `ctx.db` — Escape Hatch yang Mengorbankan Portabilitas
Akses SQL mentah sengaja backend-coupled: resource yang memakainya terkunci ke
PersistBackend berdialek itu. Bukan bug — konsekuensi yang harus disadari saat
memilihnya. Ini **satu-satunya** primitive `ctx.*` yang boleh backend-coupled;
`ctx.cache`, `ctx.lock`, `ctx.queue`, `ctx.pubsub`, `ctx.storage`,
`ctx.kvstore`, `ctx.config` tetap wajib storage-agnostic di seluruh
PersistBackend.

**Tidak ada penyimpanan tak terkelola di satu workspace** — data di luar
primitive `ctx.*` keluar diam-diam dari seluruh jaminan framework (backup §3,
credible exit, isolasi tenant). Tangga resmi untuk kebutuhan lanjutan di luar
Document/Entity biasa: (1) `ctx.db` mentah (bagian ini); (2) tabel milik
module lewat `kind: Migration` — struktur bebas, tapi **wajib** kolom
`tenant_id` dan tetap tunduk backup/isolasi/audit; (3) engine eksotik
(search/vector/graph) lewat provider app yang dimiliki vendor, atau dibungkus
`kind: Service`. Workspace Owner tidak pernah menyediakan storage mentah
langsung ke module.

## 6. Batas dengan Spec Resolution API
Bentuk data yang diserahkan ke Shell tidak boleh membocorkan detail backend
(nama kolom fisik, path JSONB) — lihat
[`../frontend/04-spec-resolution-api.md`](../frontend/04-spec-resolution-api.md)
§3. Mekanisme kolom per-extension (mis. `ALTER TABLE DROP COLUMN` di backend
JSONB) adalah detail implementasi backend tertentu — kontraknya cuma
"extension harus bisa di-uninstall bersih tanpa sisa" (§2); *cara* mencapainya
urusan masing-masing PersistBackend.

## 7. Menambah PersistBackend Baru
Alur: (1) implementasikan seluruh kemampuan wajib §2 dan jaminan §3; (2)
daftarkan sebagai kind `PersistBackend` dengan `trust_tier` yang sama
(`official | verified | community`) dengan Renderer visual
([`../frontend/03-renderer-kind.md`](../frontend/03-renderer-kind.md)); (3)
distribusi lewat marketplace ([`../platform/07-marketplace.md`](../platform/07-marketplace.md)).

**Konformansi (normatif).** Mengikuti pola berjenjang yang sama dengan
Renderer ([`../frontend/03-renderer-kind.md`](../frontend/03-renderer-kind.md)
§5): validasi statis deklarasi adalah syarat minimum semua tier;
**test-suite konformansi** yang mengeksekusi seluruh kemampuan wajib §2 dan
jaminan §3 (structural diff, query semantics identik, `ctx.next_key`
atomik/gap-free, backup/restore format normatif, uninstall extension bersih)
**wajib lulus untuk tier `verified` dan `official`**. Hanya PersistBackend
`official` yang terpilih otomatis; tier lain wajib dipilih eksplisit dan
muncul di consent footprint.

## 8. Status Implementasi Hari Ini (Gap)
`internal/db.DB`/`Tx` (implementasi resmi jsonb-persist) **belum** jadi
interface PersistBackend yang bersih terhadap kontrak §2 — ia bocor semantik
SQL langsung ke pemanggil (`ExecContext`, `QueryContext`, `Driver() *sql.DB`),
dan migration engine (`internal/db/migrate.go`, `PlanMigrations`/
`ApplyMigrations`) menghasilkan `DDLResult` (teks SQL) sebagai representasi
diff-nya, bukan diff storage-agnostic yang lantas diterjemahkan tiap backend.
Ini bukan kesalahan implementasi — jsonb-persist memang satu-satunya backend
hari ini — tapi berarti kode inti belum benar-benar berbicara ke seam
PersistBackend yang storage-agnostic; ia masih memanggil `internal/db`
langsung. Dicatat sebagai gap arsitektural untuk fase restrukturisasi kode,
bukan diam-diam dianggap sudah selesai — lihat
[`../../architecture/08-repo-structure.md`](../../architecture/08-repo-structure.md)
§4.
