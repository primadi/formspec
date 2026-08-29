# formspec — CLI Reference

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0 (dokumen) — binary-nya sendiri FSL (open source)

> `formspec` adalah CLI utama untuk App/Module Owner dan Developer — satu binary (`cmd/formspec`), banyak subcommand. Semua verb di dokumen ini adalah subcommand dari **satu proses `formspec`**, bukan binary terpisah. Untuk CLI darurat Platform Operator, lihat [`04-formspec-ctl.md`](04-formspec-ctl.md). Dokumen ini normatif — perilaku setiap verb didefinisikan di sini secara lengkap; implementasi kode mengikuti dokumen ini, bukan sebaliknya.

---

## 1. Ringkasan Verb

| Kategori                       | Verb                                                                                       |
| ------------------------------ | ------------------------------------------------------------------------------------------ |
| **Deployment**                 | `apply`, `diff`, `delete`, `get`, `describe`, `validate`, `check [--fix]`, `promote`       |
| **Scaffolding**                | `new <kind>`                                                                               |
| **Dev loop**                   | `dev`, `repl`                                                                              |
| **Codegen**                    | `generate`                                                                                 |
| **Data lifecycle**             | `migrate`, `seed`, `backup create\|inspect`, `restore`                                     |
| **Data archival**              | `archive run\|view\|restore-batch`                                                         |
| **Distributed workflow**       | `saga list\|resolve`                                                                       |
| **Marketplace & signing**      | `module list\|install\|uninstall\|publish`, `sign`, `override adopt\|diff\|list`, `verify` |
| **Scripting**                  | `script validate\|test`                                                                    |
| **Emergency (Resource Plane)** | `freeze`, `rollback`, `lock workspace`                                                     |
| **Ops**                        | `workspace create`, `logs`                                                                 |

---

## 2. Deployment

### `formspec apply`

Satu-satunya cara mendaftarkan YAML manifest ke Control Plane (lihat [`docs/architecture/03-deployment-flow.md`](../architecture/03-deployment-flow.md), [`docs/runtimes/01-formspec-ctl.md`](../runtimes/01-formspec-ctl.md) §5).

```bash
formspec apply -f myapp/
formspec apply -f myapp/ --watch          # hot-reload, debounce 500ms
```

Di belakang layar: walk direktori spec (skip `impl/`, hidden dir, `node_modules`) → kirim `.yaml`/`.yml`/`.star` sebagai payload ke `POST /v1/artifacts` → response `{artifact_id, version, sha256}`.

### `formspec diff`

Bandingkan spec lokal dengan state yang sudah ter-deploy di Control Plane, tanpa mengubah apapun.

```bash
formspec diff -f myapp/
# Menunjukkan: field yang ditambah/dihapus, action baru, permission yang berubah
```

### `formspec get` / `formspec describe`

Ambil resource yang sudah terdaftar — pola mirip `kubectl get`/`kubectl describe`.

```bash
formspec get document invoice                # ringkas: nama, versi, status
formspec describe document invoice           # detail: field, action, state machine, permission
```

### `formspec delete`

Hapus resource dari deployment (menandai artifact versi berikutnya tanpa kind tersebut).

```bash
formspec delete document old-report --confirm
```

### `formspec validate`

Validasi tanpa mendaftarkan — dry-run.

**Status: ✅ sebagian** — dua lapis sudah jalan (lihat di bawah); honesty
scan Starlark masih roadmap.

```bash
formspec validate --spec myapp/spec             # default: ./spec
formspec validate --spec myapp/spec --no-schema # engine loader saja
formspec validate --spec myapp/spec --schema schemas/  # paksa pakai schema dir lokal ini
formspec validate --spec myapp/spec --schema-refresh   # re-fetch dari registry walau sudah ter-cache
```

**Versi schema dari `apiVersion`:** tanpa `--schema`, `formspec validate`
membaca versi spec dari `apiVersion` tiap manifest (`formspec.dev/v1`) dan
memakai JSON Schema dari **schema registry** (default
`https://schemas.formspec.dev`; override `FORMSPEC_SCHEMA_REGISTRY` atau
`schema-registry:` di `formspec-app.yaml`). Schema di-cache lokal di
`os.UserCacheDir()/formspec/schemas/<version>` — versi spec baru tidak perlu
install ulang CLI. `--schema <dir>` memaksa pakai folder `schemas/` lokal
(tanpa versioning). `--schema-refresh` mengulang fetch dari registry meski
sudah ter-cache. Sumber yang dipakai dicetak di baris pertama output
(`schema: v1 (registry <url>, cache <dir>)` atau `schema: <dir> (local
override)`).

