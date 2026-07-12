# Plan: Positional Directory Argument untuk `forma dev`

## Problem

Setiap kali `forma dev` harus menyertakan `--spec` dan `--dsn`:
```
go run ./cmd/forma/ dev --spec examples/Clinic-UI-Showcase/spec --dsn "sqlite:.forma/clinic.db"
```

## Solusi

1. **Config file** — `forma-sidecar.yaml` di root project dengan `spec` dan `dsn`.
   Sudah ada `mergeConfigFile()` di `dev_config.go`, tinggal membuat file-nya.

2. **Positional directory argument** — `forma dev .` atau `forma dev /path/to/project`
   akan `chdir` ke folder itu sebelum discovery, sehingga `./spec` dan
   `forma-sidecar.yaml` di-resolve relatif terhadap folder tersebut.

## File

- `cmd/forma/dev.go` — fungsi `chdirIfPositionalArg()` + panggilan di `runDev()`
- `forma-sidecar.yaml` — config default untuk showcase

## Level of Effort

Small — satu fungsi helper + config file.
