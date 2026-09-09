# 2026-09-09-006 — formspec init: workspace seed + App di root_url /

## Apa

`formspec init` kini men-generate dua manifest:

1. `spec/workspaces/<module>.yaml` — `kind: Workspace` seed:
   - `metadata.name` = module name (= folder project, kebab-case) — slug
     menjadi prefix URL `/{ws}/...` dan scope tenant semua data.
   - `display_name` = nama project asli (boleh mengandung spasi).
   - Upsert ke formspec.core/workspace registry saat boot (plan
     named-workspaces.md).
2. `spec/apps/<module>.yaml` — App scaffold kini `root_url: /` (mounted di
   root workspace), bukan `/app/<module>` lagi.

Dir `spec/workspaces/` ditambahkan ke struktur scaffold. Output init
menyebut kedua file.

## Bug yang ditemukan saat test

Setelah `root_url` diubah jadi literal `/`, jumlah arg `fmt.Sprintf`
melebihi verb format → `%!(EXTRA string=...)` tercetak di YAML dan baris
`%…` bikin parse error. Fixed (arg count 4).

## File terdampak

- `cmd/formspec/init.go`
