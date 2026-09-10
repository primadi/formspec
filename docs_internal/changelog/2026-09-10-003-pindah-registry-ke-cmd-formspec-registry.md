# 2026-09-10-003 — Pindah app content registry ke `cmd/formspec-registry/`

## Apa yang diubah

Folder `registry/` di root (app content yang hanya dipakai binary
`cmd/formspec-registry`) dipindah ke bawah foldernya:

```
registry/                       →  cmd/formspec-registry/
  embed.go + spec/              →    app-spec/            (package appspec, //go:embed spec)
  web/                          →    web/                 (package web, //go:embed all:dist)
  deploy/                       →    deploy/              (Dockerfile + k8s, non-Go)
  formspec-app.yaml             →    formspec-app.yaml    (config dev, sejajar main.go)
```

Folder root `registry/` kini dihapus — root repo bersih dari folder
single-consumer. Nama `app-spec` dipilih agar isinya murni spec; web dan
deploy dikeluarkan sebagai sibling `main.go` karena bukan bagian spec.

## Perubahan terkait

- `cmd/formspec-registry/main.go` — import
  `cmd/formspec-registry/app-spec` (package `appspec`, dulu `registry`)
  dan `cmd/formspec-registry/web`; `native.SpecFS()` → `appspec.SpecFS()`.
- `cmd/formspec-registry/formspec-app.yaml` — `spec:
cmd/formspec-registry/app-spec/spec`; DSN tidak berubah
  (`sqlite:.formspec/registry.db`) sehingga DB existing tetap terpakai.
- `scripts/run-registry.sh` — `--config cmd/formspec-registry/formspec-app.yaml`.
- `Makefile` `build-registry` — sync SPA → `cmd/formspec-registry/web/dist`.
- `.vscode/settings.json` — glob yaml.schemas →
  `cmd/formspec-registry/app-spec/spec/**`.
- Docs living: `docs/registry/01-concepts.md`, `04-rest-api.md`,
  `05-self-hosting.md`; link rusak `docs/spec/platform/08-project-layout.md`
  (`registry/README.md` tidak pernah ada) → `docs/registry/05-self-hosting.md`;
  komentar path di `deploy/{Dockerfile,README.md}` dan
  `deploy/k8s/datastore-valkey.yaml`.

## Kenapa

App content milik satu binary sebaiknya tinggal di bawah binary itu
(co-location), bukan menambah top-level folder di root repo. Konvensi Go
tetap terjaga: `cmd/` hanya berisi package main (`main.go`) dan aset
non-Go/leaf packages sebagai subfolder.

## File terkena dampak

- `cmd/formspec-registry/{app-spec,web,deploy,formspec-app.yaml,main.go}`
- `scripts/run-registry.sh`, `Makefile`, `.vscode/settings.json`
- `docs/registry/*`, `docs/spec/platform/08-project-layout.md`

## Referensi

- `docs/spec/platform/08-project-layout.md` — project layout
- `docs/registry/05-self-hosting.md` — dev & prod run registry
- Changelog sebelumnya: `2026-09-10-002-pindah-formspec-app-yaml-ke-registry.md`