Dua lapis, keduanya dilaporkan per manifest:

1. **Engine loader** (`internal/manifest`) — ground truth apa yang `formspec dev` /
   `formspec apply` terima: error parse YAML dan validasi dalam Entity (expose,
   lifecycle, relation, state machine, `transaction_date`, reserved fields, …).
   Ini adalah hard gate.
2. **JSON Schema** (`schemas/kinds/*.schema.json`) — kontrak untuk SEMUA kind
   (App, Module, Form, Workflow, Table, …) yang belum di-deep-validate loader.
   Menangkap sintaks usang seperti `expose: all` atau Workflow dengan
   `states`/`transitions`.

Catatan: lapis schema lebih ketat dari engine untuk konstruk shorthand yang
belum bisa diekspresikan generator schema — mis. `guard: "..."` (string) vs
`guard: { expression: ... }`, atau `render: drawer` vs `render: { mode: drawer }`
(`GuardDecl`/`FormRenderDecl` punya `UnmarshalYAML` scalar+map, schema cuma
mengekspresikan bentuk objek). Gunakan bentuk objek untuk lolos schema.

Exit code 1 jika ada manifest gagal di salah satu lapis.

Roadmap (todo 3.1.1): honesty scan untuk script Starlark — undeclared usage →
error, declared-but-unused → warning, `ctx.environment` branching → warning.

