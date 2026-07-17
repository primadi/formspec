# Strategi Migrasi Docs — Contract vs Renderer

**Dokumen kerja internal.** Hidup di `docs_old/` (bukan `docs/`) dengan sengaja:
`docs/` baru murni eksternal-facing dan present-tense; semua materi historis dan
proses migrasi dicatat di sini. Dokumen ini dihapus bersama `docs_old/` saat migrasi selesai.

Sumber arsitektur: `reff_docs/forma-technical-notes-contract-renderer.md` (hirarki
Shell → App → Page → Component, VisualSpecKind/Renderer, Spec Resolution API,
PersistBackend seam) dan `reff_docs/forma-technical-notes-kind-system.md`.

---

## 1. Aturan Tree Otoritatif Selama Transisi

| Topik | Otoritatif hari ini | Berpindah ketika |
|---|---|---|
| Perilaku kode/engine yang berjalan | `docs/spec/*` ✅ 2026-07-16 (S8) — seluruh `docs_old/spec/00,01,02,03,04,06,07,10,12` sudah punya penerus Draft | selesai |
| Arsitektur visual baru (hirarki, VisualSpecKind, Renderer, PersistBackend) | `docs/spec/frontend/01–04`, `docs/spec/backend/04` ✅ 2026-07-15/16 (S2–S4) | selesai |
| Ops/topologi (architecture, runtimes, cli-tools), comparison, guides O2C, error-glossary | `docs/` (sudah pindah utuh 2026-07-15; link sweep S8 2026-07-16, ditemukan masih bocor — dituntaskan pada review 2026-07-16: 10 broken link + beberapa sitasi teks penomoran lama di `architecture/01,02,07,README` dan `guides/order-to-cash-*` diperbaiki) | — |

Sisa `docs_old/spec/` yang **belum** sepenuhnya terserap: bagian D-ledger
`11-reference.md` yang belum di-sweep eksplisit (lihat §5). (App/navigation/menu
`05-frontend.md` §2 sudah tuntas — lihat checklist §4, dikoreksi pada review 2026-07-16.)

Aturan tegas: `docs/` **tidak boleh** mereferensikan `docs_old/` (dicek dengan
`grep -rn docs_old docs/` → harus kosong). Kode boleh sementara menunjuk
`docs_old/spec/...` sampai S5/S9.

---

## 2. Peta Migrasi Per File

Fate: **moved** (pindah utuh, sudah dilakukan) · **rewrite** (ditulis ulang jadi dokumen baru; file lama = source material) · **split** (isinya terbagi ke beberapa dokumen baru) · **historis** (tidak dibawa; terhapus bersama docs_old).

### docs_old/spec/

