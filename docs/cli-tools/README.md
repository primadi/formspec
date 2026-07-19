# Forma CLI Tools

**Status:** Draft
**License:** Creative Commons CC0

> Forma punya **2 CLI inti**, bukan lebih: `forma` (developer/App Owner, permukaan luas) dan `forma-ctl` (emergency, Platform Operator, sebenarnya bagian dari binary `forma-ctl`). Dokumen di folder ini adalah referensi verb-per-verb; keputusan normatif tetap di `docs/spec/`. `forma consult` (baris ke-5 tabel di bawah) adalah lapisan AI opsional di atas `forma` yang sudah ada — bukan CLI inti ketiga, lihat `05-forma-consult.md` §2.

---

## Dokumen

| # | CLI | Audiens | Wujud | Dokumen |
|---|---|---|---|---|
| 1 | `forma dev` | App/Module Owner, Developer | Binary `cmd/forma` — single-process dev server | [`01-forma-dev.md`](./01-forma-dev.md) |
| 2 | `forma` (CLI reference) | App/Module Owner, Developer | Binary `cmd/forma` — `apply`, `generate`, dll | [`02-forma-cli.md`](./02-forma-cli.md) |
| 3 | `forma generate` (browser client) | Frontend Developer | `@forma/client` (npm, `sdk/browser`) + codegen | [`03-forma-generate.md`](./03-forma-generate.md) |
| 4 | `forma-ctl` | Cloud Owner / Platform Operator | Subcommand di binary `forma-ctl` (D43) | [`04-forma-ctl.md`](./04-forma-ctl.md) |
| 5 | `forma consult` (AI business consultant & spec author) | App/Module Owner, Developer | Binary terpisah `forma-consult` (TS/Bun) + subcommand `forma mcp-serve` (Go) — opsional; arsitektur lengkap di [`docs/ai/`](../ai/README.md) | [`05-forma-consult.md`](./05-forma-consult.md) |

---

## Kenapa 2, Bukan 1

`forma` dan `forma-ctl` melayani dua persona dan dua sisi yang berbeda secara fundamental:

| | `forma` | `forma-ctl` |
|---|---|---|
| Sisi | Resource Plane + registrasi ke Control Plane | Control Plane murni |
| Persona | App/Module Owner, Developer | Cloud Owner |
| Filosofi | Kaya fitur, DX-first (`dev`, `repl`, `generate`) | Minimal, tidak bergantung platform yang diperbaiki (D43) |
| Kegagalan yang ditoleransi | Boleh gagal kalau Control Plane down (developer tunggu) | **Tidak boleh** ikut gagal kalau bagian lain `forma-ctl` rusak — inilah alasan ia bukan proses terpisah dengan dependency sendiri |

Menyatukan keduanya ke satu CLI akan merusak properti paling penting `forma-ctl`: independensi dari platform yang sedang ia perbaiki.

---

## Status Implementasi (Ringkas)

| CLI | Status kode hari ini |
|---|---|
| `forma` | `apply` dan `generate --lang typescript` implemented sebagai subcommand nyata di `cmd/forma` (lihat `03-forma-generate.md`). ~19 verb lain dikenali dispatcher tapi cuma print "not implemented yet" |
| `forma-ctl` | Subcommand `serve` implemented (lihat `docs/runtimes/01-forma-ctl.md`). Verb emergency (`freeze`, `revoke`, `key`, `policy`, `log`) dikenali dispatcher tapi cuma print "not implemented yet" — bergantung fondasi yang sendiri belum dibangun (persistent store, OPA, transparency log — lihat `docs/runtimes/01-forma-ctl.md` §7) |
| `forma consult` | Belum diimplementasikan sama sekali — target desain, lihat `docs/ai/README.md` §5 dan `docs/plan/todo.md` Fase 10 |

---

## Kaitan dengan Dokumen Lain

| Kalau Anda ingin tahu... | Baca |
|---|---|
| Fitur/API/desain server yang jadi target verb-verb ini (`forma-ctl`) | `docs/runtimes/01-forma-ctl.md` |
| Skema YAML yang divalidasi/di-apply | `docs/spec/` |
| Bagaimana `forma apply` masuk ke pipeline deployment production | `docs/architecture/03-deployment-flow.md` |