`formspec validate` **tidak pernah memberi grant** — ia cuma verifikator kejujuran otomatis atas deklarasi `required_permission`/`uses` setiap action terhadap kode sungguhan, bukan sumber kebenaran permission itu sendiri. Model permission lengkap (kelima jenis impl, kenapa grant tidak pernah diturunkan dari pemakaian): [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §5. Aturan environment binding pada business logic (kenapa `ctx.environment` hanya untuk logging, bukan percabangan bisnis): [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §8. Dijalankan tiap PR sebagai gate CI.

### `formspec schema`

Kelola cache JSON Schema versi lokal. Schema di-fetch dari registry (default
`https://schemas.formspec.dev`) dan di-cache di
`os.UserCacheDir()/formspec/schemas/<version>`. `formspec validate` dan
`formspec init` memakai cache yang sama.

```bash
formspec schema fetch v1                  # fetch/cache versi v1 (default v1)
formspec schema fetch v1 --out schemas    # + salin ke folder schemas/ project
formspec schema update v1                 # force re-fetch dari registry
formspec schema list                      # daftar versi yang ter-cache
formspec schema clear                     # hapus seluruh cache
```

Registry bisa di-override via env `FORMSPEC_SCHEMA_REGISTRY` atau
`schema-registry:` di `formspec-app.yaml`.

### `formspec check [--fix]`

Analisis statis menyeluruh atas satu project — melampaui `validate` (yang
per-manifest): resolusi lintas-file dan lintas-module dalam satu workspace.
Wajib melaporkan minimal:

```bash
formspec check -f myapp/
# unresolved varname di script (referensi field/identifier yang tidak ada) → error
# FormSpecExpr mereferensi field yang tidak ada di skema                       → error
# akses lintas-module yang dipakai tapi belum dideklarasikan/di-approve      → error
# deklarasi lintas-module yang tidak pernah dipakai                          → warning
```

`formspec check --fix` memperbaiki apa yang bisa diperbaiki otomatis: menambah
deklarasi `depends_on`/`uses` yang kurang (setelah konfirmasi interaktif —
penambahan deklarasi adalah perluasan footprint consent, tidak pernah
diam-diam), dan menghapus deklarasi yang tidak terpakai. Error kelas
unresolved-reference **menggagalkan `formspec apply`** — inilah yang menjamin
error referensi FormSpecExpr/script tidak mungkin muncul di runtime
([`../spec/frontend/08-formspec-expr.md`](../spec/frontend/08-formspec-expr.md),
[`../spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §7).

### `formspec promote`

Promosikan artifact yang **sama** (checksum diverifikasi identik) dari satu
environment ke environment lain — tanpa build/sign ulang. Mengikuti siklus
Sign → Apply → Approve → Promote; approval memakai Policy environment tujuan.

```bash
formspec promote myapp --from staging --to production
```

Kontrak lengkap (verifikasi checksum, gate re-consent, chain transparency
log): [`docs/spec/platform/10-deployment-operations.md`](../spec/platform/10-deployment-operations.md)
§5 dan [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md)
§2–3.

---

## 3. Scaffolding

### `formspec new <kind>`

Scaffold boilerplate untuk kind tertentu — anak tangga kedua dari empat anak tangga yang mengurangi verbositas YAML:

1. JSON Schema per kind (dipublikasikan di `formspec.dev/schemas`) + LSP — autocomplete, hover docs, validasi realtime; nyaris gratis berkat format seragam `apiVersion/kind`.
2. **`formspec new <kind>`** — scaffold CLI (dokumen ini).
3. Editor visual di admin panel (mirip DocType editor Frappe) — **menulis YAML ke file/PR, bukan ke database tersembunyi**; git tetap jadi source of truth.
4. Agent Skill — spec editor untuk AI.

```bash
formspec new app tokoku                # scaffold App baru
formspec new document invoice           # scaffold Document + field dasar
```

---

## 4. Dev Loop

### `formspec dev`

Development server — satu perintah untuk menjalankan backend API + SPA frontend.
SPA sudah embedded dalam binary (`//go:embed renderers/react-shadcn/dist/*`), tidak perlu npm.

```bash
# Single process — API + SPA di :8080
formspec dev --spec ./my-app/spec

# Dengan Vite HMR (edit frontend)
formspec dev --spec ./my-app/spec --dev-ui

# Auto-detect config file (formspec-app.yaml)
formspec dev
```

**Behavior:**

- Membaca YAML manifests dari `--spec` (default: `./spec`)
- Generate tabel database sesuai entity spec
- Serve REST API di `--addr` (default: `:8080`)
- Serve SPA (embedded atau dari `--web-dir`)
- Auto-detect runtime dari project files (`composer.json` → PHP, dll.)
- `--dev-ui`: spawn Vite HMR (cari `renderers/react-shadcn/` dari CWD atau module cache)
- `--force` implied oleh `--dev` / `--dev-ui`

**Flag referensi:**

| Flag             | Default                    | Fungsi                                                    |
| ---------------- | -------------------------- | --------------------------------------------------------- |
| `--spec`         | `./spec`                   | Path ke direktori YAML manifests                          |
| `--dsn`          | `sqlite:.formspec/data.db` | Database DSN                                              |
| `--addr`         | `:8080`                    | REST API listen address                                   |
| `--listen`       | `none`                     | Ctx listener mode: `none`, `local_http`, `unix_socket`    |
| `--app-endpoint` | `none`                     | App endpoint mode: `none`, `local_http`, `unix_socket`    |
| `--runtime`      | auto-detect                | Runtime auto-detect (php/python/node)                     |
| `--dev-ui`       | `false`                    | Start Vite HMR (implies `--dev`)                          |
| `--dev`          | `false`                    | Dev mode (auth bypass)                                    |
| `--force`        | `false`                    | Kill previous instance. Implied oleh `--dev` / `--dev-ui` |
| `--web-dir`      | auto-detect                | Override SPA directory                                    |
| `--state-dir`    | `.formspec`                | State directory (auto-create)                             |

**Runtime auto-detect:**

| File di CWD                           | Runtime          |
| ------------------------------------- | ---------------- |
| `composer.json`                       | php              |
| `package.json`                        | node             |
| `pyproject.toml` / `requirements.txt` | python           |
| `go.mod`                              | local (Go)       |
| (none)                                | local (API-only) |

Referensi lengkap flag, mode `--listen`/`--app-endpoint`, dan arsitektur proses: [`01-formspec-dev.md`](01-formspec-dev.md).

### `formspec repl`

Console Starlark interaktif dengan akses `ctx.*` penuh — fitur first-class (bukan alat debug darurat sekali pakai), termasuk sebagai permukaan untuk AI Agent Skill debugging.

```bash
formspec repl --environment staging
>>> invoice.load("inv-001")
>>> ctx.db.query("...")
```

Scope environment policy (tabel akses per profil environment, jaminan "bukan superuser shell"): [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) §7.

---

## 5. Codegen

### `formspec generate`

Menurunkan typed client/server types (Go; TypeScript untuk frontend), konstanta permission/enum, dan dokumen OpenAPI — dari manifest sebagai satu-satunya source of truth. **Kode hasil generate tidak pernah diedit manual.**

```bash
formspec generate --spec ./spec --out ./src/generated/formspec-client.ts   # implemented
formspec generate --lang go,typescript                                  # go: not implemented yet
formspec generate --openapi > api-spec.json                             # not implemented yet
```

**Status:** `--lang typescript` implemented (`cmd/formspec/generate.go`) — lihat [`03-formspec-generate.md`](03-formspec-generate.md) untuk referensi lengkap dan panduan pemakaian di frontend (termasuk `@formspec/client`, runtime SDK-nya). `--lang go` dan `--openapi` belum dibangun.

---

## 6. Data Lifecycle

### `formspec migrate`

Verb CLI untuk migrasi structural — migrasi sendiri **fully automatic dari Document diff** (bukan hand-written); `formspec migrate` adalah cara memicu/inspeksi proses itu, bukan tempat menulis migrasi (migrasi custom pakai `kind: Migration`, DDL-only, DML ditolak runtime).

```bash
formspec migrate plan     # tampilkan DDL yang akan dijalankan, tanpa eksekusi
formspec migrate apply    # eksekusi (biasanya otomatis lewat formspec apply)
```

Rename field wajib dideklarasikan lewat `renamed_from` pada field — kalau tidak, diff menafsirkannya sebagai drop+add; penghapusan field butuh dua langkah (deprecate, lalu remove) lintas dua versi apply. Backfill data adalah urusan migrasi tipe data (scripted, run/rollback per versi), bukan migrasi structural.

### `formspec seed`

Jalankan seeder & factory (`formspec/seed` official module) untuk data dev/testing.

```bash
formspec seed --module billing
```

### `formspec backup create|inspect` / `formspec restore`

Jaminan format backup/restore (credible exit guarantee, kenapa operasi baca/ekspor tidak boleh license-gated): [`docs/spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) §3.

```bash
formspec backup create --full                       # atau --incremental, --filter <query>
formspec backup inspect backup-2026-07-10.tar

formspec restore --from backup-2026-07-10.tar \
  --map-resource old-customer=new-customer \      # remap saat konflik ID
  --conflict remap \                              # skip | overwrite | remap (UUID+FK di-remap)
  --dry-run                                       # laporan kompatibilitas dulu
```

File storage ikut ter-backup; summary/agregat tidak (bisa dihitung ulang). Transform per-record via script Starlark saat restore. `restore` yang meng-overwrite data yang sudah ada wajib tanda tangan pemilik workspace atau delegasi eksplisit ber-scope `backup.restore`, selalu tercatat di transparency log.

---

## 7. Data Archival

### `formspec archive run|view|restore-batch`

Hanya **transaction** (`characteristic: transaction`) yang diarsipkan penuh; **master** yang direferensikan cuma di-snapshot "as-of" tanggal arsip (baris master di production tetap utuh, ditandai `locked_for_deletion: true` selama masih direferensikan arsip).

```bash
formspec archive run --max-age 3y --dry-run    # tampilkan rencana, minta konfirmasi operator
formspec archive run --max-age 3y              # eksekusi: tulis Parquet, set locked_for_deletion,
                                             # hapus baris transaction dari production
formspec archive view --batch-id archive-2021-2023   # query langsung Parquet, tanpa live DB
formspec archive restore-batch --batch-id archive-2021-2023 --target staging
```

Format penyimpanan:

```
archive-2021-2023.parquet/
  manifest.yaml           # archive_date, max_age, record_count
  transactions/           # invoices.parquet, journal_entries.parquet, ...
  masters/                # snapshot as-of archive_date: customers.parquet, ...
```

Restore **hanya ke staging**, restore dependency-ordered, **selective per-document restore tidak didukung** (risiko corrupt state). Konfigurasi retensi lewat `retention:` di `formspec.yaml` (`archive_after`, `strategy: cold_storage|delete`, `destination`).

---

## 8. Distributed Workflow (Saga/Compensation)

### `formspec saga list|resolve <id>`

Antrian intervensi manual untuk `compensation-failure-log` (resource `formspec.core`, `persist.category: compliance`):

| Sub-status            | Arti                                              | Tindakan benar                                                                                      |
| --------------------- | ------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `compensation_failed` | Step gagal, undo dicoba, undo juga gagal          | Manusia perbaiki manual — state sudah diketahui                                                     |
| `outcome_unknown`     | Tidak diketahui apakah step berhasil, retry habis | Manusia **verifikasi state aktual dulu** — tombol retry/compensate otomatis TIDAK boleh ditampilkan |

```bash
formspec saga list --status outcome_unknown
formspec saga resolve saga-abc123 --action confirm-succeeded  # atau --action compensate-now
```

Tidak ada retry otomatis tanpa batas — kalau butuh manusia, sistem tidak berpura-pura bisa menyelesaikan sendiri.

---

## 9. Marketplace & Signing

### `formspec module list|install|uninstall`

```bash
formspec module install billing-pro --from registry.formspec.dev
# Menampilkan ModuleFootprint (aggregate required_permission + uses) untuk consent SEBELUM install
# Default: fetch ke vendors/, catat formspec.lock, tulis entri ter-comment (nonaktif) di App manifest.

formspec module install billing-pro --from registry.formspec.dev --use
# Langsung menulis entri ter-uncomment (aktif) — lewati langkah aktivasi manual.
```

Model folder (`vendors/` read-only), alias otomatis saat konflik nama, dan
format marker aktivasi ada di
[`../spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md)
§6 — **terimplementasikan** (todo 13.1, 2026-08-28).

### `formspec override adopt|diff`

```bash
formspec override adopt stripe-connector Form checkout-form
# Copy spec asli ke overrides/, catat checksum sumber ke formspec.lock (shadow copy)

formspec override diff stripe-connector Form checkout-form
# Bandingkan shadow copy lokal vs versi vendor upstream saat ini
```

Shadow copy hanya berlaku untuk kind presentation (`Form` dan instance
`VisualSpecKind` seperti `Table`/`Kanban`) — bukan `Entity`/`Service`/
`Workflow`. Detail whitelist dan deteksi drift ada di
[`../spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md)
§6.4 — **terimplementasikan** (todo 13.2, 2026-08-28).

> Referensi lengkap seluruh verb registry (termasuk `module publish` dan
> `formspec sign keygen|sign|verify`): [`../registry/03-cli-reference.md`](../registry/03-cli-reference.md).

### `formspec sign`

Signing module ed25519 (todo 13.3.6, terimplementasikan):

```bash
formspec sign keygen --out ~/.formspec/keys --name acme
formspec sign <module-dir> --key ~/.formspec/keys/acme.key
formspec sign verify <module-dir> --signature <b64|file> --public-key <pub.file>
```

Payload yang ditandatangani adalah tree checksum module — nilai yang sama
dicatat di `formspec.lock` dan registry.

Integrator (cross-boundary call) yang idempotency-nya tidak `true` **ditolak** `formspec apply` — action target harus `idempotent: true` untuk dipakai lintas boundary.

---

## 10. Scripting

### `formspec script validate|test`

```bash
formspec script validate invoice.star     # sandbox check: 5000ms/64MB/100k iterasi, no network/fs/subprocess
formspec script test invoice.star --fixture fixtures/invoice_submit.json
```

---

## 11. Emergency (Resource Plane Side)

Perintah darurat yang dijalankan **App Admin yang diotorisasi**, di sisi Resource Plane (bukan Platform Operator — itu `formspec-ctl`, lihat [`04-formspec-ctl.md`](04-formspec-ctl.md)):

```bash
formspec freeze --reason "..."
formspec rollback --since 1h --all
formspec lock workspace <name> --reason "..."
formspec suspend scripts --all --reason "..."   # stop semua handler Starlark, engine tetap layani read/CRUD
```

Setiap aksi darurat **wajib** menyertakan alasan, ditandatangani aktor, dan tercatat di transparency log.

---

## 12. Ops

### `formspec workspace create`

```bash
formspec workspace create --region jakarta --cluster-class premium
```

### `formspec logs`

Baca stream log terstruktur (JSON lines) dari engine Resource Plane — tail
dan filter tanpa menyaring JSON manual.

```bash
formspec logs --workspace corp-456 --follow          # tail live
formspec logs --module billing --entity invoice        # filter per module/entity
formspec logs --level error --since 1h                  # hanya error, jendela waktu
formspec logs --request-id req-abc123                    # satu request, lintas komponen
```

`formspec logs` **tidak pernah** menembus disiplin PII: nilai bisnis hanya
muncul kalau operator mengaktifkan level `debug`, yang off secara default di
`prod`. Kontrak lengkap (field wajib log, disiplin PII, filter):
[`docs/spec/platform/09-observability.md`](../spec/platform/09-observability.md)
§2, §7.

---

## 13. Status Implementasi Hari Ini

**`formspec apply` ada dan sudah jadi subcommand asli** dari binary `cmd/formspec` (bukan lagi binary terpisah `formspec-apply`). Verb lain di dispatcher `cmd/formspec/main.go` langsung mencetak `not implemented yet` dan exit 1 kalau dipanggil — bukan silent-fail.

| Verb                                | Status             | Catatan                                                                                                                                                                |
| ----------------------------------- | ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `apply`                             | ⚠️ Sebagian        | Subcommand nyata di `cmd/formspec`, tapi pipeline register→deploy putus di sisi server — lihat [`docs/runtimes/01-formspec-ctl.md`](../runtimes/01-formspec-ctl.md) §7 |
| `apply --watch`                     | ✅                 | `fsnotify`, debounce 500ms                                                                                                                                             |
| `validate`                          | ✅ Sebagian        | Engine loader + JSON Schema per kind; honesty scan Starlark masih roadmap (§2)                                                                                         |
| `new`, `dev`, `generate`, `migrate` | ⏳                 | Belum dikerjakan                                                                                                                                                       |
| Semua verb lain (§2–§12)            | ❌ Belum ada logic | Dikenali dispatcher, tapi cuma print "not implemented yet" — lihat `cmd/formspec/main.go`                                                                              |

### 13.1 Urutan Pembangunan yang Disarankan

1. **`formspec validate`** — ✅ sudah jalan (engine loader + JSON Schema per kind) di `cmd/formspec/validate.go`; masih ada sisa: honesty scan Starlark, `--fix`.
2. **`formspec new <kind>`** — scaffold sederhana, tidak bergantung komponen lain, cepat memberi nilai ke DX.
3. **`formspec dev`** — baru bermakna penuh setelah gap pipeline di [`docs/runtimes/01-formspec-ctl.md`](../runtimes/01-formspec-ctl.md) §7 (register→deployment) diperbaiki, karena `formspec dev` mengandalkan hot-reload lewat jalur yang sama.
4. **`formspec generate`** — bergantung stabilitas skema `pkg/spec` (sudah cukup stabil untuk kind `Document`), realistis dikerjakan setelah `validate`.
5. Sisanya (`backup`/`restore`/`archive`/`saga`/`module`/`sign`/emergency) bergantung fitur yang sendiri belum ada di `internal/*` (outbox lengkap, marketplace registry, dsb) — realistis fase lanjutan.

---

## 14. Referensi

| Dokumen                                                                            | Isi                                                                    |
| ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| [`docs/runtimes/01-formspec-ctl.md`](../runtimes/01-formspec-ctl.md)               | API server yang jadi target `apply`/`diff`/`get`                       |
| [`docs/architecture/03-deployment-flow.md`](../architecture/03-deployment-flow.md) | Bagaimana `formspec apply` masuk ke pipeline deployment production     |
| [`04-formspec-ctl.md`](04-formspec-ctl.md)                                         | CLI darurat Platform Operator (binary berbeda peran, sama proses)      |
| [`01-formspec-dev.md`](01-formspec-dev.md)                                         | Referensi lengkap `formspec dev`                                       |
| [`03-formspec-generate.md`](03-formspec-generate.md)                               | Referensi lengkap `formspec generate` + browser client SDK             |
| [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md)           | Kontrak: model permission, query/filter, API delivery                  |
| [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md)     | Kontrak: Mockup & environment binding                                  |
| [`docs/spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) | Kontrak: jaminan backup/restore                                        |
| [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md)   | Kontrak: Policy, transparency log, REPL governance, emergency controls |
