# FormSpec CLI Tools

**Status:** Draft
**License:** Creative Commons CC0

> FormSpec punya **2 CLI inti**, bukan lebih: `formspec` (developer/App Owner, permukaan luas) dan `formspec-ctl` (emergency, Platform Operator, sebenarnya bagian dari binary `formspec-ctl`). Dokumen di folder ini adalah referensi verb-per-verb; keputusan normatif tetap di `docs/spec/`. `formspec consult` (baris ke-5 tabel di bawah) adalah lapisan AI opsional di atas `formspec` yang sudah ada — bukan CLI inti ketiga, lihat `05-formspec-consult.md` §2.

---

## Dokumen

| #   | CLI                                                       | Audiens                         | Wujud                                                                                                                                              | Dokumen                                                |
| --- | --------------------------------------------------------- | ------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ |
| 1   | `formspec dev`                                            | App/Module Owner, Developer     | Binary `cmd/formspec` — single-process dev server                                                                                                  | [`01-formspec-dev.md`](./01-formspec-dev.md)           |
| 2   | `formspec` (CLI reference)                                | App/Module Owner, Developer     | Binary `cmd/formspec` — `apply`, `generate`, dll                                                                                                   | [`02-formspec-cli.md`](./02-formspec-cli.md)           |
| 3   | `formspec generate` (browser client)                      | Frontend Developer              | `@formspec/client` (npm, `sdk/browser`) + codegen                                                                                                  | [`03-formspec-generate.md`](./03-formspec-generate.md) |
| 4   | `formspec-ctl`                                            | Cloud Owner / Platform Operator | Subcommand di binary `formspec-ctl` (D43)                                                                                                          | [`04-formspec-ctl.md`](./04-formspec-ctl.md)           |
| 5   | `formspec consult` (AI business consultant & spec author) | App/Module Owner, Developer     | Binary terpisah `formspec-consult` (TS/Bun) + subcommand `formspec mcp-serve` (Go) — opsional; arsitektur lengkap di [`docs/ai/`](../ai/README.md) | [`05-formspec-consult.md`](./05-formspec-consult.md)   |

---

## Kenapa 2, Bukan 1

`formspec` dan `formspec-ctl` melayani dua persona dan dua sisi yang berbeda secara fundamental:

|                            | `formspec`                                              | `formspec-ctl`                                                                                                                       |
| -------------------------- | ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| Sisi                       | Resource Plane + registrasi ke Control Plane            | Control Plane murni                                                                                                                  |
| Persona                    | App/Module Owner, Developer                             | Cloud Owner                                                                                                                          |
| Filosofi                   | Kaya fitur, DX-first (`dev`, `repl`, `generate`)        | Minimal, tidak bergantung platform yang diperbaiki (D43)                                                                             |
| Kegagalan yang ditoleransi | Boleh gagal kalau Control Plane down (developer tunggu) | **Tidak boleh** ikut gagal kalau bagian lain `formspec-ctl` rusak — inilah alasan ia bukan proses terpisah dengan dependency sendiri |

Menyatukan keduanya ke satu CLI akan merusak properti paling penting `formspec-ctl`: independensi dari platform yang sedang ia perbaiki.

---

## Status Implementasi (Ringkas)

| CLI                | Status kode hari ini                                                                                                                                                                                                                                                                                                                      |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `formspec`         | `apply` dan `generate --lang typescript` implemented sebagai subcommand nyata di `cmd/formspec` (lihat `03-formspec-generate.md`). ~19 verb lain dikenali dispatcher tapi cuma print "not implemented yet"                                                                                                                                |
| `formspec-ctl`     | Subcommand `serve` implemented (lihat `docs/runtimes/01-formspec-ctl.md`). Verb emergency (`freeze`, `revoke`, `key`, `policy`, `log`) dikenali dispatcher tapi cuma print "not implemented yet" — bergantung fondasi yang sendiri belum dibangun (persistent store, OPA, transparency log — lihat `docs/runtimes/01-formspec-ctl.md` §7) |
| `formspec consult` | Belum diimplementasikan sama sekali — target desain, lihat `docs/ai/README.md` §5 dan `docs_internal/plan/todo.md` Fase 10                                                                                                                                                                                                                         |

---

## Kaitan dengan Dokumen Lain

| Kalau Anda ingin tahu...                                                | Baca                                      |
| ----------------------------------------------------------------------- | ----------------------------------------- |
| Fitur/API/desain server yang jadi target verb-verb ini (`formspec-ctl`) | `docs/runtimes/01-formspec-ctl.md`        |
| Skema YAML yang divalidasi/di-apply                                     | `docs/spec/`                              |
| Bagaimana `formspec apply` masuk ke pipeline deployment production      | `docs/architecture/03-deployment-flow.md` |
