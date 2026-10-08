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
| **Data lifecycle**             | `migrate`, `seed`, `summary list\|rebuild`, `backup create\|inspect`, `restore`            |
| **Data archival**              | `archive run\|view\|restore-batch`                                                         |
| **Distributed workflow**       | `saga list\|resolve`                                                                       |
| **Marketplace & signing**      | `module list\|install\|uninstall\|publish`, `sign`, `override adopt\|diff\|list`, `verify` |
| **Scripting**                  | `script validate\|test`                                                                    |
| **Emergency (Resource Plane)** | `freeze`, `rollback`, `lock workspace`                                                     |
| **Ops**                        | `workspace create\|list\|delete`, `logs`, `spa install\|compress\|path\|remove`, `upgrade` |

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

Tiga lapis, semuanya dilaporkan per manifest:

1. **Engine loader** (`internal/manifest`) — ground truth apa yang `formspec dev` /
   `formspec apply` terima: error parse YAML dan validasi dalam Entity (expose,
   lifecycle, relation, state machine, `transaction_date`, reserved fields, …).
   Ini adalah hard gate.
2. **Cross-manifest (Layer 1.5)** — hal yang hanya bisa diperiksa dengan seluruh
   tree dalam pandangan, dan yang kalau lolos akan **diam-diam tidak melakukan
   apa-apa**: target `deliver` (7.7.3) dan `job:` `queue` harus menunjuk
   action/Service action yang ada, `Integration` wajib punya penangan cancel
   simetris (7.7.2), role-grant dan `row_scope` harus bisa di-resolve, dan
   `events:` sebuah `kind: Subscription` harus menunjuk event yang benar-benar
   dideklarasikan (kalau tidak, subscription itu **tidak pernah menyala**).
3. **JSON Schema** (`schemas/kinds/*.schema.json`) — kontrak untuk SEMUA kind
   (App, Module, Form, Table, …) yang belum di-deep-validate loader.
   Menangkap sintaks usang seperti `expose: all`, atau
   `state_machine.transitions[].guard` dalam bentuk skalar.

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

### `formspec spa install|compress|path|remove`

UI untuk binary yang dibangun tanpa embedded SPA (mis. hasil `go install` —
`go install` hanya menjalankan compiler Go, tidak bisa menjalankan npm).
Download artifact `spa-<versi>.tar.gz` dari GitHub Releases **versi yang sama
dengan binary** (`formspec version` — bukan `latest`, mencegah mismatch SPA
↔ CLI), verifikasi checksum terhadap `SHA256SUMS.txt` dari release yang sama,
lalu extract ke cache `~/.formspec/spa/<versi>/`.

```bash
formspec spa install             # download + verify + extract ke cache
formspec spa install --force     # download ulang meski sudah ada
formspec spa compress            # tulis sidecar .br (brotli q11) untuk dist/
formspec spa path                # path cache (untuk scripting / --web-dir)
formspec spa remove              # hapus cache versi ini
formspec spa remove --all        # hapus semua versi cache
```

- Eksplisit, bukan auto-download — `formspec dev` tidak pernah mengunduh apa
  pun di belakang layar; ia hanya MEMBACA cache yang sudah ada sebagai
  fallback sebelum embedded FS.
- Binary build `dev` menolak `spa install` (tidak ada tag rilis untuk
  di-match) — gunakan auto-detect repo atau `--dev-ui`.
- Base URL bisa di-override via env `FORMSPEC_SPA_URL`.

#### `formspec spa compress`

Menulis sidecar `<file>.br` (brotli) untuk aset yang layak dikompres di
`renderers/react-shadcn/dist`, dan menghapus sidecar yang sumbernya tidak lagi
layak. Dijalankan **setelah** build SPA — dipanggil otomatis oleh
`make build-spa` dan `make web-build`, jadi tidak perlu dijalankan manual
kecuali bekerja di luar Makefile.

```bash
formspec spa compress                          # default: q11, dir repo
formspec spa compress --dir /path/to/dist
formspec spa compress --quality 5              # lebih cepat build, byte lebih besar
```

