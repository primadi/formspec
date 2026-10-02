# Command CLI membaca `formspec-app.yaml` seperti `formspec dev`

**Tanggal**: 2026-09-29
**Status**: implementasi
**Kontrak terkait**: `docs/cli-tools/02-formspec-cli.md`, `docs/spec/platform/08-project-layout.md`, `docs_internal/plan/dsn-spec-anchored.md`
**Effort**: medium

---

## 1. Masalah (terukur)

`formspec dev` membaca `formspec-app.yaml` (spec + dsn + workspace). Command lain
**tidak**: setiap parser membawa literal sendiri (`specPath := "spec"`,
`dsn := "sqlite:.formspec/data.db"`, workspace hardcoded). Literal itu adalah nilai
yang dipakai `dev` **justru ketika config file tidak ada** — jadi di setiap project
yang punya `formspec-app.yaml`, menjalankan command dari direktori project
diam-diam menyasar **database yang berbeda** dari yang disajikan `dev`.

Diukur di `examples/kafe` (config: `spec: spec`, `dsn: sqlite:.formspec/kafe.db`):

|           | `formspec dev`                                          | `formspec repl` (tanpa flag)              |
| --------- | ------------------------------------------------------- | ----------------------------------------- |
| spec      | `spec`                                                  | `spec` (kebetulan sama)                   |
| DSN       | `sqlite:…/kafe.db`                                      | `sqlite:…/**data.db**` ← **file berbeda** |
| workspace | `kafe` (satu-satunya yang dideklarasikan, diadopsi #48) | **`demo`** ← tenant berbeda               |

Bukti:

- `formspec repl --no-sync -e 'ctx.db().query("PRAGMA database_list")'` →
  `/workspaces/formspec/examples/kafe/.formspec/data.db`.
- `ctx.workspace.id` → **`demo`**; `dev` mengumumkan `workspace: kafe`.
- Tenant di `kafe.db`: `default` + `kafe` — dan `default` adalah jejak tulisan
  yang mendarat di tenant yang tidak pernah dibaca aplikasi.

Konsekuensinya bukan teoretis. Alur repair yang **baru saja** diperbaiki (4.2.7)
adalah `formspec repl --no-sync -f repair.star` → `formspec migrate apply`. Dua
perintah itu masing-masing membuka DSN default-nya sendiri: perbaikannya mendarat
di `data.db`, lalu `migrate apply` menyelaraskan `data.db`, sementara `dev`
menyajikan `kafe.db` yang tetap belum diperbaiki. `formspec seed` lebih jauh lagi:
default workspace-nya `demo`, jadi seed tanpa flag membuat tenant **ketiga**.
(`make seed-kafe` menghindarinya karena eksplisit `--workspace kafe` — artinya
bug-nya tersembunyi di balik Makefile, bukan tidak ada.)

## 2. Aturan yang dipakai (sama dengan `dev`)

Urutan resolusi `dev` (`cmd/formspec/dev.go` + `dev_config.go`):

1. flag CLI mengisi default;
2. `formspec-app.yaml` (CWD, lalu `formspec-sidecar.yaml`) menimpa nilai yang
   masih default → _config adalah default, flag menang_;
3. DSN sqlite relatif di-anchor ke project root (`resolveDSN`);
4. aturan workspace #48 (`activeWorkspaceFor`): eksplisit → dipakai apa adanya
   (+ peringatan bila tidak dideklarasikan); tidak eksplisit & tepat satu
   dideklarasikan → diadopsi; beberapa → tetap default + peringatan.

Command data-lifecycle memakai **urutan yang sama**, lewat helper bersama — bukan
salinan. Pencarian config tetap **CWD saja** seperti `dev` (parity; `dev` punya
`chdirIfPositionalArg` untuk kasus di luar itu).

## 3. Perubahan

| #   | File                                            | Perubahan                                                                                                                                                           |
| --- | ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | `cmd/formspec/project_defaults.go` (baru)       | `loadProjectDefaults()` (spec/dsn/workspace dari config file, dengan fallback literal lama) + `applyProjectWorkspace()` (aturan #48). Satu tempat untuk aturan itu. |
| 2   | `cmd/formspec/{repl,migrate,diff,seed}.go`      | Memakai helper: literal lokal dibuang; `repl`/`seed` berhenti hardcode `demo`; `repl` meneruskan workspace teresolusi ke `NewCtxAPI`.                               |
| 3   | `cmd/formspec/{logs,archive,summary,backup}.go` | Sama (semua command yang membuka DB project). `backup`/`restore` sudah benar untuk workspace (#48, 4.8.7) — yang ditambahkan hanya spec/dsn dari config.            |
| 4   | `cmd/formspec/project_defaults_test.go` (baru)  | Kunci perilaku: config dibaca; flag menang; DSN di-anchor; workspace diadopsi bila tunggal / tidak ditebak bila jamak. Dibuktikan gagal tanpa helper.               |
| 5   | `docs/cli-tools/*.md`                           | Satu aturan dijelaskan sekali (config → flag → anchor → workspace) dan dirujuk dari bagian per command.                                                             |
| 6   | `docs_internal/plan/todo.md`                    | Item 3.10 ditutup + sisa nyata apa pun yang muncul.                                                                                                                 |
| 7   | `docs_internal/changelog/2026-09-29-00N-*.md`   | Catatan perubahan.                                                                                                                                                  |

## 4. Verifikasi

- `go test ./cmd/formspec/`.
- E2E: dari `examples/kafe` **tanpa flag** — `formspec repl --no-sync -e` membuka
  `kafe.db` dan `ctx.workspace.id == "kafe"`; `formspec migrate plan` menyasar
  `kafe.db`; `formspec diff` → `No differences`.
- Flag tetap menang: `--dsn sqlite:/tmp/x.db --workspace default` mengarah ke sana.
- `gofmt` + `go build ./...` + `go vet`.

## 5. Di luar scope

- `formspec get`/`delete`/`validate`/`check` tidak membuka DB project (spec-tree
  saja) → tidak berubah.
- Mencari config **di sebelah spec** (bukan CWD) saat dipanggil dari luar project:
  perilaku baru yang membuatnya menyimpang dari `dev`; dicatat sebagai item
  `[⏸️]` bila perlu.
