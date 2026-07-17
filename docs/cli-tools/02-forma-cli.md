# forma — CLI Reference

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0 (dokumen) — binary-nya sendiri FSL (open source)

> `forma` adalah CLI utama untuk App/Module Owner dan Developer — satu binary (`cmd/forma`), banyak subcommand. Semua verb di dokumen ini adalah subcommand dari **satu proses `forma`**, bukan binary terpisah. Untuk CLI darurat Platform Operator, lihat [`04-forma-ctl.md`](04-forma-ctl.md). Dokumen ini normatif — perilaku setiap verb didefinisikan di sini secara lengkap; implementasi kode mengikuti dokumen ini, bukan sebaliknya.

---

## 1. Ringkasan Verb

| Kategori | Verb |
|---|---|
| **Deployment** | `apply`, `diff`, `delete`, `get`, `describe`, `validate`, `check [--fix]`, `promote` |
| **Scaffolding** | `new <kind>` |
| **Dev loop** | `dev`, `repl` |
| **Codegen** | `generate` |
| **Data lifecycle** | `migrate`, `seed`, `backup create\|inspect`, `restore` |
| **Data archival** | `archive run\|view\|restore-batch` |
| **Distributed workflow** | `saga list\|resolve` |
| **Marketplace & signing** | `module list\|install\|uninstall`, `sign` |
| **Scripting** | `script validate\|test` |
| **Emergency (Resource Plane)** | `freeze`, `rollback`, `lock workspace` |
| **Ops** | `workspace create`, `logs` |

---

## 2. Deployment

### `forma apply`

Satu-satunya cara mendaftarkan YAML manifest ke Control Plane (lihat [`docs/architecture/03-deployment-flow.md`](../architecture/03-deployment-flow.md), [`docs/runtimes/01-forma-ctl.md`](../runtimes/01-forma-ctl.md) §5).

```bash
forma apply -f myapp/
forma apply -f myapp/ --watch          # hot-reload, debounce 500ms
```

Di belakang layar: walk direktori spec (skip `impl/`, hidden dir, `node_modules`) → kirim `.yaml`/`.yml`/`.star` sebagai payload ke `POST /v1/artifacts` → response `{artifact_id, version, sha256}`.

### `forma diff`

Bandingkan spec lokal dengan state yang sudah ter-deploy di Control Plane, tanpa mengubah apapun.

```bash
forma diff -f myapp/
# Menunjukkan: field yang ditambah/dihapus, action baru, permission yang berubah
```

### `forma get` / `forma describe`

Ambil resource yang sudah terdaftar — pola mirip `kubectl get`/`kubectl describe`.

```bash
forma get document invoice                # ringkas: nama, versi, status
forma describe document invoice           # detail: field, action, state machine, permission
```

### `forma delete`

Hapus resource dari deployment (menandai artifact versi berikutnya tanpa kind tersebut).

```bash
forma delete document old-report --confirm
```

### `forma validate`

Validasi tanpa mendaftarkan — dry-run. Termasuk **honesty scan** untuk script Starlark.

```bash
forma validate -f myapp/
# undeclared usage  → error
# declared-but-unused → warning
# ctx.environment branching di business script → warning
```