Kenapa saat build, bukan saat request: encoder q11 ≈ 1,7 s/MiB. Dijalankan
sekali di sini, biayanya tidak pernah terlihat pengguna — dan klien tidak
membayar apa pun untuk kualitas tinggi itu (decode q11 ≈ decode q5, karena `q`
adalah knob _encode_). Terukur pada bundle kafe: 1.839.789 byte raw → 438.994
byte `.br`, versus 608.324 byte gzip (−27,8%). Gzip runtime tetap ada sebagai
fallback untuk klien tanpa brotli.

Predikat “layak dikompres” (tipe, ambang 1 KiB, dan syarat benar-benar
menyusut) dibagi dengan kode yang menyajikan aset, jadi langkah build ini tidak
bisa menyimpang dari apa yang benar-benar disajikan server.

### `formspec upgrade`

Self-update binary dari GitHub Releases — tanpa install ulang. Resolve versi
target (`releases/latest`, atau `--version <tag>`), download artifact
`formspec-<os>-<arch>.tar.gz|.zip` + `SHA256SUMS.txt` dari tag yang sama,
verifikasi checksum, smoke test (`<binary-baru> version`), lalu menimpa binary
di path `os.Executable()` secara atomik.

```bash
formspec upgrade                     # ke versi terbaru
formspec upgrade --check             # cek tanpa mengubah binary
formspec upgrade --dry-run           # tampilkan rencana (versi, URL, path)
formspec upgrade --version v0.0.7    # pin / rollback ke tag tertentu
formspec upgrade --force             # paksa walau versi sama (re-install)
formspec upgrade --yes               # non-interaktif (tanpa konfirmasi)
```

- **Eksplisit, bukan auto-update** — tidak ada cek versi di background.
- **Rilis resmi saja** — checksum wajib cocok; mismatch = abort tanpa
  menyentuh binary lama. Sumber bisa di-override via env
  `FORMSPEC_RELEASE_BASE` (mirror/proxy) dan `FORMSPEC_RELEASE_API`.
- **Fail-safe** — binary baru di-extract ke temp di direktori yang sama, di-smoke
  test dulu, baru di-swap. Gagal di langkah mana pun sebelum swap = tidak ada
  perubahan.
- **Tanpa `sudo`** — bila direktori binary tidak writable, perintah berhenti
  dengan pesan + fallback installer.
- Build `dev` (tanpa tag rilis) menolak `upgrade` — sama seperti `spa install`.
- Setelah upgrade, cache SPA `~/.formspec/spa/<versi-lama>/` tidak dipakai
  binary baru; jalankan `formspec spa install` bila memakai binary tanpa
  embedded SPA (mis. hasil `go install`).
- Untuk instalasi yang dikelola package manager (brew/scoop/apt), upgrade lewat
  package manager tersebut.

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

### Default project: `formspec-app.yaml`

**Semua command yang membaca spec tree atau database memakai default yang sama
dengan `formspec dev`**: spec, DSN, dan workspace diambil dari
`formspec-app.yaml` di direktori kerja, lalu flag CLI menimpanya. Urutannya:

1. **flag CLI** (menang atas semuanya);
2. **`formspec-app.yaml`** — mengisi nilai yang belum di-set flag
   (`spec:`, `dsn:`, `workspace-id:`);
3. **fallback** bila tidak ada config file: `spec`, `sqlite:.formspec/data.db`,
   workspace `default`;
4. **DSN SQLite relatif** di-anchor ke project root (diturunkan dari lokasi spec,
   [`../../docs_internal/plan/dsn-spec-anchored.md`]) — file db statis di
   `<project-root>/.formspec/…` di mana pun command dijalankan;
5. **workspace** mengikuti aturan #48: eksplisit → dipakai (dengan peringatan bila
   tidak dideklarasikan); tidak eksplisit & spec mendeklarasikan **tepat satu** →
   diadopsi; **beberapa** → tetap default + peringatan (tidak menebak tenant).

Yang berlaku: `dev`, `serve`, `repl`, `migrate`, `diff`, `seed`, `backup`,
`restore`, `archive`, `summary`, `logs`, `get`, `describe`, `delete`.

