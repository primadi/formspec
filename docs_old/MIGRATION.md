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
| Perilaku kode/engine yang berjalan | `docs_old/spec/*` | Dokumen penerus di `docs/spec/` naik status **Outline → Draft** |
| Arsitektur visual baru (hirarki, VisualSpecKind, Renderer, PersistBackend) | `reff_docs/forma-technical-notes-contract-renderer.md` | Dokumen `docs/spec/frontend/01–04` dan `docs/spec/backend/04` naik ke Draft |
| Ops/topologi (architecture, runtimes, cli-tools), comparison, guides O2C, error-glossary | `docs/` (sudah pindah utuh 2026-07-15) | — |

Aturan tegas: `docs/` **tidak boleh** mereferensikan `docs_old/` (dicek dengan
`grep -rn docs_old docs/` → harus kosong). Kode boleh sementara menunjuk
`docs_old/spec/...` sampai S5/S9.

---

## 2. Peta Migrasi Per File

Fate: **moved** (pindah utuh, sudah dilakukan) · **rewrite** (ditulis ulang jadi dokumen baru; file lama = source material) · **split** (isinya terbagi ke beberapa dokumen baru) · **historis** (tidak dibawa; terhapus bersama docs_old).

### docs_old/spec/

| File lama | Fate | Tujuan |
|---|---|---|
| `README.md` | rewrite | `docs/README.md` + README per section (pola reading-path dipertahankan) |
| `00-kind-plane-mapping.md` | rewrite | `docs/spec/platform/03-kind-system.md` (tabel kind→plane jadi lampiran; +baris VisualSpecKind, Renderer, PersistBackend) |
| `01-overview.md` | rewrite | `docs/spec/platform/01-overview.md` (+prinsip contract-vs-renderer) |
| `02-core-basic.md` (v0.3.0) | split | Kontrak → `docs/spec/backend/01-core-basic.md`; detail Postgres/JSONB/SQL → `docs/renderers/persist-postgres/`; section migration direformulasi jadi kontrak structural-diff |
| `03-core-extended.md` (v0.2.0) | rewrite | `docs/spec/backend/02-core-extended.md` |
| `04-control-plane.md` | rewrite | `docs/spec/platform/04-control-plane.md` |
| `05-frontend.md` (v0.5.0, 603 baris) | split | `docs/spec/frontend/01–08` — lihat checklist coverage §4 |
| `06-plane-protocol.md` (v0.2.0) | rewrite | `docs/spec/platform/05-plane-protocol.md` |
| `07-marketplace.md` | rewrite | `docs/spec/platform/07-marketplace.md` (+Renderer/VisualSpecKind sebagai artifact) |
| `08-order-to-cash-tutorial.md` | **moved** | `docs/guides/order-to-cash-tutorial.md` |
| `09-order-to-cash-companion.md` | **moved** | `docs/guides/order-to-cash-companion.md` |
| `10-entity-extension.md` | split | Kontrak "uninstall bersih" → `docs/spec/backend/03-entity-extension.md`; mekanisme kolom JSONB → `docs/renderers/persist-postgres/02-schema-strategies.md` |
| `11-reference.md` | split | Glossary → `docs/reference/glossary.md`; ledger D1–D50 = **historis** (rationale relevan dilebur inline, lihat §5); Laravel map → sudah tercakup `docs/comparison/forma-vs-laravel.md`; katalog concern→kind → `docs/spec/platform/03-kind-system.md` |
| `12-datastore.md` | rewrite | `docs/spec/platform/06-datastore.md` |
| `12-sidecar-entity-primitives.md` | rewrite | merge ke `docs/runtimes/04-forma-sidecar.md` |
| `error-glossary.yaml` | **moved** | `docs/spec/backend/error-glossary.yaml` |

### docs_old/implementation/

