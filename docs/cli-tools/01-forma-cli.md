# forma — CLI Reference

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0 (dokumen) — binary-nya sendiri FSL (open source)
**Governed by:** `docs/spec/02-core-basic.md` §23–25/27, `docs/spec/01-overview.md` §11–12, `docs/spec/04-control-plane.md` §11, `docs/spec/11-reference.md` (D13, D34)

> `forma` adalah CLI utama untuk App/Module Owner dan Developer — satu binary (`cmd/forma`), banyak subcommand. Semua verb di dokumen ini adalah subcommand dari **satu proses `forma`**, bukan binary terpisah. Untuk CLI darurat Platform Operator, lihat `02-forma-ctl.md`.

---

## 1. Ringkasan Verb

| Kategori | Verb |
|---|---|
| **Deployment** | `apply`, `diff`, `delete`, `get`, `describe`, `validate` |
| **Scaffolding** | `new <kind>` |
| **Dev loop** | `dev`, `repl` |
| **Codegen** | `generate` |
| **Data lifecycle** | `migrate`, `seed`, `backup create\|inspect`, `restore` |
| **Data archival** | `archive run\|view\|restore-batch` |
| **Distributed workflow** | `saga list\|resolve` |
| **Marketplace & signing** | `module list\|install\|uninstall`, `sign` |
| **Scripting** | `script validate\|test` |
| **Emergency (Resource Plane)** | `freeze`, `rollback`, `lock workspace` |
| **Ops** | `workspace create` |

---

## 2. Deployment

### `forma apply`

Satu-satunya cara mendaftarkan YAML manifest ke Control Plane (lihat `docs/architecture/03-deployment-flow.md`, `docs/runtimes/01-forma-ctl.md` §5).

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

Validasi tanpa mendaftarkan — dry-run. Termasuk **honesty scan** untuk script: `uses` yang dideklarasikan vs yang benar-benar dipakai script (D20).

```bash
forma validate -f myapp/
# undeclared usage  → error
# declared-but-unused → warning
# ctx.environment branching di business script → warning (§spec/03-core-extended.md:103)
```

`forma validate` **tidak pernah memberi grant** — ia cuma verifikasi kejujuran deklarasi terhadap kode. Dijalankan tiap PR sebagai gate CI.

---

## 3. Scaffolding

### `forma new <kind>`

Scaffold boilerplate untuk kind tertentu — bagian dari "spec tooling ladder" (D34): JSON Schema+LSP → `forma new` → visual editor di admin panel → Agent Skill.

```bash
forma new app tokoku                # scaffold App baru
forma new document invoice           # scaffold Document + field dasar
```

---

## 4. Dev Loop

### `forma dev`

Satu perintah, seluruh environment dev lewat Docker Compose: Postgres 16, Valkey/Redis, Mailpit, MinIO, `forma-ctl:dev` (relaxed policy).

```bash
forma dev
```

Startup sequence: health check semua service → migrate → seed → hot reload aktif → regenerate types saat YAML berubah → buka dashboard.

Untuk standalone mode tanpa Docker Compose (single Go binary + `forma-ctl --mode=standalone`), lihat `docs/architecture/01-architecture-overview.md` §10 — `forma dev` di skenario itu cukup menjalankan dua proses lokal.

### `forma repl`

Console Starlark interaktif dengan akses `ctx.*` penuh — fitur first-class (D13), termasuk sebagai permukaan untuk AI Agent Skills debugging.

```bash
forma repl --environment staging
>>> invoice.load("inv-001")
>>> ctx.db.query("...")
```

**Scope environment policy** (`docs/spec/04-control-plane.md` §11):

| Environment | Akses REPL |
|---|---|
| Dev/staging profile | Read-write, sesi direkam di audit log |
| Production profile | **Read-only** — write butuh policy approval eksplisit time-boxed, tercatat di transparency log |

REPL selalu berjalan di bawah identitas user nyata dengan permission user tersebut — **bukan** superuser shell.

---

## 5. Codegen

### `forma generate`

Menurunkan typed client/server types (Go; TypeScript untuk frontend), konstanta permission/enum, dan dokumen OpenAPI — dari manifest sebagai satu-satunya source of truth. **Kode hasil generate tidak pernah diedit manual.**

```bash
forma generate --spec ./spec --out ./src/generated/forma-client.ts   # implemented
forma generate --lang go,typescript                                  # go: not implemented yet
forma generate --openapi > api-spec.json                             # not implemented yet
```

**Status:** `--lang typescript` implemented (`cmd/forma/generate.go`) — lihat `docs/cli-tools/03-forma-generate.md` untuk referensi lengkap dan panduan pemakaian di frontend (termasuk `@forma/client`, runtime SDK-nya). `--lang go` dan `--openapi` belum dibangun.

---

## 6. Data Lifecycle

