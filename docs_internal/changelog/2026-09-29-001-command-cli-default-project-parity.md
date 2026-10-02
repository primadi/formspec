# 2026-09-29-001 — Command CLI memakai default project yang sama dengan `formspec dev`

**Plan**: `docs_internal/plan/cli-command-config-parity.md`
**Todo**: 3.10 ✅ (ditutup), 3.10.1 ⏸️ (dibuka)

## Apa yang diubah

`formspec dev` membaca `formspec-app.yaml`; command lain tidak. Masing-masing
membawa literal sendiri — `specPath := "spec"`,
`dsn := "sqlite:.formspec/data.db"`, workspace `"demo"` — yaitu nilai yang `dev`
pakai justru ketika config file **tidak ada**. Jadi di setiap project yang punya
`formspec-app.yaml` (semua hasil `formspec init`), menjalankan command dari
direktori project menyasar **database dan/atau tenant yang berbeda** dari yang
disajikan server.

**Terukur di `examples/kafe`** (config: `spec: spec`, `dsn: sqlite:.formspec/kafe.db`):

|           | `formspec dev`      | binary lama (27 Sep) | sesudah             |
| --------- | ------------------- | -------------------- | ------------------- |
| database  | `.formspec/kafe.db` | `.formspec/data.db`  | `.formspec/kafe.db` |
| workspace | `kafe`              | `demo`               | `kafe`              |

Bukti: `repl -e 'ctx.db().query("PRAGMA database_list")'` + `ctx.workspace.id`
pada binary lama vs baru (keduanya dijalankan dari `examples/kafe` tanpa flag).
Akibat nyata: alur repair (4.2.7) `repl --no-sync -f repair.star` → `migrate apply`
masing-masing mengerjakan DSN default-nya sendiri, sehingga repair mendarat di
`data.db` sementara `dev` tetap menyajikan `kafe.db` — terbaca sebagai "repair
tidak berefek". `formspec seed` tanpa flag membuat tenant **ketiga**: terukur
`74 inserted` dan `cafe_master_branches` menjadi
`[default:1, demo:2, kafe:2]`, padahal App hanya membaca `kafe`.

**Perubahan:** helper bersama `loadProjectDefaults()` +
`finishProjectDefaults()` (`cmd/formspec/project_defaults.go`) menerapkan urutan
`dev`: flag → config file → fallback → anchor DSN ke project root → aturan
workspace #48. Dipakai `repl`, `migrate`, `diff`, `seed`, `backup`, `restore`,
`archive`, `summary`, `logs`, `get`, `describe`, `delete`. `repl` meneruskan
workspace teresolusi ke `NewCtxAPI` (dulu hardcoded `"demo"`) dan mencetak
`repl: spec=… dsn=… workspace=…`; `seed` berhenti memakai literal `"demo"`.

**Guard kelasnya:** `TestNoCommandHardcodesProjectDefaults`
(`cmd/formspec/project_defaults_guard_test.go`) memindai paket dan menolak
literal-literal itu muncul kembali di command mana pun (dikecualikan: helper,
`dev.go` yang memilikinya, scaffold generator yang menulisnya sebagai **isi**
file config, dan test). Guard ini langsung menemukan dua tersangka tambahan yang
tidak saya sadari (`get`, `describe`, `delete` membaca spec tree dengan literal
`spec`) — sudah diperbaiki.

## Verifikasi

- `go test ./cmd/formspec/` hijau; guard dibuktikan gagal saat regresi disuntikkan
  ke `diff.go`.
- Dari `examples/kafe` tanpa flag: `repl` → `.formspec/kafe.db` + `ws=kafe`;
  `migrate plan` → `No pending migrations`; `diff` → `No differences`;
  `backup create --full` → `161 record(s)` di workspace `kafe`;
  `seed` → seluruh record menyasar `kafe` (tanpa tenant `demo` baru).
- Flag tetap menang (`--dsn`, `--workspace`); project tanpa config file tetap
  memakai fallback lama.
- `gofmt`, `go vet`, `go build ./...`.

## Sisa (item bernomor)

- **3.10.1 ⏸️** — config file dicari **hanya di CWD**, sama seperti `dev`
  (parity disengaja). Menjalankan command dari luar direktori project (mis.
  `cd /tmp && formspec migrate --spec …/examples/kafe/spec`) masih jatuh ke
  fallback, karena `formspec-app.yaml` tidak ada di CWD. Menutupnya berarti
  mencari config di sebelah spec — perilaku baru yang membuat CLI **menyimpang**
  dari `dev`; perlu keputusan dulu (menaikkan `dev` sekalian, atau tidak).
