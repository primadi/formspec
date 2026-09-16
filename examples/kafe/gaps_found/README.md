# Gap Found — FormSpec × Aplikasi Kafe

Catatan gap yang ditemukan saat membangun **aplikasi kafe** (multi-outlet, POS +
QR order pelanggan) sebagai **test case FormSpec untuk masalah bisnis nyata**.

Tujuan folder ini: memberi umpan balik konkret ke FormSpec — apa yang sudah
_benar-benar_ tersedia out-of-the-box, apa yang belum, dan di mana
dokumen/kode tidak sinkron.

> **Status:** catatan riset (read-only terhadap FormSpec). Setiap item
> mencantumkan bukti (file/kode/dokumen) + tingkat keyakinan.

---

## 🎯 Mulai dari sini: `13-kelengkapan-spec-untuk-kafe.md`

**Kalau tujuannya mencari apa yang kurang dari BAHASA SPEC FormSpec** — bukan
bug engine — buka **[`13-kelengkapan-spec-untuk-kafe.md`](13-kelengkapan-spec-untuk-kafe.md)**.

Dokumen itu menjawab satu pertanyaan: _apa yang tidak bisa dinyatakan di YAML
untuk membangun aplikasi kafe?_ Isinya dikelompokkan sebagai:

| Bagian | Isi                                                                               |
| ------ | --------------------------------------------------------------------------------- |
| §A     | **Tidak bisa dinyatakan sama sekali** (S1–S10) + sketsa konstruk yang dibutuhkan  |
| §B     | **Bisa dinyatakan tapi tidak lengkap** (S11–S16)                                  |
| §C     | **Sudah cukup — jangan diubah**                                                   |
| §D     | **Semantik yang belum ditetapkan spec** (D1–D7) — penulis spec tidak bisa menebak |
| §E     | Bukan gap spec (perbaikan di project FormSpec)                                    |
| §F     | Urutan perbaikan berdasarkan "berapa banyak aplikasi yang terbuka"                |

### Klasifikasi seluruh ledger

File #1–#12 mencampur tiga **jenis** gap yang berbeda. Pemisahannya:

| Jenis                                           | Perbaikan di mana       | Cara menemukan                        |
| ----------------------------------------------- | ----------------------- | ------------------------------------- |
| **SPEC** — tidak bisa dinyatakan di YAML        | Perluasan bahasa spec   | lihat §A–§B di dokumen 13             |
| **ENGINE** — bisa dinyatakan, belum dikerjakan  | Source FormSpec         | #10, #15, #22, #27, #30–#33, #40–#42  |
| **DOC** — dokumen/skill/validator tidak sinkron | Sumber dokumen & schema | #18, #19, #20, #21, #25, #37, #43     |
| **PERILAKU** — semantik belum ditetapkan        | Keputusan desain spec   | §D dokumen 13 (+ #26, #46)            |
| **DIKOREKSI/DIBATALKAN**                        | —                       | #2 (separuh), #18, #24, #26 (separuh) |

> Empat klaim saya gugur setelah diuji: #2 (storage money), #18 (`target:`
> ditolak validator), #24 (`formspec dev` "mati" — server jalan normal),
> #26 (tanpa `settings.currency` tidak selalu gagal). Pola penyebabnya sama:
> menyimpulkan dari pembacaan kode/dokumen atau **satu** perintah, bukan dari
> percobaan yang bisa gagal.

## Cara Kerja Catatan Ini

Karena repo FormSpec tidak dibuka di workspace ini, verifikasi dilakukan atas
repo <https://github.com/primadi/formspec> (branch `main`) + site
<https://docs.formspec.dev>. Setiap klaim menyertakan path file sebagai bukti.

Label keyakinan:

| Label                           | Arti                                                                         |
| ------------------------------- | ---------------------------------------------------------------------------- |
| **✅ Pasti**                    | Dilihat langsung di kode/dokumen; tidak ada cabang kode lain yang menutupnya |
| **⚠️ Perlu verifikasi runtime** | Simpul kode jelas mengarah ke kesimpulan, tapi belum diuji di browser/DB     |
| **📄 Drift dokumen**            | Dua dokumen/kode saling bertentangan — salah satu kedaluwarsa                |

Label dampak ke aplikasi kafe:

| Label       | Arti                                                                     |
| ----------- | ------------------------------------------------------------------------ |
| **BLOCKER** | Tidak bisa diakali dari YAML; butuh perubahan engine                     |
| **HIGH**    | Bisa diakali dengan kerja manual/script, tapi jauh dari "out of the box" |
| **MEDIUM**  | Bisa diakali dengan deklarasi YAML atau composition                      |