### `forma migrate`

Verb CLI untuk migrasi structural — migrasi sendiri **fully automatic dari Document diff** (bukan hand-written); `forma migrate` adalah cara memicu/inspeksi proses itu, bukan tempat menulis migrasi (migrasi custom pakai `kind: Migration`, DDL-only, DML ditolak runtime).

```bash
forma migrate plan     # tampilkan DDL yang akan dijalankan, tanpa eksekusi
forma migrate apply    # eksekusi (biasanya otomatis lewat forma apply)
```

### `forma seed`

Jalankan seeder & factory (`forma/seed` official module) untuk data dev/testing.

```bash
forma seed --module billing
```

### `forma backup create|inspect` / `forma restore`

Normatif untuk **Credible Exit Guarantee** (D31) — format backup adalah bagian dari spec terbuka, **tidak boleh license-gated** (D27).

```bash
forma backup create --full                       # atau --incremental, --filter <query>
forma backup inspect backup-2026-07-10.tar

forma restore --from backup-2026-07-10.tar \
  --map-resource old-customer=new-customer \      # remap saat konflik ID
  --conflict remap \                              # skip | overwrite | remap (UUID+FK di-remap)
  --dry-run                                       # laporan kompatibilitas dulu
```

File storage ikut ter-backup; summary/agregat tidak (bisa dihitung ulang). Transform per-record via script Starlark saat restore.

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

Perintah darurat yang dijalankan **App Admin yang diotorisasi**, di sisi Resource Plane (bukan Platform Operator — itu `forma-ctl`, lihat `02-forma-ctl.md`):

```bash
forma freeze --reason "..."
forma rollback --since 1h --all
forma lock workspace <name> --reason "..."
```

Setiap aksi darurat **wajib** menyertakan alasan, ditandatangani aktor, dan tercatat di transparency log.

---

## 12. Ops

### `forma workspace create`

```bash
forma workspace create --region jakarta --cluster-class premium
```

---

## 13. Status Implementasi Hari Ini

**`forma apply` ada dan sudah jadi subcommand asli** dari binary `cmd/forma` (bukan lagi binary terpisah `forma-apply`). Verb lain di dispatcher `cmd/forma/main.go` langsung mencetak `not implemented yet` dan exit 1 kalau dipanggil — bukan silent-fail.

| Verb | Status | Catatan |
|---|---|---|
| `apply` | ⚠️ Sebagian | Subcommand nyata di `cmd/forma`, tapi pipeline register→deploy putus di sisi server — lihat `docs/runtimes/01-forma-ctl.md` §7 |
| `apply --watch` | ✅ | `fsnotify`, debounce 500ms |
| Semua verb lain (§2–§12) | ❌ Belum ada logic | Dikenali dispatcher, tapi cuma print "not implemented yet" — lihat `cmd/forma/main.go` |

`docs/plan/todo.md` Fase 6 (CLI & DX) menandai `validate`, `new`, `dev`, `generate`, `migrate` sebagai ⏳ (belum dikerjakan) — konsisten dengan temuan ini.

### 13.1 Urutan Pembangunan yang Disarankan

1. **`forma validate`** berikutnya — nilainya tinggi (CI gate) dan tidak butuh pipeline deploy penuh untuk berfungsi, cukup manifest loader + honesty scanner (`internal/manifest`, `internal/permission` sudah ada). Ganti `case "validate":` di dispatcher dengan implementasi nyata.
2. **`forma new <kind>`** — scaffold sederhana, tidak bergantung komponen lain, cepat memberi nilai ke DX.
3. **`forma dev`** — baru bermakna penuh setelah gap pipeline di `docs/runtimes/01-forma-ctl.md` §7 (register→deployment) diperbaiki, karena `forma dev` mengandalkan hot-reload lewat jalur yang sama.
4. **`forma generate`** — bergantung stabilitas skema `pkg/spec` (sudah cukup stabil untuk kind `Document`), realistis dikerjakan setelah `validate`.
5. Sisanya (`backup`/`restore`/`archive`/`saga`/`module`/`sign`/emergency) bergantung fitur yang sendiri belum ada di `internal/*` (outbox lengkap, marketplace registry, dsb) — realistis fase lanjutan.

---

## 14. References

| Dokumen | Isi |
|---|---|
| `docs/spec/02-core-basic.md` §23–25, 27 | Verb normatif, dev environment, backup/restore, codegen |
| `docs/spec/01-overview.md` §11–12 | Tools per persona, ekosistem CLI |
| `docs/spec/11-reference.md` D13, D34 | REPL first-class, spec tooling ladder |
| `docs/runtimes/01-forma-ctl.md` | API server yang jadi target `apply`/`diff`/`get` |
| `docs/cli-tools/02-forma-ctl.md` | CLI darurat Platform Operator (binary berbeda peran, sama proses) |
