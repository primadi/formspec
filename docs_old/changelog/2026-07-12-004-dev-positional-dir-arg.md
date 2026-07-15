# Dev: Positional Directory Argument + Config Default

## Perubahan

1. **`cmd/forma/dev.go`** — fungsi baru `chdirIfPositionalArg()`:
   - Jika arg pertama adalah direktori (bukan flag), `os.Chdir()` ke sana
   - Sisa arg diproses sebagai flag normal
   - Contoh: `forma dev examples/Clinic-UI-Showcase/` → chdir + discover spec

2. **`forma-sidecar.yaml`** — config default di root repo:
   ```yaml
   spec: examples/Clinic-UI-Showcase/spec
   dsn: sqlite:.forma/clinic.db
   ```
   Sekarang `go run ./cmd/forma/ dev` langsung pakai showcase tanpa flag.

## File Terkena Dampak

- `cmd/forma/dev.go` (+ fungsi `chdirIfPositionalArg`, ~15 baris)
- `forma-sidecar.yaml` (file baru)

## Contoh Pemakaian

```bash
# Dari root repo — baca forma-sidecar.yaml otomatis
go run ./cmd/forma/ dev

# Pakai folder tertentu — chiri + discover
go run ./cmd/forma/ dev examples/Clinic-UI-Showcase/

# Flag tetap override config
go run ./cmd/forma/ dev --spec my-spec
```

## Referensi

- `docs/spec/01-overview.md` — DX principles
- Plan: `docs/plan/dev-positional-dir-arg.md`