> **Catatan verifikasi:** gap #1–#21 ditemukan lewat pembacaan kode & dokumen repo
> FormSpec. Gap #4b, #21, #22, #23, #24 ditemukan/diperkuat saat menulis spec dan
> menjalankan CLI sungguhan (`formspec v0.0.8`). Label **✅ Terverifikasi** berarti
> klaimnya didukung keluaran CLI pada spec ini — bukan lagi inferensi kode.
> Lihat `08-ddl-index-dan-devtools.md` untuk bukti mentahnya.

## Daftar Gap

| #   | Gap                                                                                                                                                                                                                                                                                                                                                                       | Dampak                            | Keyakinan                                                                        |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------- | -------------------------------------------------------------------------------- |
| 1   | Widget `money` & `time` tidak ada — jatuh senyap ke input teks                                                                                                                                                                                                                                                                                                            | **BLOCKER** (POS)                 | ✅ Pasti                                                                         |
| 2   | ✅ **Diperbaiki 2026-09-14** (`moneyAmount()` di renderer) — Nilai `money` berbentuk objek & renderer hanya format angka — **storage terbukti aman**, sisi renderer belum diuji                                                                                                                                                                                           | **HIGH**                          | ⚠️ Renderer belum diuji                                                          |
| 3   | Tidak ada widget/kind **QR code** (generate maupun scan)                                                                                                                                                                                                                                                                                                                  | **BLOCKER** (QR order)            | ✅ Pasti                                                                         |
| 4   | **Gambar produk tidak tampil** di Table/Listing/Detail (hanya upload + thumbnail form)                                                                                                                                                                                                                                                                                    | **BLOCKER** (menu)                | ✅ Pasti                                                                         |
| 4b  | Format `storage.allowed_types` ambigu (`jpg` vs `.jpg` vs `image/jpeg`)                                                                                                                                                                                                                                                                                                   | **MEDIUM**                        | ✅ Pasti                                                                         |
| 5   | Tidak ada blok **cart/keranjang** di Page; `Listing` read-only tanpa aksi baris                                                                                                                                                                                                                                                                                           | **BLOCKER** (QR order)            | ✅ Pasti                                                                         |
| 6   | ✅ **Diperbaiki 2026-09-14** (`App.spec.public_entities`: allowlist per entity+aksi; `member`/`employee`/`shift`/`cash-movement` → 401 untuk anonim) — Akses anonim bersifat **per-module, bukan per-entitas** — anonim bisa `list` semua pesanan                                                                                                                         | **HIGH** (privasi)                | ✅ Pasti                                                                         |
| 7   | `exclude: [public_api]` belum ditegakkan (masih "concern Fase 6")                                                                                                                                                                                                                                                                                                         | **HIGH** (privasi)                | ✅ Pasti                                                                         |
| 8   | 🟡 **Mekanisme S2 siap 2026-09-14** (`EntitySpec.Scope`, server-enforced, fail-closed) — **adopsi di kafe menunggu 1.8/1.2** — Multi-outlet: hanya `tenant_id` yang di-inject framework; `TenantDecl` **tidak terpakai**                                                                                                                                                  | **BLOCKER** (multi-cabang)        | ✅ Pasti                                                                         |
| 9   | `scope_field` natural key tidak sampai ke `ctx.next_key()`                                                                                                                                                                                                                                                                                                                | **HIGH** (nomor per outlet)       | ✅ Pasti                                                                         |
| 10  | Print `thermal`/`dotmatrix` belum diimplementasi (selalu keluar PDF)                                                                                                                                                                                                                                                                                                      | **BLOCKER** (struk)               | ✅ Pasti                                                                         |
| 11  | Relasi lintas `persist.category` **diblokir senyap** (log warning saja)                                                                                                                                                                                                                                                                                                   | **HIGH**                          | ✅ Pasti                                                                         |
| 12  | Resolusi tabel target relasi naif (`+s`) — guard referenceability bisa lolos                                                                                                                                                                                                                                                                                              | **HIGH**                          | ✅ Pasti                                                                         |
| 13  | Inventory vertical: tanpa valuasi (FIFO/HPP), tanpa stock opname, `transfer` rusak                                                                                                                                                                                                                                                                                        | **BLOCKER** (HPP/margin)          | ✅ Pasti                                                                         |
| 14  | Vertical `purchase` belum ada (supplier & pembelian bahan)                                                                                                                                                                                                                                                                                                                | **HIGH**                          | ✅ Pasti                                                                         |
| 15  | Cross-app grant tidak ditegakkan; SyncAgent belum tersambung ke router                                                                                                                                                                                                                                                                                                    | **BLOCKER** (integrasi akuntansi) | ✅ Pasti                                                                         |
| 16  | `Dashboard widget.ref` dicocokkan dengan nama polos, bukan module-qualified                                                                                                                                                                                                                                                                                               | **MEDIUM**                        | ✅ Pasti                                                                         |
| 17  | Realtime hanya di Table/Kanban/Dashboard — Timeline belum di-wire                                                                                                                                                                                                                                                                                                         | **MEDIUM** (KDS)                  | ✅ Pasti                                                                         |
| 18  | Skill `entity-authoring` mengajarkan `relation` wajib `target` — **dikoreksi:** validator menangkapnya, jadi bug dokumen bukan gagal senyap                                                                                                                                                                                                                               | **MEDIUM** (friction authoring)   | ⚠️ Dikoreksi                                                                     |
| 19  | Drift dokumen: `03-kind-renderers.md` & `realtime.md` sudah kedaluwarsa                                                                                                                                                                                                                                                                                                   | **MEDIUM**                        | 📄 Drift                                                                         |
| 20  | `spec.version: v1` vs `formspec-app.yaml` vs nama field — beberapa dokumen belum selaras                                                                                                                                                                                                                                                                                  | **MEDIUM**                        | 📄 Drift                                                                         |
| 21  | Validator tidak menangkap referensi menggantung (`App.spec.modules`, menu `view:`, `impl.ref` ke file `.star`)                                                                                                                                                                                                                                                            | **HIGH**                          | ✅ Pasti                                                                         |
| 22  | ✅ **Diperbaiki 2026-09-14 (indexes dihormati) + dituntaskan 2026-09-15 (S8)** — `indexes:` (root **maupun** `persist`) dihormati, field `relation` dapat kolom turunan, **dan** index kini bisa **parsial** (`where:`); tiga aturan keunikan kafe dinyatakan di manifest dan `kind: Migration` DDL mentahnya dihapus                                                     | **HIGH**                          | ✅ Terverifikasi (DDL + constraint ditegakkan)                                   |
| 23  | Kolom turunan `money` bertipe `text` → sortir/rentang atas uang leksikografis, bukan numerik                                                                                                                                                                                                                                                                              | **HIGH**                          | ✅ Terverifikasi DDL                                                             |
| 24  | ~~`formspec dev` menolak start di Windows~~ → **DIBATALKAN**: server jalan normal; saya salah diagnosis (port sedang terpakai). Sisa: pesan error tidak menuntun                                                                                                                                                                                                          | **LOW**                           | ⛔ Dibatalkan                                                                    |
| 25  | Skill mendokumentasikan `Config.spec.data` — schema menolaknya; bentuk benar `spec.keys`                                                                                                                                                                                                                                                                                  | **MEDIUM**                        | ✅ Terverifikasi                                                                 |
| 26  | ~~`settings.currency` wajib untuk `money`~~ → **DIKOREKSI**: tidak diterapkan; money tanpa currency **masuk tanpa keluhan**                                                                                                                                                                                                                                               | **HIGH**                          | ⚠️ Dikoreksi                                                                     |
| 27  | ✅ **Diperbaiki 2026-09-14** (`fieldTypeToSQLFor`) — pemetaan tipe SQL tidak driver-aware — tipe PostgreSQL (`timestamptz`) bocor ke DDL SQLite                                                                                                                                                                                                                           | **LOW–MEDIUM**                    | ✅ Terverifikasi DDL                                                             |
| 28  | Aritmetika (`computed`) & agregasi (`SUM`) atas `money` belum teruji — seluruh laporan finansial bergantung padanya                                                                                                                                                                                                                                                       | **HIGH (berpotensi)**             | ⚠️ Terbuka                                                                       |
| 29  | Kontrak `Report` beda dari `Table` (`widget` ditolak, `totals` butuh `{label,field,fn}`); `ReportParam.type`/`aggregate`/`format` string bebas                                                                                                                                                                                                                            | **MEDIUM**                        | ✅ Terverifikasi                                                                 |
| 30  | `ctx.db()` query di dalam transaksi aksi **deadlock di SQLite** (koneksi tunggal) — guard tidak bisa diuji di dev                                                                                                                                                                                                                                                         | **HIGH** (dev)                    | ⚠️ Kuat                                                                          |
| 31  | Tidak ada API Starlark “find by field value” → guard keunikan terpaksa raw SQL, melanggar konvensi “never raw SQL”                                                                                                                                                                                                                                                        | **HIGH**                          | ✅ Terverifikasi                                                                 |
| 32  | Guard keunikan di script tidak atomik — butuh `ctx.lock`, jadi reimplementasi UNIQUE yang lebih rapuh dari constraint-nya                                                                                                                                                                                                                                                 | **HIGH**                          | ✅ Terverifikasi                                                                 |
| 33  | `hooks:`/`conditions:` tidak pernah dipanggil pada entity `summary` → guard terlihat terpasang tapi tidak jalan                                                                                                                                                                                                                                                           | **MEDIUM–HIGH**                   | ⚠️ Kesimpulan desain                                                             |
| 34  | `HookDecl` tidak punya `uses` → akses script hook tidak terlihat di consent footprint                                                                                                                                                                                                                                                                                     | **MEDIUM**                        | ✅ Terverifikasi                                                                 |
| 35  | `MigrationSpec.ddl` hanya satu string → DDL **tidak bisa portabel** (`json_extract` SQLite vs `->>` PostgreSQL)                                                                                                                                                                                                                                                           | **HIGH**                          | ✅ Terverifikasi                                                                 |
| 36  | Migration hanya boleh DDL → `CREATE UNIQUE INDEX` gagal bila sudah ada duplikat, dan datanya tidak bisa dirapikan                                                                                                                                                                                                                                                         | **HIGH**                          | ✅ Terverifikasi                                                                 |
| 37  | Shorthand `render: drawer` disebut diterima di deskripsi schema, tapi **ditolak schema** — loader dan validator beda aturan                                                                                                                                                                                                                                               | **MEDIUM**                        | ✅ Terverifikasi                                                                 |
| 38  | ✅ **Ditutup 2026-09-15 (S9, TODO 1.7)** — pemicu Workflow merujuk transisi lewat `name:` (`via`), sehingga satu workflow mengawal **seluruh** state asal; pasangan `from`/`to` yang hanya mencakup sebagian transisi kini **ditolak** `formspec validate`; sebelumnya: workflow tidak bisa mengawal transisi dengan banyak state asal → void lolos approval tanpa gejala | **HIGH**                          | ✅ Terverifikasi (unit + validator)                                              |
| 39  | `WorkflowStep` tanpa `title`/`description` → tugas approval tanpa label; approver tidak tahu apa yang disetujui                                                                                                                                                                                                                                                           | **MEDIUM**                        | ✅ Terverifikasi                                                                 |
| 40  | Tidak jelas apakah transisi state machine memancarkan event otomatis — nama event = state (dokumen vertical) vs prefix `on_*` (validator)                                                                                                                                                                                                                                 | **HIGH**                          | ✅ Terverifikasi                                                                 |
| 41  | `Integrator` tidak punya pemetaan payload (`call` hanya resource+action) → integrasi akuntansi tak bisa dinyatakan deklaratif                                                                                                                                                                                                                                             | **HIGH**                          | ⚠️ Verifikasi                                                                    |
| 42  | Dua App meng-mount module yang sama (`kafe-qr` + `kafe-pos`) → tidak jelas App mana pemilik antarmuka `publishes`                                                                                                                                                                                                                                                         | **MEDIUM–HIGH**                   | ✅ Terverifikasi                                                                 |
| 43  | Aturan simetri cancel (7.7.2) **ditegakkan validator tapi tidak terdokumentasi** — aturannya bagus, penemuannya sulit                                                                                                                                                                                                                                                     | **MEDIUM** (dokumentasi)          | ✅ Terverifikasi                                                                 |
| 44  | ✅ **Diperbaiki 2026-09-14** (aturan `LifecycleFree()`: `master`/`reference`/`plain_crud` ⇒ langsung referenceable) — Record baru duduk di `doc_status: draft` → **tidak bisa direferensikan**; nota lama “`submit` butuh permission `update`” **salah** (lihat #52/#53)                                                                                                  | 🔴 **BLOCKER**                    | ✅ Terverifikasi runtime                                                         |
| 45  | 🟡 **Sebagian 2026-09-14** — izin per-entity ✅ (S3/`public_entities`) & penyelesaian oleh kasir ✅ (#52 rute lifecycle, #44 `order` lifecycle-free) — Permukaan publik bisa `create` tapi tidak bisa `submit` → anonim menghasilkan data yang tak bisa diselesaikan. **Sisa:** klaim kepemilikan tamu (scope per-permukaan)                                              | 🔴 **BLOCKER**                    | ✅ Terverifikasi runtime                                                         |
| 46  | ✅ **Diperbaiki 2026-09-14** (`spec.NormalizeMoneyValue` + `ValidateMoneyValue` di batas API) — `money` **tidak divalidasi & tidak dinormalisasi** (3 bentuk diterima); `settings.currency` tidak diterapkan                                                                                                                                                              | **HIGH**                          | ✅ Terverifikasi runtime                                                         |
| 47  | Kontrak REST surface `/_ui/` tidak terdokumentasi — body flat, envelope respons, endpoint aksi (ditemukan dengan trial-and-error)                                                                                                                                                                                                                                         | **MEDIUM**                        | ✅ Terverifikasi runtime                                                         |
| 48  | `kind: Workspace` tidak menentukan workspace aktif; default `default` via flag — data masuk tenant yang tidak diduga, `/kafe/` tampak sehat tapi kosong                                                                                                                                                                                                                   | **MEDIUM**                        | ⚠️ Kemungkinan by design                                                         |
| 49  | **Tiga guard script Starlark tidak bisa dikompilasi** (implicit string-literal concatenation, kebiasaan Python, tidak didukung Starlark) → `create`/`update` pada `menu-item-price`, `shift`, `stock-level` **selalu gagal** `HOOK_ABORTED`                                                                                                                               | 🔴 **BLOCKER**                    | ✅ **Diperbaiki 2026-09-14** (script di-`+`; guard kini berjalan)                |
| 50  | ✅ **Diperbaiki 2026-09-14** — **`formspec validate` tidak mendeteksi script yang gagal kompilasi** — validate hijau (0 problem) padahal 3 script yang dirujuk `hooks:` tidak bisa dikompilasi                                                                                                                                                                            | **HIGH** (gerbang otomatis)       | ✅ Terverifikasi runtime · ⏳ belum diperbaiki (TODO 8.7)                        |
| 51  | **`ctx.db().query()` menolak bind parameter** — kontrak `Querier` menyatakan `query(sql, args...)` tapi `builtinQuery` hanya menerima `sql` dan tidak meneruskan argumen → `query: got 2 arguments, want at most 1`                                                                                                                                                       | **HIGH**                          | ✅ **Diperbaiki 2026-09-14** (`internal/starlark/primitive.go` + test regresi)   |
| 52  | **Aksi lifecycle `submit`/`cancel`/`amend` tidak punya rute di surface UI** — rute wildcard file `/{id}/{field}` menelannya dan membalas 403 menyesatkan (aksi karangan pun memberi error sama). Akibat: master data tak pernah bisa `submit` → karena #44, **tak ada satu transaksi pun bisa dibuat**                                                                    | 🔴 **BLOCKER**                    | ✅ **Diperbaiki 2026-09-14** (rute lifecycle di surface UI; wildcard file → 404) |
| 53  | **Rute file memakai nama entity singular untuk permission** (`file.go:451`, enam situs) — registry & generator memakai plural → pengguna yang diberi `…menu-categories.update` tetap ditolak                                                                                                                                                                              | 🟡 **MEDIUM**                     | ✅ **Diperbaiki 2026-09-14** (`permName` → plural)                               |

## Isi Folder

| File                                | Cakupan                                                                                                                                        |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `01-widget-money-time.md`           | Gap #1, #2, #28 — widget nilai uang, bentuk nilai, aritmetika/agregasi                                                                         |
| `02-media-dan-qr.md`                | Gap #3, #4, #4b — QR code, gambar produk, format `allowed_types`                                                                               |
| `03-pos-dan-public-ordering.md`     | Gap #5, #6, #7 — cart, akses anonim, privasi                                                                                                   |
| `04-multi-outlet.md`                | Gap #8, #9 — scoping outlet & penomoran                                                                                                        |
| `05-vertical-modules.md`            | Gap #13, #14, #15 — inventory, purchase, integrasi akuntansi                                                                                   |
| `06-engine-kontrak.md`              | Gap #10, #11, #12, #16, #17, #29, #38, #39 — print, relasi, widget, realtime, kontrak Report, workflow                                         |
| `07-dokumen-dan-skill.md`           | Gap #18, #19, #20, #21, #37 — drift dokumen, skill, validator lenient                                                                          |
| `08-ddl-index-dan-devtools.md`      | Gap #22, #23, #24, #27, #35, #36 — DDL, index, migration, dev tooling                                                                          |
| `09-config-dan-settings.md`         | Gap #25, #26 — bentuk `kind: Config` yang benar & `settings.currency` wajib untuk `money`                                                      |
| `10-script-dan-hook.md`             | Gap #30–#34 — guard script eksplisit, hook, deadlock SQLite, consent footprint                                                                 |
| `11-integrasi-lintas-app.md`        | Gap #40–#43 — Integrator, pemetaan payload, kepemilikan antarmuka, simetri cancel                                                              |
| `12-hasil-verifikasi-runtime.md`    | **Verifikasi runtime nyata** — Gap #44–#46 + koreksi #24 & #26                                                                                 |
| `13-kelengkapan-spec-untuk-kafe.md` | ⭐ **Kelengkapan BAHASA SPEC** — apa yang tidak bisa dinyatakan di YAML (S1–S16) + semantik yang belum ditetapkan (D1–D7)                      |
| `14-temuan-fase-0.md`               | **Verifikasi ulang Fase 0** — gap mana yang sudah tertutup (#7), mana yang terbukti masih terbuka (#22, #27, #44, #46), + **gap baru #49/#50** |
| `validate-baseline.md`              | Baseline `formspec validate` yang **diharapkan** (kini: 0 problem) + daftar yang tidak ditangkap validator                                     |
| `TODO.md`                           | ⭐ **Checklist penutupan gap** — 10 fase, indeks gap → fase, tanpa gap yang hilang                                                             |
| `decisions-needed.md`               | **D1–D7 dijawab dari kode** (Fase 0.3) — termasuk koreksi D4 (prefix `on_*` kanonik) & D6 (`submit` punya permission sendiri)                  |

## Yang Sudah Bagus (jangan dibangun ulang)

Bagian ini sering terlewat saat membuat gap list. Berikut yang **sudah tersedia
out-of-the-box** dan terbukti dari kode:

- **Upload + preview gambar** — field type `file`/`attachment` + `StorageSpec`
  (`allowed_types`, `max_size_mb`, `max_count`, `transform` resize) + widget
  `FileInput` (preview thumbnail, Replace/Remove) + route upload
  `POST /_ui/entity/{module}/{entity}/{id}/{field}`. Lihat `02-media-dan-qr.md`.
- **`richtext`** — field + widget `RichText` + sanitizer + render di DetailPage.
- **Real-time** — WebSocket dengan subscription per-resource, filter permission
  per-pesan, listener-gated publish, reconnect + backoff. Table/Kanban/Dashboard
  sudah memakainya (`realtime: true`). **KDS berbasis Kanban bisa jalan hari ini.**
- **Akuntansi (vertical `gl`)** — double-entry, `account` [reference],
  `journal-entry` [transaction], `gl-balance` [summary], + script post/reverse.
  Sudah ada sebagai App mandiri. Lihat `05-vertical-modules.md`.
- **Registry & vendoring** — `formspec module install/publish/list/uninstall`,
  `formspec sign`, `formspec verify` (checksum/tamper), trust tier, `vendors/`
  read-only, `overrides/` shadow copy.
- **Denormalisasi finansial** — `snapshot:` pada field relation `belongs_to`,
  otomatis disalin saat create/submit. Ini yang membuat harga menu bisa
  "dibekukan" di transaksi — penting untuk kafe dan **sudah ada**.
- **Master-detail split view** — `Page.layout.mode: split` + `binds`
  (`source`/`param`).
- **Wizard, Kanban, Timeline, Calendar, Report, Print(html/pdf), Theme,
  ApprovalInbox, NotificationCenter, Listing** — renderer-nya ada.
- **App publik anonim** — `access: public` memungkinkan `list`/`find`/`create`
  anonim di `/_ui/entity/`. Pola dua App (publik + privat) sudah dicontohkan di
  `examples/storefront/`.

## Catatan Penting

Repo FormSpec **sudah punya `examples/cafe/`** — modul `cafe-master`,
`cafe-order`, `cafe-report`, 16 manifest, `formspec validate` hijau. Jadi
aplikasi kafe ini adalah test case yang pernah disentuh; beberapa gap di sini
kemungkinan adalah **alasan example itu berhenti** dan tidak pernah dilanjutkan
ke POS/QR/multi-outlet.