**Kenapa ini penting, bukan kosmetik.** Sebelum 2026-09-29 command-command itu
membawa literal sendiri (`spec`, `sqlite:.formspec/data.db`, `demo`) — yaitu nilai
yang `formspec dev` pakai justru ketika config file **tidak ada**. Di setiap
project yang punya `formspec-app.yaml` (semua hasil `formspec init`), menjalankan
command dari direktori project menyasar **database/tenant yang berbeda** dari yang
disajikan server. Terukur di `examples/kafe` (config: `spec: spec`,
`dsn: sqlite:.formspec/kafe.db`):

|           | `formspec dev`      | sebelum             | sesudah             |
| --------- | ------------------- | ------------------- | ------------------- |
| database  | `.formspec/kafe.db` | `.formspec/data.db` | `.formspec/kafe.db` |
| workspace | `kafe`              | `demo`              | `kafe`              |

Akibat nyatanya: alur repair (`repl -f repair.star` → `migrate apply`) mengerjakan
database yang tidak dibaca siapa pun, dan `formspec seed` tanpa flag membuat tenant
**ketiga** (`demo`) yang tak pernah dibaca App — sementara perintahnya melaporkan
sukses. Guard kelasnya: `TestNoCommandHardcodesProjectDefaults`
(`cmd/formspec/project_defaults_guard_test.go`) menolak literal-literal itu kembali
muncul di command mana pun.

### `formspec repl`

Console Starlark interaktif dengan akses `ctx.*` penuh — fitur first-class (bukan alat debug darurat sekali pakai), termasuk sebagai permukaan untuk AI Agent Skill debugging.

```bash
formspec repl                                     # interactive console
formspec repl -e 'ctx.config.get("currency")'     # one-shot expression
formspec repl --no-sync -f migrations/dedupe.star  # repair a refused migration
```