| File lama | Fate | Tujuan |
|---|---|---|
| `README.md` | rewrite ✅ (S1/S8) | `docs/README.md` + README per section — semua section punya README index (diverifikasi 2026-07-16) |
| `00-kind-plane-mapping.md` | rewrite ✅ 2026-07-16 | `docs/spec/platform/03-kind-system.md` (tabel kind→plane jadi lampiran §4; +baris VisualSpecKind, Renderer, PersistBackend) |
| `01-overview.md` | rewrite ✅ 2026-07-16 | `docs/spec/platform/01-overview.md` (dipadatkan — §14 Roadmap dan sebagian besar detail §6/§9/§10/§12/§13 tidak dibawa karena sudah tercakup lebih detail di backend/01, frontend/01-03, atau bersifat historis/roadmap yang tidak masuk `docs/`) |
| `02-core-basic.md` (v0.3.0) | split ✅ 2026-07-15 | Kontrak → `docs/spec/backend/01-core-basic.md`; detail Postgres/JSONB/SQL → `docs/renderers/jsonb-persist/` (01, 02, 03, 04 — semua terisi konten evakuasi); section migration direformulasi jadi kontrak structural-diff |
| `03-core-extended.md` (v0.2.0) | rewrite ✅ 2026-07-15 | `docs/spec/backend/02-core-extended.md` |
| `04-control-plane.md` | rewrite ✅ 2026-07-16 | `docs/spec/platform/04-control-plane.md` (§1-4 baru ditulis S8; §5-8/Policy/Transparency/REPL/Emergency sudah ada dari draf sebelumnya; §6 Contracts + Backup Governance ditambahkan S8, melengkapi yang ditunda dari S4) |
| `05-frontend.md` (v0.5.0, 603 baris) | split ✅ tuntas, 2026-07-15/16 (dikonfirmasi ulang 2026-07-16) | `docs/spec/frontend/01–08` — lihat checklist coverage §4 (§2 App/navigation/menu tuntas di `platform/02-workspace-app-module.md` §4, S8) |
| `06-plane-protocol.md` (v0.2.0) | rewrite ✅ 2026-07-16 | `docs/spec/platform/05-plane-protocol.md` |
| `07-marketplace.md` | rewrite ✅ 2026-07-16 | `docs/spec/platform/07-marketplace.md` (+Renderer/VisualSpecKind/PersistBackend sebagai artifact) |
| `08-order-to-cash-tutorial.md` | **moved** | `docs/guides/order-to-cash-tutorial.md` |
| `09-order-to-cash-companion.md` | **moved** | `docs/guides/order-to-cash-companion.md` |
| `10-entity-extension.md` | split ✅ 2026-07-15 | Kontrak "uninstall bersih" → `docs/spec/backend/03-entity-extension.md`; mekanisme kolom JSONB → `docs/renderers/jsonb-persist/02-schema-strategies.md` |
| `11-reference.md` | split ✅ tuntas 2026-07-16 | Glossary → `docs/reference/glossary.md` (**Draft, terisi penuh** 2026-07-16 — definisi kanonik seluruh istilah, bersumber docs/spec baru); ledger D1–D50 = **historis, ter-sweep penuh baris-per-baris** (lihat §5 baris terakhir — 39 absorbed + 8 dilebur saat sweep + 3 obsolete); Laravel map → sudah tercakup `docs/comparison/forma-vs-laravel.md`; katalog concern→kind → `docs/spec/platform/03-kind-system.md` §3 |
| `12-datastore.md` | rewrite ✅ 2026-07-16 | `docs/spec/platform/06-datastore.md` |
| `12-sidecar-entity-primitives.md` | rewrite ✅ 2026-07-16 | merge ke `docs/runtimes/04-forma-sidecar.md` §4.3a (lima operasi entity primitive, wire protocol, praktik, gap transaksi) |
| `error-glossary.yaml` | **moved** | `docs/spec/backend/error-glossary.yaml` |

### Baru — tanpa penerus file lama (ditulis S8, dicatat pada review 2026-07-16)

| File baru | Sumber | Catatan |
|---|---|---|
| `docs/spec/platform/08-project-layout.md` | tidak ada penerus langsung; sebagian menyerap D14 (§5) | Kontrak layout project app yang dibangun di atas Forma (`forma-app.yaml`, `apps/*.yaml`, `modules/<name>/module.yaml`, `impl/` per Module) |
| `docs/architecture/08-repo-structure.md` | tidak ada penerus langsung | Peta repo framework Forma sendiri (`cmd/`, `pkg/spec/`, `internal/*`, dst.) ke dokumen spec/renderer; cross-ref ke `platform/08-project-layout.md` untuk kontrak sisi app |

### docs_old/implementation/

| File lama | Fate | Tujuan |
|---|---|---|
| `README.md` | historis | Digantikan `docs/renderers/README.md` |
| `database-layer.md` | rewrite ✅ 2026-07-16 | `docs/renderers/jsonb-persist/03-migration-engine.md` + `04-query-and-keys.md` (audit kode nyata, bukan cuma rewrite tekstual — lihat §6 gap baru) |
| `api-layer.md` | rewrite ✅ 2026-07-16 | `docs/runtimes/05-engine-api-layer.md` (Draft — audit kode `internal/api` nyata: router/middleware, `/_meta` terverifikasi backend-agnostic [menutup open question §6], wshub gap dikonfirmasi masih ada) |
| `frontend-renderer.md` | rewrite ✅ 2026-07-16 | `docs/renderers/shadcn-shell/01-architecture.md` + `02-derivation-engine.md` + `03-kind-renderers.md` + `04-theming-assets.md` — ditulis dari audit kode `web/src` nyata (agent Explore), bukan dari rencana desain 07-11 yang sudah banyak berubah |

### Sudah pindah utuh (moved, 2026-07-15)

`architecture/` (8 file), `runtimes/` (5), `cli-tools/` (5), `comparison/` (9),
`how-to-run.md` → `docs/guides/`. Catatan: file-file ini masih mengandung link
relatif ke `../spec/...` lama yang kini putus — dibereskan di S8 (link sweep),
diarahkan ke dokumen penerus di `docs/spec/`.