`forma validate` **tidak pernah memberi grant** — ia cuma verifikator kejujuran otomatis atas deklarasi `required_permission`/`uses` setiap action terhadap kode sungguhan, bukan sumber kebenaran permission itu sendiri. Model permission lengkap (kelima jenis impl, kenapa grant tidak pernah diturunkan dari pemakaian): [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §5. Aturan environment binding pada business logic (kenapa `ctx.environment` hanya untuk logging, bukan percabangan bisnis): [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §8. Dijalankan tiap PR sebagai gate CI.

### `forma check [--fix]`

Analisis statis menyeluruh atas satu project — melampaui `validate` (yang
per-manifest): resolusi lintas-file dan lintas-module dalam satu workspace.
Wajib melaporkan minimal:

```bash
forma check -f myapp/
# unresolved varname di script (referensi field/identifier yang tidak ada) → error
# FormaExpr mereferensi field yang tidak ada di skema                       → error
# akses lintas-module yang dipakai tapi belum dideklarasikan/di-approve      → error
# deklarasi lintas-module yang tidak pernah dipakai                          → warning
```

`forma check --fix` memperbaiki apa yang bisa diperbaiki otomatis: menambah
deklarasi `depends_on`/`uses` yang kurang (setelah konfirmasi interaktif —
penambahan deklarasi adalah perluasan footprint consent, tidak pernah
diam-diam), dan menghapus deklarasi yang tidak terpakai. Error kelas
unresolved-reference **menggagalkan `forma apply`** — inilah yang menjamin
error referensi FormaExpr/script tidak mungkin muncul di runtime
([`../spec/frontend/08-formaexpr.md`](../spec/frontend/08-formaexpr.md),
[`../spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §7).

### `forma promote`

Promosikan artifact yang **sama** (checksum diverifikasi identik) dari satu
environment ke environment lain — tanpa build/sign ulang. Mengikuti siklus
Sign → Apply → Approve → Promote; approval memakai Policy environment tujuan.

```bash
forma promote myapp --from staging --to production
```

Kontrak lengkap (verifikasi checksum, gate re-consent, chain transparency
log): [`docs/spec/platform/10-deployment-operations.md`](../spec/platform/10-deployment-operations.md)
§5 dan [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md)
§2–3.

---

## 3. Scaffolding

### `forma new <kind>`

Scaffold boilerplate untuk kind tertentu — anak tangga kedua dari empat anak tangga yang mengurangi verbositas YAML:

1. JSON Schema per kind (dipublikasikan di `forma.dev/schemas`) + LSP — autocomplete, hover docs, validasi realtime; nyaris gratis berkat format seragam `apiVersion/kind`.
2. **`forma new <kind>`** — scaffold CLI (dokumen ini).
3. Editor visual di admin panel (mirip DocType editor Frappe) — **menulis YAML ke file/PR, bukan ke database tersembunyi**; git tetap jadi source of truth.
4. Agent Skill — spec editor untuk AI.

```bash
forma new app tokoku                # scaffold App baru
forma new document invoice           # scaffold Document + field dasar
```

---

## 4. Dev Loop

### `forma dev`

Development server — satu perintah untuk menjalankan backend API + SPA frontend.
SPA sudah embedded dalam binary (`//go:embed web/dist/*`), tidak perlu npm.

```bash
# Single process — API + SPA di :8080
forma dev --spec ./my-app/spec

# Dengan Vite HMR (edit frontend)
forma dev --spec ./my-app/spec --dev-ui

# Auto-detect config file (forma-app.yaml)
forma dev
```

**Behavior:**
- Membaca YAML manifests dari `--spec` (default: `./spec`)
- Generate tabel database sesuai entity spec
- Serve REST API di `--addr` (default: `:8080`)
- Serve SPA (embedded atau dari `--web-dir`)
- Auto-detect runtime dari project files (`composer.json` → PHP, dll.)
- `--dev-ui`: spawn Vite HMR (cari `web/` dari CWD atau module cache)
- `--force` implied oleh `--dev` / `--dev-ui`

**Flag referensi:**

| Flag | Default | Fungsi |
|---|---|---|
| `--spec` | `./spec` | Path ke direktori YAML manifests |
| `--dsn` | `sqlite:.forma/data.db` | Database DSN |
| `--addr` | `:8080` | REST API listen address |
| `--listen` | `none` | Ctx listener mode: `none`, `local_http`, `unix_socket` |
| `--app-endpoint` | `none` | App endpoint mode: `none`, `local_http`, `unix_socket` |
| `--runtime` | auto-detect | Runtime auto-detect (php/python/node) |
| `--dev-ui` | `false` | Start Vite HMR (implies `--dev`) |
| `--dev` | `false` | Dev mode (auth bypass) |
| `--force` | `false` | Kill previous instance. Implied oleh `--dev` / `--dev-ui` |
| `--web-dir` | auto-detect | Override SPA directory |
| `--state-dir` | `.forma` | State directory (auto-create) |

**Runtime auto-detect:**

| File di CWD | Runtime |
|---|---|
| `composer.json` | php |
| `package.json` | node |
| `pyproject.toml` / `requirements.txt` | python |
| `go.mod` | local (Go) |
| (none) | local (API-only) |

Referensi lengkap flag, mode `--listen`/`--app-endpoint`, dan arsitektur proses: [`01-forma-dev.md`](01-forma-dev.md).

### `forma repl`

Console Starlark interaktif dengan akses `ctx.*` penuh — fitur first-class (bukan alat debug darurat sekali pakai), termasuk sebagai permukaan untuk AI Agent Skill debugging.

```bash
forma repl --environment staging
>>> invoice.load("inv-001")
>>> ctx.db.query("...")
```

Scope environment policy (tabel akses per profil environment, jaminan "bukan superuser shell"): [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) §7.

---

## 5. Codegen

### `forma generate`

Menurunkan typed client/server types (Go; TypeScript untuk frontend), konstanta permission/enum, dan dokumen OpenAPI — dari manifest sebagai satu-satunya source of truth. **Kode hasil generate tidak pernah diedit manual.**

```bash
forma generate --spec ./spec --out ./src/generated/forma-client.ts   # implemented
forma generate --lang go,typescript                                  # go: not implemented yet
forma generate --openapi > api-spec.json                             # not implemented yet
```

**Status:** `--lang typescript` implemented (`cmd/forma/generate.go`) — lihat [`03-forma-generate.md`](03-forma-generate.md) untuk referensi lengkap dan panduan pemakaian di frontend (termasuk `@forma/client`, runtime SDK-nya). `--lang go` dan `--openapi` belum dibangun.

---

## 6. Data Lifecycle

### `forma migrate`

Verb CLI untuk migrasi structural — migrasi sendiri **fully automatic dari Document diff** (bukan hand-written); `forma migrate` adalah cara memicu/inspeksi proses itu, bukan tempat menulis migrasi (migrasi custom pakai `kind: Migration`, DDL-only, DML ditolak runtime).

```bash
forma migrate plan     # tampilkan DDL yang akan dijalankan, tanpa eksekusi
forma migrate apply    # eksekusi (biasanya otomatis lewat forma apply)
```

Rename field wajib dideklarasikan lewat `renamed_from` pada field — kalau tidak, diff menafsirkannya sebagai drop+add; penghapusan field butuh dua langkah (deprecate, lalu remove) lintas dua versi apply. Backfill data adalah urusan migrasi tipe data (scripted, run/rollback per versi), bukan migrasi structural.

### `forma seed`

Jalankan seeder & factory (`forma/seed` official module) untuk data dev/testing.

```bash
forma seed --module billing
```

### `forma backup create|inspect` / `forma restore`

Jaminan format backup/restore (credible exit guarantee, kenapa operasi baca/ekspor tidak boleh license-gated): [`docs/spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) §3.

```bash
forma backup create --full                       # atau --incremental, --filter <query>
forma backup inspect backup-2026-07-10.tar

forma restore --from backup-2026-07-10.tar \
  --map-resource old-customer=new-customer \      # remap saat konflik ID
  --conflict remap \                              # skip | overwrite | remap (UUID+FK di-remap)
  --dry-run                                       # laporan kompatibilitas dulu
```

File storage ikut ter-backup; summary/agregat tidak (bisa dihitung ulang). Transform per-record via script Starlark saat restore. `restore` yang meng-overwrite data yang sudah ada wajib tanda tangan pemilik workspace atau delegasi eksplisit ber-scope `backup.restore`, selalu tercatat di transparency log.

---

## 7. Data Archival

### `forma archive run|view|restore-batch`

Hanya **transaction** (`characteristic: transaction`) yang diarsipkan penuh; **master** yang direferensikan cuma di-snapshot "as-of" tanggal arsip (baris master di production tetap utuh, ditandai `locked_for_deletion: true` selama masih direferensikan arsip).

```bash
forma archive run --max-age 3y --dry-run    # tampilkan rencana, minta konfirmasi operator
forma archive run --max-age 3y              # eksekusi: tulis Parquet, set locked_for_deletion,
                                             # hapus baris transaction dari production
forma archive view --batch-id archive-2021-2023   # query langsung Parquet, tanpa live DB
forma archive restore-batch --batch-id archive-2021-2023 --target staging
```

Format penyimpanan:

```
archive-2021-2023.parquet/
  manifest.yaml           # archive_date, max_age, record_count
  transactions/           # invoices.parquet, journal_entries.parquet, ...
  masters/                # snapshot as-of archive_date: customers.parquet, ...
```

Restore **hanya ke staging**, restore dependency-ordered, **selective per-document restore tidak didukung** (risiko corrupt state). Konfigurasi retensi lewat `retention:` di `forma.yaml` (`archive_after`, `strategy: cold_storage|delete`, `destination`).

---

## 8. Distributed Workflow (Saga/Compensation)

### `forma saga list|resolve <id>`

Antrian intervensi manual untuk `compensation-failure-log` (resource `forma.core`, `persist.category: compliance`):

| Sub-status | Arti | Tindakan benar |
|---|---|---|
| `compensation_failed` | Step gagal, undo dicoba, undo juga gagal | Manusia perbaiki manual — state sudah diketahui |
| `outcome_unknown` | Tidak diketahui apakah step berhasil, retry habis | Manusia **verifikasi state aktual dulu** — tombol retry/compensate otomatis TIDAK boleh ditampilkan |

```bash
forma saga list --status outcome_unknown
forma saga resolve saga-abc123 --action confirm-succeeded  # atau --action compensate-now
```

Tidak ada retry otomatis tanpa batas — kalau butuh manusia, sistem tidak berpura-pura bisa menyelesaikan sendiri.

---

## 9. Marketplace & Signing

### `forma module list|install|uninstall`

```bash
forma module install billing-pro --from registry.forma.dev
# Menampilkan ModuleFootprint (aggregate required_permission + uses) untuk consent SEBELUM install
```

### `forma sign`

Tanda tangan artifact dengan owner key (App/Module Owner).

```bash
forma sign -f order.yaml --key ~/.forma/keys/billing-team.key --environment staging
```

Integrator (cross-boundary call) yang idempotency-nya tidak `true` **ditolak** `forma apply` — action target harus `idempotent: true` untuk dipakai lintas boundary.

---

## 10. Scripting

### `forma script validate|test`

```bash
forma script validate invoice.star     # sandbox check: 5000ms/64MB/100k iterasi, no network/fs/subprocess
forma script test invoice.star --fixture fixtures/invoice_submit.json
```

---

## 11. Emergency (Resource Plane Side)

Perintah darurat yang dijalankan **App Admin yang diotorisasi**, di sisi Resource Plane (bukan Platform Operator — itu `forma-ctl`, lihat [`04-forma-ctl.md`](04-forma-ctl.md)):

```bash
forma freeze --reason "..."
forma rollback --since 1h --all
forma lock workspace <name> --reason "..."
forma suspend scripts --all --reason "..."   # stop semua handler Starlark, engine tetap layani read/CRUD
```

Setiap aksi darurat **wajib** menyertakan alasan, ditandatangani aktor, dan tercatat di transparency log.

---

## 12. Ops

### `forma workspace create`

```bash
forma workspace create --region jakarta --cluster-class premium
```

### `forma logs`

Baca stream log terstruktur (JSON lines) dari engine Resource Plane — tail
dan filter tanpa menyaring JSON manual.

```bash
forma logs --workspace corp-456 --follow          # tail live
forma logs --module billing --entity invoice        # filter per module/entity
forma logs --level error --since 1h                  # hanya error, jendela waktu
forma logs --request-id req-abc123                    # satu request, lintas komponen
```

`forma logs` **tidak pernah** menembus disiplin PII: nilai bisnis hanya
muncul kalau operator mengaktifkan level `debug`, yang off secara default di
`prod`. Kontrak lengkap (field wajib log, disiplin PII, filter):
[`docs/spec/platform/09-observability.md`](../spec/platform/09-observability.md)
§2, §7.

---

## 13. Status Implementasi Hari Ini

**`forma apply` ada dan sudah jadi subcommand asli** dari binary `cmd/forma` (bukan lagi binary terpisah `forma-apply`). Verb lain di dispatcher `cmd/forma/main.go` langsung mencetak `not implemented yet` dan exit 1 kalau dipanggil — bukan silent-fail.

| Verb | Status | Catatan |
|---|---|---|
| `apply` | ⚠️ Sebagian | Subcommand nyata di `cmd/forma`, tapi pipeline register→deploy putus di sisi server — lihat [`docs/runtimes/01-forma-ctl.md`](../runtimes/01-forma-ctl.md) §7 |
| `apply --watch` | ✅ | `fsnotify`, debounce 500ms |
| `validate`, `new`, `dev`, `generate`, `migrate` | ⏳ | Belum dikerjakan |
| Semua verb lain (§2–§12) | ❌ Belum ada logic | Dikenali dispatcher, tapi cuma print "not implemented yet" — lihat `cmd/forma/main.go` |

### 13.1 Urutan Pembangunan yang Disarankan

1. **`forma validate`** berikutnya — nilainya tinggi (CI gate) dan tidak butuh pipeline deploy penuh untuk berfungsi, cukup manifest loader + honesty scanner (`internal/manifest`, `internal/permission` sudah ada). Ganti `case "validate":` di dispatcher dengan implementasi nyata.
2. **`forma new <kind>`** — scaffold sederhana, tidak bergantung komponen lain, cepat memberi nilai ke DX.
3. **`forma dev`** — baru bermakna penuh setelah gap pipeline di [`docs/runtimes/01-forma-ctl.md`](../runtimes/01-forma-ctl.md) §7 (register→deployment) diperbaiki, karena `forma dev` mengandalkan hot-reload lewat jalur yang sama.
4. **`forma generate`** — bergantung stabilitas skema `pkg/spec` (sudah cukup stabil untuk kind `Document`), realistis dikerjakan setelah `validate`.
5. Sisanya (`backup`/`restore`/`archive`/`saga`/`module`/`sign`/emergency) bergantung fitur yang sendiri belum ada di `internal/*` (outbox lengkap, marketplace registry, dsb) — realistis fase lanjutan.

---

## 14. Referensi

| Dokumen | Isi |
|---|---|
| [`docs/runtimes/01-forma-ctl.md`](../runtimes/01-forma-ctl.md) | API server yang jadi target `apply`/`diff`/`get` |
| [`docs/architecture/03-deployment-flow.md`](../architecture/03-deployment-flow.md) | Bagaimana `forma apply` masuk ke pipeline deployment production |
| [`04-forma-ctl.md`](04-forma-ctl.md) | CLI darurat Platform Operator (binary berbeda peran, sama proses) |
| [`01-forma-dev.md`](01-forma-dev.md) | Referensi lengkap `forma dev` |
| [`03-forma-generate.md`](03-forma-generate.md) | Referensi lengkap `forma generate` + browser client SDK |
| [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) | Kontrak: model permission, query/filter, API delivery |
| [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) | Kontrak: Mockup & environment binding |
| [`docs/spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) | Kontrak: jaminan backup/restore |
| [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) | Kontrak: Policy, transparency log, REPL governance, emergency controls |