Default spec/DSN/workspace dari `formspec-app.yaml` (lihat [Default project](#default-project-formspec-appyaml));
artinya `formspec repl` di direktori project membuka database dan tenant yang
**sama** dengan `formspec dev`. Console mencetak
`[formspec] repl: spec=… dsn=… workspace=…` supaya hal itu terlihat, bukan
ditemukan belakangan lewat query.

Predeclared di console: `ctx.*` (wired ke datastore aplikasi), `resource`
(kosong — `resource.find`/`fetch`/`save` **belum ter-wire** di console; untuk
perbaikan data pakai `ctx.db().query(...)`), `ok`, `fail`.

| Flag            | Default                                              | Fungsi                                                   |
| --------------- | ---------------------------------------------------- | -------------------------------------------------------- |
| `--spec`        | `spec` _(atau `spec:` di config)_                    | Path ke direktori YAML manifests                         |
| `--dsn`         | `sqlite:.formspec/data.db` _(atau `dsn:` di config)_ | Database DSN                                             |
| `--workspace`   | workspace aktif _(config / #48 / `default`)_         | Tenant scope untuk `ctx.workspace` dan `ctx.db()`        |
| `--environment` | —                                                    | Diterima untuk forward-compat; policy-nya masih deferred |
| `--no-sync`     | `false`                                              | Buka datastore **tanpa** sync schema (lihat di bawah)    |
| `-e`            | —                                                    | One-shot ekspresi (scriptable)                           |
| `-f`            | —                                                    | One-shot script file                                     |

`-f` adalah permukaan resmi untuk **perbaikan data sekali jalan**: migrasi yang
ditolak karena datanya belum memenuhi syarat (mis. masih ada duplikat) tidak
menyediakan tempat untuk DML di spec, jadi repair dijalankan operator di sini,
lalu `formspec migrate apply` diulang — lihat
[`../spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §4.4.

**`--no-sync` — kenapa repair butuh flag ini.** Penolakan migrasi berlaku juga
pada boot console: `formspec repl` memanggil `formspec.New`, dan `New`
menyelaraskan schema — penolakan yang sama. Tanpa `--no-sync`, operator
diperintahkan menjalankan perintah yang gagal dengan pesan yang seharusnya ia
selesaikan. `--no-sync` membuka datastore **tanpa** menyentuh schema; ia
**tidak** melewati gerbang untuk boot normal (`dev`/`serve` tetap menolak), dan
hanya untuk repair. Contoh pesan penolakan:

```
apply migrations: 1 destructive change(s) refused
  - [lossy] index_added idx_cafe_order_table_sessions_dining_table_id: new unique index (12 row(s) affected)
    repair the duplicates first — run the repair once via
    `formspec repl --no-sync -f repair.star` (write through ctx.db()), then apply again
```

Alur lengkapnya:

```bash
formspec repl --spec spec --dsn sqlite:.formspec/data.db --no-sync -f repair.star
formspec migrate apply      # gerbang yang sama, sekarang lolos
```

**Bahasa script repair = subset Starlark yang sama dengan script action.**
Console memakai opsi parse yang identik (`syntax.LegacyFileOptions()`), jadi dua
batasan berikut berlaku dan mudah terlewat karena contoh umum di internet sering
tidak mematuhinya:

- **tanpa implicit string concatenation.** `"a" "b"` adalah **syntax error**
  (`got string literal, want ','`), bukan penggabungan — tulis satu literal
  panjang atau pakai `+`.
- **tanpa `for`/`if` di top-level.** Loop harus di dalam `def` yang lalu
  dipanggil (`for loop not within a function`).

`scripts/` pada spec tidak diperiksa secara statis: `formspec validate` dan
`formspec check` **tidak** mem-parse `.star`, sehingga script yang rusak sintaksis
lolos keduanya dan baru gagal saat action-nya dipanggil (terukur 2026-09-28).

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

Verb CLI untuk migrasi structural — migrasi sendiri **sepenuhnya otomatis dari diff Entity** (bukan hand-written), dan tidak ada manifest migrasi: DDL di luar bahasa spec dinyatakan sebagai `persist.raw_ddl` pada Entity-nya.

```bash
formspec migrate plan     # tampilkan perubahan berklasifikasi + jumlah baris, tanpa eksekusi
formspec migrate apply    # eksekusi (biasanya otomatis lewat formspec dev / apply)
```

Default spec/DSN dari `formspec-app.yaml` ([Default project](#default-project-formspec-appyaml)) — `migrate apply` adalah paruh kedua dari alur repair yang paruh pertamanya `repl --no-sync -f`, jadi keduanya harus menyasar database yang sama dengan yang disajikan server.

Setiap perubahan dinilai sebelum dieksekusi: **aditif** dan **derived** (kolom turunan dibangun ulang, index dihapus) berjalan otomatis; yang **lossy** — field dihapus, type change yang nilainya gagal cast, unique index sementara duplikat masih ada — ditolak sampai manifest menyatakannya (`removed: true` / `accept_data_loss: true` + `reason`). **Tabel tidak pernah di-drop dari manifest**: backup, drop manual, lalu hapus manifest-nya.

Perbaikan data (duplikat sebelum constraint, backfill) dijalankan operator **sekali** di luar spec lewat `formspec repl --no-sync -f repair.star` (flag wajib ada justru saat migrasinya belum bisa diterapkan) — bukan bagian dari migrasi. Alasan dan aturan lengkap: [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §4.

### `formspec seed`

Jalankan seeder & factory (`formspec/seed` official module) untuk data dev/testing.

```bash
formspec seed --module billing
formspec seed                     # workspace project dari config / #48
formspec seed --workspace staging # pilih tenant lain secara eksplisit
```

Default spec/DSN/**workspace** dari `formspec-app.yaml` ([Default project](#default-project-formspec-appyaml)). Ini bukan detail: baris seed mendarat di satu tenant, dan dulu defaultnya literal `"demo"` — sehingga `formspec seed` tanpa flag di project bernama (kafe) menulis seluruh data ke tenant yang tak pernah dibaca App sambil melaporkan sukses. `make seed-kafe` menghindarinya hanya karena eksplisit `--workspace kafe`.

**Record yang sudah ada di-reconcile, bukan sekadar dilewati.** Record yang
natural key-nya sudah ada tetapi field-nya berbeda dari seed akan di-**update**
(laporan menyebut field yang berubah), sisanya `skip`. Alasannya konkret: dengan
semantik skip murni, memperbaiki nilai di file seed tidak pernah sampai ke
database yang sudah ada — file terlihat benar sementara aplikasi tetap salah
(foto yang tadinya kosong, harga yang berubah). Dua pengecualian:

- **Field write-only (`masked: true`) tidak pernah di-reconcile.** Nilai
  tersimpannya bukan nilai yang ditulis (mis. `user.password` di-hash hook
  menjadi `password_hash`), dan reconcile menulis lewat jalur yang tidak
  menjalankan hook — menyalin nilai seed akan menyimpan kredensial dalam bentuk
  terbaca. Akun `user` karena itu selalu `skip`.
- **Record yang `doc_status`-nya bukan `draft`/kosong tidak disentuh.** Dokumen
  yang sudah di-submit adalah fakta bisnis, bukan data seed.

`$ref` di dalam record dapat menunjuk baris yang dibuat blok **lain** (urutan blok
di file tidak perlu mengikuti urutan dependensi).

#### `$asset` — aset seed diunggah lewat storage service

Nilai field `file` **adalah object key**, dan key kanoniknya memuat **id record**
(`{workspace}/{module}/{entity}/{id}/{field}/{uuid}-{nama}`) — yang belum ada
sebelum insert. Karena itu seed tidak menulis key sendiri; ia menyatakan **path
aset** dan membiarkan engine mengunggahnya:

```yaml
- entity: menu-item
  records:
    - code: MKN-002
      name: "Sate Ayam Madura"
      photo: { $asset: "menu/sate-ayam.jpg" } # relatif ke <module-dir>/assets/
```

Alurnya: record di-**insert** dulu (id tercipta) → file dibaca dari
`<module-dir>/assets/<path>` → **diunggah melalui storage service** (datastore
registry: filesystem di dev, garage/minio/s3 di prod) → **key kanonik** hasilnya
ditulis ke field. Bentuk key-nya sama persis dengan jalur unggah HTTP, sehingga
rute unduh dan `storage.visibility` bekerja tanpa cabang baru. `allowed_types`
dan `max_size_mb` field ditegakkan dengan matcher yang sama dengan handler
unggah — seed tidak bisa menulis objek yang API-nya tolak.

Karena itu **tidak ada langkah menyalin file terpisah**: `formspec seed` saja
sudah cukup, dan menghapus folder storage tidak permanen — seed berikutnya
meng-unggah ulang objek yang hilang (perbandingan byte, sehingga **mengganti**
file aset dengan versi yang benar pun ikut terkirim).

`$asset` hanya sah pada field `file`/`attachment`; di field lain ia ditolak saat
seed berjalan.

### `formspec backup create|inspect` / `formspec restore`

Jaminan format backup/restore (credible exit guarantee, kenapa operasi baca/ekspor tidak boleh license-gated): [`docs/spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) §3.

```bash
formspec backup create --full                       # atau --incremental, --filter <query>
formspec backup inspect backup-2026-07-10.tar

formspec restore --from backup-2026-07-10.tar \
  --map-resource beta/customer=staging/lead \     # arahkan record ke resource LAIN (3.7.7)
  --conflict remap \                              # skip | overwrite | remap (natural key baru)
  --dry-run                                       # laporan kompatibilitas dulu
```

**`--map-resource` dan `--conflict remap` dua hal yang berbeda**, dan namanya mudah tertukar:

| Flag                         | Artinya                                                                                                                                                                                                                                         |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--map-resource <src>=<dst>` | record dari resource `src` di arsip ditulis ke resource `dst` di spec tujuan (mis. memuat sample produksi ke entity dev). Boleh diulang. Target **wajib ada** di spec — kalau tidak, perintah berhenti dengan error sebelum menyentuh database. |
| `--conflict remap`           | resource-nya tetap sama; yang diubah **natural key** saat bentrok (`C-001` → `C-001-r1`) supaya record lama tidak tertimpa.                                                                                                                     |

Kedua sisi `--map-resource` menerima `module/entity` atau `module_entity` (spelling yang dicetak `backup inspect`). Pemetaan dilaporkan di awal output, dan laporan dry-run mencetak `sumber -> target` per entity sehingga hasil yang kosong tidak salah dibaca sebagai "pemetaan tidak melakukan apa-apa".

File storage ikut ter-backup **lewat storage service** — objek dibaca/ditulis melalui `ResolveStorage` (datastore registry), bukan dari path `{state}/storage` yang hardcoded, sehingga `kind: Datastore` ber-driver garage/minio/s3 juga tercakup. Kunci objek di-enumerasi dari field `file`/`attachment` pada record yang ikut ter-backup (kontrak `Storage` tidak punya operasi list), dan di-upload kembali **verbatim** saat restore — kunci di arsip identik dengan yang dirujuk record, jadi tidak ada remap yang bisa memutusnya. Summary/agregat tidak ikut (bisa dihitung ulang). Transform per-record via script Starlark saat restore. `restore` yang meng-overwrite data yang sudah ada wajib tanda tangan pemilik workspace atau delegasi eksplisit ber-scope `backup.restore`, selalu tercatat di transparency log.

> **Batasan yang diketahui:** backup/restore memakai workspace aktif hasil aturan #48 (`kind: Workspace` **mendaftarkan** slug, tidak memilih satu): dengan satu workspace dideklarasikan, slug itulah yang dipakai; dengan beberapa, perintah **memperingatkan** dan memakai default `demo` sampai `--workspace <slug>` diberikan. Sebelumnya perintah selalu membaca `demo` secara hardcoded, sehingga `formspec backup create` pada aplikasi ber-tenant lain (kafe → `kafe`) melaporkan **0 record** padahal tabelnya berisi — dan manifest.json kini mencatat `workspace` supaya arsip kosong bisa dibedakan dari arsip aplikasi yang memang kosong. Ditutup di todo **4.8.7**.

### `formspec summary list|rebuild`

Summary Entity (`characteristic: summary`) tidak ikut ter-backup justru karena ia proyeksi: seluruh isinya bisa dihitung ulang dari data sumber (spec §6). Verb ini membuat janji itu bisa dijalankan — bukan sekadar teori di dokumen.

```bash
formspec summary list                        # inventaris: mana yang bisa di-rebuild, mana yang tidak
formspec summary rebuild cafe-stock/stock-level --dry-run
formspec summary rebuild stock-level --reset # kosongkan proyeksi dulu, lalu replay
```

Mekanismenya: `rebuild` me-replay stream event durabel dari **awal** untuk setiap Subscription durabel yang mendengarkan event sumber, lewat jalur yang **sama persis** dengan delivery live (filter → transform → handler) — sehingga hasil rebuild identik dengan hasil operasi normal.

Tiga sifat yang perlu diketahui sebelum menjalankannya:

| Sifat                                  | Konsekuensi                                                                                                                                                                                 |
| -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Replay jalan di consumer group sendiri | Cursor dan pending entry worker live **tidak tersentuh** — rebuild aman dijalankan terhadap server yang sedang melayani.                                                                    |
| Handler durabel wajib idempoten        | Replay mengulang **semua** subscriber durabel dari event sumber, bukan hanya yang mengisi proyeksi itu. Aman menurut kontrak at-least-once, tapi `--subscriber` mempersempitnya bila perlu. |
| Butuh stream backend bersama           | Redis/Valkey. Backend dev default (in-memory) hidup di dalam proses server, jadi proses CLI terpisah tidak punya riwayat — perintah ini mengatakan itu, bukan melaporkan sukses kosong.     |

Kegagalan handler **dilaporkan, tidak di-retry** oleh rebuild (retry tetap tugas worker live; run yang gagal di-ack di group-nya sendiri supaya tidak menggantung). Perintah keluar dengan status ≠ 0 dan minta dijalankan ulang setelah penyebabnya diperbaiki. Sumber yang dideklarasikan tapi tidak punya subscriber durabel dilaporkan sebagai **orphaned** — gap itu ditampilkan, bukan disembunyikan.

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

### `formspec workspace`

Kelola registry workspace bernama (slug = workspace ID; slug tak terdaftar
ditolak 404 oleh `WorkspaceMiddleware`):

```bash
formspec workspace create kopi --name "Kopi Kita" --dsn sqlite:.formspec/cafe.db
formspec workspace list --dsn sqlite:.formspec/cafe.db
formspec workspace delete kopi --dsn sqlite:.formspec/cafe.db --confirm
```

Slug wajib kebab-case dan tidak boleh memakai segmen reserved router.
Workspace `default` tidak bisa dihapus. Contoh deklaratif: `kind: Workspace`
manifest (lihat [`../spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §1.1).

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
| `workspace create\|list\|delete`    | ✅                 | Registry workspace bernama (entity `formspec.core/workspace`); slug = workspace ID, tak terdaftar → 404 (§12)                                                          |
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