### Historis — tidak dibawa (terhapus bersama docs_old)

- `changelog/` (23 file) — log kerja internal; pembaca eksternal tidak butuh sejarah "fitur X diganti Y".
- `audit/` (3 file todo) — snapshot gap lama.
- `plan/` (5 file) — catatan kerja; item yang masih hidup dilanjutkan di issue/plan session, bukan di docs.
- Ledger keputusan D1–D50 dan D-ARCH-1..31 — lihat §5.

---

## 3. Rencana Sesi Konten (S2–S9)

| Sesi | Deliverable | Source material utama |
|---|---|---|
| **S2** ✅ 2026-07-15 | `spec/frontend/01-visual-hierarchy.md`, `02-visual-spec-kind.md`, `03-renderer-kind.md` → Draft | `reff_docs/forma-technical-notes-contract-renderer.md` §1–§4, §6 |
| **S3** ✅ 2026-07-15 | `spec/frontend/04-spec-resolution-api.md` → Draft | note §5; API `/_meta` existing (`internal/api/meta.go`, `internal/ui/`) sebagai acuan v0; delta backend-agnostic dicatat di §6 dokumen ini |
| **S4** ✅ 2026-07-15 | `spec/backend/04-persist-backend.md` → Draft; reformulasi section migration + `ctx.db` | note §8; `internal/db/interface.go`, `internal/datastore/` |
| **S5** ✅ 2026-07-15 | `spec/backend/01–03` → Draft; evakuasi konten Postgres ke `renderers/jsonb-persist/`; sweep referensi kode `docs_old/spec/` → path baru | `docs_old/spec/02,03,10` |
| **S6** ✅ 2026-07-15 | `spec/frontend/05-app-kinds.md`, `06-page-kinds.md`, `07-component-kinds.md`, `08-formaexpr.md` → Draft | `docs_old/spec/05-frontend.md` §3–§13 (checklist §4) |
| **S7** ✅ 2026-07-16 | `renderers/shadcn-shell/*`, `renderers/jsonb-persist/*` → terisi | `docs_old/implementation/*`, kode `web/`, `internal/db`, `internal/ui` (audit kode nyata via agent Explore — lihat gap baru §6) |
| **S8** ✅ 2026-07-16 | `spec/platform/*` → Draft; `guides/authoring-*` terisi; link sweep architecture/runtimes/cli-tools/guides | `docs_old/spec/00,01,04,06,07,12`; `reff_docs/*kind-system*` |
| **S9** | Retirement: audit §2 tuntas 100%, `grep docs_old` di docs/ & kode = kosong → **hapus docs_old/** (+ konfirmasi user untuk file reff_docs yang sudah tuntas ditambang) | — |

---

## 4. Checklist Split `05-frontend.md` (operasi paling berisiko)

Setiap section file lama harus punya rumah; centang saat konten terserap:

- [x] §1 arsitektur/prinsip → `frontend/01-visual-hierarchy.md` (digantikan model 4-tier) — S2
- [x] §3–§13 katalog kind (Form, Table, Wizard, Kanban, Dashboard, Timeline, Report, Print, Page, Theme, Widget) → `frontend/06-page-kinds.md` / `07-component-kinds.md`, di-recast sebagai instance VisualSpecKind — S6, 2026-07-15
- [x] Section FormaExpr → `frontend/08-formaexpr.md` — S6, 2026-07-15 (§ perilaku error evaluasi ditandai Open — belum ada di source lama)
- [x] Kontrak komponen `asset` / escape hatch → `frontend/07-component-kinds.md` — S6, 2026-07-15
- [x] Konvensi realtime/websocket → `frontend/04-spec-resolution-api.md` — S3
- [x] §7 `forma.api` / client generation → tetap di `cli-tools/03-forma-generate.md` (sudah pindah S1); framing "escape hatch untuk programmer hand-written" sudah ada di §intro dokumen itu (setara "Tier 2/3 dev", beda kata)
- [x] §2 App/navigation/menu → `frontend/05-app-kinds.md` (§1, §6 sudah cross-ref) + `platform/02-workspace-app-module.md` §4 (menu 2-mode — sudah ditulis lengkap: mode `module`/`custom`, `MenuItem`, batas nesting 3 level, resolusi route) — dicek ulang saat review 2026-07-16, checklist ini sebelumnya stale (menyebut "belum ditulis" padahal §8 sudah selesai)

## 5. Absorpsi Ledger D1–D50 (jangan sampai rationale hilang)

Ledger tidak dibawa sebagai dokumen; rationale yang masih relevan **dilebur
inline** ke spec penerus sebagai penjelasan desain present-tense. Prosedur per
sesi: sebelum menaikkan sebuah dokumen ke Draft, sweep `docs_old/spec/11-reference.md`
untuk entri D yang menyangkut topiknya, lebur yang relevan, centang di sini.

Anchor yang sudah diketahui: D17 (derivation engine) → `renderers/shadcn-shell/02`;
D36 (frontend spec) → `spec/frontend/*`; D43 (forma-ctl) → `cli-tools/02`;
D49 (expose default) → `spec/backend/01`. D-ARCH-1..31 tetap terlebur di
`docs/architecture/01` (sudah inline di sana). Keputusan baru dari technical note
contract-renderer (stack_family, runtime-not-codegen, PersistBackend seam,
menu 2-mode, qualifier `module/resource`) ditulis langsung sebagai isi normatif
spec baru — tanpa ledger bernomor.

- [x] Sweep D-ledger untuk backend/04 (S4, 2026-07-15) — D27 (list/find/export/backup tidak boleh license-gated) dan D31 (credible exit guarantee) sudah dilebur ke `spec/backend/04` §3; D45 (tangga eskalasi `ctx.db` → `kind: Migration` (wajib `tenant_id`) → provider app/Service) dilebur ke §5. D41/D44/D46 (governance backup: siapa boleh restore, tanda tangan pemilik, sandboxing native module) **sengaja belum dilebur** — itu kontrak Control Plane/Workspace Owner, bukan kontrak interface PersistBackend; ditunda ke sweep `platform/04-control-plane.md` di S8.
- [x] Sweep D-ledger untuk backend/01–03 (S5, 2026-07-15) — D20 (permission model), D32 (idempotency & optimistic concurrency), D49 (exposure deny-by-default) dilebur ke `backend/01` §5/§8; D33 (deliver vocabulary closed) dan D35 (Subscription pattern) dilebur ke `backend/01` §7 dan `backend/02` §3. D38 (capability footprint, halaman bukan otorisasi) sudah dilebur di S3 (`frontend/04`, dicatat di baris itu). D17/D18 (kind system 3-layer, KindDefinition) ditunda ke `platform/03-kind-system.md` (S8) — di luar cakupan backend/01-03.
- [x] Sweep D-ledger untuk frontend/01–03 (S2, 2026-07-15) — `grep -n "Shell\|VisualSpecKind\|stack_family\|Renderer kind\|renderer_contract" docs_old/spec/11-reference.md` kosong; konsep Shell/VisualSpecKind/Renderer baru diperkenalkan di `reff_docs/forma-technical-notes-contract-renderer.md`, tidak ada di ledger D-lama untuk dilebur.
- [x] Sweep D-ledger untuk frontend/05–08 (S6, 2026-07-15) — D17 (derived by default) → `06-page-kinds.md` §9; D33 (closed vocabulary/deliver) sudah di backend (S5); D36 (arsitektur frontend v0.5.0 lengkap — interpretive renderer, component contract, Print, Theme, Dashboard/Widget, Report, Wizard/Kanban/Timeline) tersebar ke seluruh `05-08`; D10 (fitur Frappe/PocketBase core scope) tidak butuh rumah spesifik — sudah terwujud lewat breadth kind yang ditulis, bukan pernyataan normatif tersendiri; D14 (tiga jenis file + asset escape hatch) → `07-component-kinds.md` §4 dan sudah di `platform/08-project-layout.md`; D24 (manifest tak terenkripsi, provenance signed) disentuh ringan di `05-app-kinds.md` §5 (Theme "signed") — kedalaman penuhnya (kenapa tak dienkripsi, proteksi IP vendor) ditunda ke `platform/07-marketplace.md` (S8). D20/D35/D38 sudah dilebur di S3/S5.
- [x] Sweep D-ledger untuk platform/01–07 (S8, 2026-07-16) — mayoritas D-number relevan (D2/D3 Control Plane scope, D8 license token, D11 Verified Badge, D20/D21/D22/D25/D27/D29/D30/D31/D37/D39/D40/D41/D42/D44/D46/D47/D51–59) sudah tersentuh secara alami karena §01,04,06,07,12 docs_old/spec ditulis ulang penuh (bukan cuma disweep berbasis grep seperti frontend/backend) — dokumen sumbernya sendiri sudah encode rationale D-nya dalam prosa. **Belum ada verifikasi baris-per-baris eksplisit** bahwa ke-20-an D-number itu 100% masuk; kalau ditemukan lubang saat baca ulang, catat di sini. D-number yang genuinely belum disentuh: rincian UX D44 (passkey/WebAuthn flow) dan D30 (alasan blockchain ditolak) — keduanya detail elaborasi, bukan kontrak inti yang hilang.
- [x] Sweep D-ledger untuk renderers/* — dikonfirmasi saat verifikasi penuh 2026-07-16: D-ledger renderers ikut ter-sweep lewat baris backend/frontend di atas, tidak ada entri tersisa.
- [x] **Verifikasi baris-per-baris D1–D50 (2026-07-16, review pass).** Hasil: 39 absorbed, 3 obsolete (D1 Dapr-foil, D7 disuperseksi D29, D9 konvensi repo di CLAUDE.md), 8 missing → **semuanya sudah dilebur pada pass yang sama**: D5 (closed set primitive) → `platform/06-datastore.md` §2; D23+D29 (tenancy-blind, workspace satu-satunya model, tanpa akses lintas-workspace) → `platform/02` §1; D34 (git source of truth) → `platform/08` §2; D37 (identitas workspace-level, membership per-App) → `platform/02` §8 baru; D44 (passkey/WebAuthn default owner key) → `platform/04` §4; D46 (scope ctx.db default + gerbang trust-tier impl) → `backend/01` §5 + `platform/07` §2; D50 (radix-tree router, internal dispatch) → `backend/01` §8. Plus: xref stale "Enam Primitive" di `platform/06` §1 diperbaiki; Config → `backend/01` §10 baru (menutup placeholder §? di `platform/03`). Ledger D1–D50 kini tuntas ter-absorb/obsolete tanpa sisa.

## 6. Gap Kode yang Terungkap Selama Migrasi (JANGAN diperbaiki di tengah migrasi docs)

Daftar misalignment kode vs kontrak baru; jadi input fase restrukturisasi kode
setelah kontrak stabil:

- `internal/db` interface (`DB`/`Tx`) bocor semantik SQL (`ExecContext`, `Driver()`)
  — belum memenuhi kontrak `PersistBackend` (structural diff, query resolution, next_key).
- API `/_meta` perlu diaudit terhadap syarat backend-agnostic Spec Resolution API
  (tidak boleh bocor nama kolom fisik / path JSONB).
- **`internal/api/wshub.go` broadcast ke seluruh koneksi satu workspace tanpa
  filter per-pesan** (ditemukan S3, 2026-07-15) — kontrak realtime
  (`spec/frontend/04-spec-resolution-api.md` §5) mensyaratkan caller cuma
  menerima event kalau punya permission `view` entity itu, dievaluasi per
  pesan. `events.Hub.Broadcast` cuma tahu `workspaceID`, tidak tahu permission
  penerima per koneksi — client dipercaya mengabaikan pesan tak relevan.
  Dicatat di `spec/frontend/04` §7 juga.
- `web/` belum terstruktur per tier shell/app/page/component; belum ada registry
  `VisualSpecKind`/`Renderer` di `pkg/spec`.
- **Mutasi entity dan penulisan outbox tidak atomik** (ditemukan S7, 2026-07-16)
  — `DB.BeginTx` tidak pernah dipanggil di `internal/db`/`internal/api`/`internal/action`;
  `OutboxStore.Enqueue` jalan setelah mutasi commit, sebagai langkah terpisah
  best-effort (error cuma di-log). Kontrak durabilitas "publisher durable"
  (`spec/backend/01-core-basic.md` §7) belum terpenuhi. Dicatat di
  `renderers/jsonb-persist/01-architecture.md` §3–§4 juga.
- **Natural key counter juga tidak dibungkus transaksi bersama insert-nya**
  (S7) — gap-free mode (lock ditahan sampai commit) belum punya mekanisme;
  gap selalu mungkin, bukan cuma saat retry. Dicatat di
  `renderers/jsonb-persist/04-query-and-keys.md` §2.
- **PK SQLite bukan UUID v7** (S7) — `integer PRIMARY KEY AUTOINCREMENT`,
  menyimpang dari kontrak `spec/backend/01-core-basic.md` §2 yang mewajibkan
  UUID v7 tanpa pengecualian backend. Dicatat di `jsonb-persist/01-architecture.md` §1.
- **Filter operator hanya 9 dari 12 yang didaftarkan kontrak** (S7) —
  `between`/`ilike`/`null`/`notnull` belum diimplementasikan; filter/sort juga
  cuma bisa menyasar field ber-`index`/`unique`/`natural_key`, tidak ada
  fallback query JSONB mentah untuk field lain. Dicatat di
  `jsonb-persist/04-query-and-keys.md` §1.
- **Generated column untuk field entity dasar salah dialek di Postgres**
  (S7) — selalu memakai sintaks `json_extract` (SQLite), bukan `->>'` yang
  valid di Postgres; tidak ketahuan karena test DDL Postgres cuma menguji
  nama schema. Dicatat di `jsonb-persist/02-schema-strategies.md` §3.
- **Uninstall extension (`DROP COLUMN`) dan baca/tulis runtime kolom
  extension (`ExtensionStore`) belum diimplementasikan** (S7) — DDL
  `ADD COLUMN ext_*` dan registry `forma_extensions` nyata dan jalan saat
  extension dipasang, tapi tidak ada jalur uninstall maupun jalur baca/tulis
  data extension saat request. Dicatat di `jsonb-persist/02-schema-strategies.md` §2.
- **Resolusi target relasi lintas-module naif** (S7) — `ValidateRelationTargets`
  memakai module milik entity sendiri + pluralisasi tambah-`s`, bisa
  meloloskan relasi lintas-module/plural tak beraturan dari guard
  referenceability. Dicatat di `jsonb-persist/03-migration-engine.md` §2.
- **Renderer `web/`: sebagian besar Fase 4.F5/F6 masih placeholder atau kode
  mati** (ditemukan S7, audit penuh `web/src` vs rencana desain 07-11) —
  drag-drop Kanban tidak ada (klaim komentar file salah, `@dnd-kit` terpasang
  tapi nol pemakaian); Dashboard/Widget cuma placeholder (tanpa fetch data);
  component contract `asset`/`forma.ui` belum ada sama sekali; realtime
  (`realtime: true`) tidak ada implementasi apa pun (bukan cuma "belum
  websocket", benar-benar tidak ada polling juga); `OverlayHost`
  (modal/drawer) dan `engine/registry.tsx` adalah kode mati, tidak terhubung
  jalur hidup mana pun; widget field untuk `date`/`datetime`/`json`/`child`
  tidak ada, diam-diam jatuh ke `TextInput` polos; kolom Table dipotong ke 8
  field pertama tanpa limpahan ke detail (data hilang dari tampilan, bukan
  cuma "belum ditata"); `TableRenderer` hardcode prefiks `/_admin` walau
  dirender di surface `app`; baris totals `Report` dihitung tapi tidak pernah
  dirender. Rincian lengkap per kind di `renderers/shadcn-shell/01-04`.
- Temuan positif (argumen port bertahap, bukan rewrite): `pkg/spec` terpisah bersih;
  frontend sudah runtime-interpreter; `internal/db` sudah punya 2 impl (Postgres+SQLite);
  ~47 file test terkonsentrasi di engine.

## 7. Kriteria Penghapusan docs_old/

Semua terpenuhi → hapus seluruh `docs_old/` (git tag `docs-pre-restructure-2026-07-15` mengawetkan):

1. Setiap baris di peta §2 berstatus tuntas (moved/rewrite/split selesai, atau historis). ✅ 2026-07-16.
2. Checklist §4 dan §5 tercentang penuh. ✅ 2026-07-16 (termasuk verifikasi baris-per-baris D1–D50).
3. `grep -rn docs_old docs/` kosong dan `grep -rn docs_old cmd/ sdk/ internal/ pkg/ web/src/` kosong (referensi kode sudah menunjuk dokumen penerus). ✅ untuk `docs/` (bersih, diverifikasi ulang 2026-07-16); referensi di kode (14 file — sdk/, cmd/forma-ctl, pkg/spec, internal/ui/registry.go) **masih ada**, diizinkan sementara sampai fase kode.
4. Konfirmasi user. **Ditunda atas instruksi eksplisit user** ("docs_old jangan dihapus dulu", 2026-07-16) — kriteria teknis 1-3 terpenuhi tapi retirement fisik `docs_old/` belum dieksekusi.

## 8. Sapuan `reff_docs/` (2026-07-16, di luar cakupan asli §1-7)

`reff_docs/` (16 file markdown + `examples/`) tidak pernah eksplisit disapu penuh sebelumnya — cuma 2 file (`forma-technical-notes-contract-renderer.md`, `forma-technical-notes-kind-system.md`) jadi source material tercatat untuk S2/S8. Diminta user untuk disapu dengan ketelitian sama seperti D-ledger. Hasil per kelompok file:

- **5 draft versi lama** (`forma-core-basic-v0.2.0-draft.md`, `Forma-core-extended-v0.2.0-draft.md`, `forma-frontend-spec-v0.4.0-draft.md`, `forma-control-spec-v0.1.0-draft.md`, `forma-plane-protocol-v0.1.0-draft.md`) — ~34 temuan "dropped-but-relevant" (mekanisme yang ada di draft lama, hilang di `docs_old` maupun `docs/` baru — bukan supersession sengaja). Semua dilebur: sandbox limits Starlark, locality-routing, idempotency prepare, async job wire+webhook callback, `kind: Api` override, field-level security/computed fields, Validation Levels 4-6, Hook Spec penuh, Storage Spec file field, Query Builder, rate limiting, LB strategies+circuit breaker, `ctx.secrets`, Transport & Identitas plane-protocol (major — gRPC+HTTPS binding, instance cert lifecycle), revocation fast-path (juga memperbaiki broken xref di `architecture/03-deployment-flow.md`), complete-vs-delta snapshot, evidence-channel rationale, policy-input enumeration, key-rotation chain, RFC 3161 timestamp, backup consistency+outbox reconciliation, `forma suspend scripts --all`, REPL production semantics, dan lainnya.
- **7 rationale/technical note** — mayoritas positioning/historis, benar dikecualikan (`Mengapa-Forma-Harus-Ada.md` penuh, sebagian besar `Kedaulatan-Data.md`/`monetisasi.md`). 4 temuan normatif dilebur: denormalisasi field finansial, resolusi `today` dari kalender bisnis, unit lisensi App-selalu-gratis/Module-satu-satunya-unit-lisensi + gate produksi-saja, blackbox DB roles (`forma_ops_backup`/`forma_ops_ddl`), dynamic short-lived DB credentials. `forma/mcp` (Katalog-Aplikasi.md §4) dicatat sebagai proposal belum diputuskan — sengaja tidak diserap, ditandai di sini supaya tidak hilang diam-diam saat arsip dihapus.
- **Foundation-Document + O2C Companion** — nyaris seluruhnya sudah terserap; 1 temuan aksi nyata: klaim atomicity outbox yang belum dikoreksi di `order-to-cash-companion.md` (paralel dengan koreksi yang sudah dilakukan di tutorial) — **sudah diperbaiki**.
- **`examples/`** (5 app contoh: Customer, GL, Inventory, Midtrans, Order-to-Cash) — sudah digantikan `verticals/` + `order-to-cash-tutorial.md`. Ditemukan 1 gap besar: seluruh Starlark script-authoring API (`execute()`, `resource.set/.save`, query builder dari script, `ok()/fail()`, resolusi `ref` native) tidak pernah dikontrak normatif di mana pun — **ditutup** lewat file baru `docs/spec/backend/06-script-runtime.md`. Beberapa keyword minor (`emits`, `sequence_field`, `call: async` Service, `retry.initial_delay_ms`, guard builtin) juga dilebur ke `02-core-extended.md`/`01-core-basic.md`.

**Hasil:** `reff_docs/` kini fully-mined — tidak ada rationale/mekanisme normatif tersisa yang belum tercermin di `docs/`. File baru dari sapuan ini: `docs/spec/backend/06-script-runtime.md`. Section baru signifikan di `02-core-extended.md` (§12-19), `04-control-plane.md`, `05-plane-protocol.md` (§2 baru + renumbering §3-6, seluruh xref lintas-file diperbaiki), `06-datastore.md` §8, `04-resource-registration.md` §6-7, `02-workspace-app-module.md` §9. `reff_docs/` **belum dihapus**, sama seperti `docs_old/` — retirement fisik menunggu instruksi user.
