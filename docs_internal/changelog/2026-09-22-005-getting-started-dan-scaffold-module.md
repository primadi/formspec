# 2026-09-22-005 — Getting Started guide + fix scaffold Module (todo 9.4.1)

**Plan/Todo**: item **9.4.1** (`docs_internal/plan/todo.md`).

`docs/guides/getting-started.md` ditulis untuk pembaca yang benar-benar baru:
install → `formspec init` → bentuk App (`access` × `app_renderer`) → model domain
(characteristic + `formspec new entity`) → `formspec dev` → jalur logic bisnis →
"berikutnya". Terdaftar di `docs/guides/README.md` dan sidebar VitePress
`docs-site/.vitepress/config.mts`; `docs-site/docs` adalah symlink ke `../docs`
(dikonfirmasi), jadi kontennya ikut otomatis.

**Prinsip penulisan**: setiap perintah dan setiap potongan YAML **dijalankan lebih
dulu** terhadap CLI/validator nyata. Ini menemukan dua bug nyata.

### Bug 1 — `formspec init` menghasilkan project yang gagal gate-nya sendiri

App yang di-scaffold berisi `spec.modules: [<module>]`, tetapi scaffold **tidak
pernah** menulis `kind: Module`. Hasil pada project yang baru dibuat:

```text
[FAIL] spec/apps/tokoku.yaml#0
       reference: App mounts module(s) tokoku, which no kind: Module declares
       — the App would mount nothing for them (declared modules: )
2 manifest(s) validated, 1 problem(s) found
```

Jadi langkah "Next steps: … run `formspec validate --spec spec`" di output
`init` **gagal** pada project yang baru saja di-scaffold. Diperbaiki dengan
template `cmd/formspec/template_init/spec/modules/_module_/module.yaml` plus
placeholder direktori `_module_` → nama module di `template_init.go`, sehingga
file mendarat di `spec/modules/{module}/module.yaml` — layout yang sama dengan
yang dipakai `formspec new entity`. Setelah perbaikan: **3 manifest, 0 problem**.
`TestRunInit_ScaffoldsProject` diperluas untuk mem-pin keberadaan + isi file itu.

### Bug 2 — draft contoh Entity di guide tidak valid

Draft pertama (ditulis dari hafalan) **ditolak** validator:
`/spec: missing property 'version'`, `state_machine/states[i]: validation failed`
(`StateDecl` butuh `name` **dan** `label`), dan
`additional properties 'permissions' not allowed` — permission bukan bagian dari
spec Entity (ia diturunkan framework dari nama module/entity dan diatur lewat
Role). Guide kini memuat versi yang **terverifikasi lolos** (`version: v1`,
`characteristic`, `plural`, `fields` dengan `required`/`rules`, `state_machine`
dengan `label`, `expose`), plus tabel error nyata dari validator.

**File terkena dampak**: `docs/guides/getting-started.md` (baru),
`docs/guides/README.md`, `docs-site/.vitepress/config.mts`,
`cmd/formspec/template_init/spec/modules/_module_/module.yaml` (baru),
`cmd/formspec/template_init.go`, `cmd/formspec/init.go`, `cmd/formspec/init_test.go`.

**Bukti**: `formspec init tokoku && formspec validate --spec spec` → **3 manifest,
0 problem** (sebelumnya 1 problem); contoh Entity di guide → **4 manifest,
0 problem**; `go test ./...` hijau; `tsc` bersih; `vitest` 288 lulus.

**Sisa** → item **9.4.1 ⏸️** di todo: dua Entity dengan `metadata.name` sama di
satu module **tidak** dilaporkan error oleh `formspec validate` (diuji: keduanya
`[OK]`; perilaku runtime belum dipastikan). Guide menyatakan ini apa adanya
alih-alih mengklaim gerbang yang tidak ada.
