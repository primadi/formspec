# 2026-09-09-005 — formspec init: scaffold App manifest dengan confirm default

## Apa

`cmd/formspec/init.go` kini menulis scaffold `spec/apps/<module>.yaml`
(sebelumnya hanya membuat dir `spec/apps/` — manifest App dibuat manual).
Scaffold berisi `kind: App` minimal (root_url `/app/<module>`, modules, menu)
plus blok App-wide default confirm dialogs (plan confirm-dialogs):

```yaml
confirm:
  create: "Buat {name} baru?"
  update: "Simpan perubahan {name} ini?"
  delete: "Hapus data {name}?"
```

`{name}` di-interpolate renderer dengan nama entity. Override per
form/action tetap bisa; set verb ke `""` untuk mematikan satu jenis.

## Catatan

- `formspec validate` tanpa `--schema` masih FAIL karena schema remote
  (`schemas.formspec.dev`) belum ter-publish untuk field `confirm` — kondisi
  yang sama dengan kind Workspace (deferred todo 2.11.10). Dengan schema
  lokal: pass.
- Output init ditambah baris "spec/apps/<module>.yaml — kind: App scaffold".

## File terdampak

- `cmd/formspec/init.go`