| File lama | Fate | Tujuan |
|---|---|---|
| `README.md` | historis | Digantikan `docs/renderers/README.md` |
| `database-layer.md` | rewrite | `docs/renderers/persist-postgres/03-migration-engine.md` + `04-query-and-keys.md` |
| `api-layer.md` | rewrite | `docs/runtimes/05-engine-api-layer.md` |
| `frontend-renderer.md` | rewrite | `docs/renderers/shadcn-shell/02-derivation-engine.md` + `03-kind-renderers.md` |

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
| **S2** | `spec/frontend/01-visual-hierarchy.md`, `02-visual-spec-kind.md`, `03-renderer-kind.md` → Draft | `reff_docs/forma-technical-notes-contract-renderer.md` §1–§4, §6 |
| **S3** | `spec/frontend/04-spec-resolution-api.md` → Draft | note §5; API `/_meta` existing (`internal/api/meta.go`, `internal/ui/`) sebagai acuan v0; delta backend-agnostic dicatat di §6 dokumen ini |
| **S4** | `spec/backend/04-persist-backend.md` → Draft; reformulasi section migration + `ctx.db` | note §8; `internal/db/interface.go`, `internal/datastore/` |
| **S5** | `spec/backend/01–03` → Draft; evakuasi konten Postgres ke `renderers/persist-postgres/`; sweep referensi kode `docs_old/spec/` → path baru | `docs_old/spec/02,03,10` |
| **S6** | `spec/frontend/05-app-kinds.md`, `06-page-kinds.md`, `07-component-kinds.md`, `08-formaexpr.md` → Draft | `docs_old/spec/05-frontend.md` §3–§13 (checklist §4) |
| **S7** | `renderers/shadcn-shell/*`, `renderers/persist-postgres/*` → terisi | `docs_old/implementation/*`, kode `web/`, `internal/db`, `internal/ui` |
| **S8** | `spec/platform/*` → Draft; `guides/authoring-*` terisi; link sweep architecture/runtimes/cli-tools/guides | `docs_old/spec/00,01,04,06,07,12`; `reff_docs/*kind-system*` |
| **S9** | Retirement: audit §2 tuntas 100%, `grep docs_old` di docs/ & kode = kosong → **hapus docs_old/** (+ konfirmasi user untuk file reff_docs yang sudah tuntas ditambang) | — |

---

## 4. Checklist Split `05-frontend.md` (operasi paling berisiko)

Setiap section file lama harus punya rumah; centang saat konten terserap:

- [ ] §1 arsitektur/prinsip → `frontend/01-visual-hierarchy.md` (digantikan model 4-tier)
- [ ] §2 App/navigation/menu → `frontend/05-app-kinds.md` + `platform/02-workspace-app-module.md` (menu 2-mode)
- [ ] §3–§13 katalog kind (Form, Table, Wizard, Kanban, Dashboard, Timeline, Report, Print, Page, Theme, Widget) → `frontend/06-page-kinds.md` / `07-component-kinds.md`, di-recast sebagai instance VisualSpecKind
- [ ] Section FormaExpr → `frontend/08-formaexpr.md`
- [ ] Kontrak komponen `asset` / escape hatch → `frontend/07-component-kinds.md`
- [ ] Konvensi realtime/websocket → `frontend/04-spec-resolution-api.md`
- [ ] §7 `forma.api` / client generation → tetap di `cli-tools/03-forma-generate.md` (sudah pindah) + preamble "codegen hanya untuk Tier 2/3 dev"

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

- [ ] Sweep D-ledger untuk backend/01–04
- [ ] Sweep D-ledger untuk frontend/01–08
- [ ] Sweep D-ledger untuk platform/01–07
- [ ] Sweep D-ledger untuk renderers/*

## 6. Gap Kode yang Terungkap Selama Migrasi (JANGAN diperbaiki di tengah migrasi docs)

Daftar misalignment kode vs kontrak baru; jadi input fase restrukturisasi kode
setelah kontrak stabil:

- `internal/db` interface (`DB`/`Tx`) bocor semantik SQL (`ExecContext`, `Driver()`)
  — belum memenuhi kontrak `PersistBackend` (structural diff, query resolution, next_key).
- API `/_meta` perlu diaudit terhadap syarat backend-agnostic Spec Resolution API
  (tidak boleh bocor nama kolom fisik / path JSONB).
- `web/` belum terstruktur per tier shell/app/page/component; belum ada registry
  `VisualSpecKind`/`Renderer` di `pkg/spec`.
- Temuan positif (argumen port bertahap, bukan rewrite): `pkg/spec` terpisah bersih;
  frontend sudah runtime-interpreter; `internal/db` sudah punya 2 impl (Postgres+SQLite);
  ~47 file test terkonsentrasi di engine.

## 7. Kriteria Penghapusan docs_old/

Semua terpenuhi → hapus seluruh `docs_old/` (git tag `docs-pre-restructure-2026-07-15` mengawetkan):

1. Setiap baris di peta §2 berstatus tuntas (moved/rewrite/split selesai, atau historis).
2. Checklist §4 dan §5 tercentang penuh.
3. `grep -rn docs_old docs/` kosong dan `grep -rn docs_old cmd/ sdk/ internal/ pkg/ web/src/` kosong (referensi kode sudah menunjuk dokumen penerus).
4. Konfirmasi user.
