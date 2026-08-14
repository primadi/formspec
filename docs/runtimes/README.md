# FormSpec Runtimes

**Status:** Draft
**License:** Creative Commons CC0

> FormSpec terdiri dari **4 komponen runtime**: 3 binary + 1 Go library. Dokumen di folder ini menjelaskan tiap komponen secara individual — fitur, desain internal, dan API/protokol — sebagai pelengkap gambaran topologi besar di `docs/architecture/`.
>
> **Perbedaan dengan `docs/architecture/`:** folder itu menjelaskan bagaimana komponen-komponen ini **saling berhubungan** di production (topologi, deployment, failover). Folder ini menjelaskan **isi tiap komponen itu sendiri** — cocok dibaca kalau Anda akan mengimplementasikan atau memodifikasi salah satu dari mereka.
>
> Untuk referensi CLI (`formspec`, `formspec-ctl`), lihat **[`docs/cli-tools/`](../cli-tools/README.md)**.

---

## Komponen

| #   | Komponen              | Wujud                                                        | Lisensi           | Dokumen                                                |
| --- | --------------------- | ------------------------------------------------------------ | ----------------- | ------------------------------------------------------ |
| 1   | **FormSpec Control**  | Binary (3 mode: region/cluster/standalone)                   | FSL (open source) | [`01-formspec-ctl.md`](./01-formspec-ctl.md)           |
| 2   | **FormSpec Resource** | Go library (`import "github.com/primadi/formspec/resource"`) | FSL (open source) | [`02-formspec-resource.md`](./02-formspec-resource.md) |
| 3   | **FormSpec Operator** | Binary (K8s CRD controller)                                  | **Closed source** | [`03-formspec-operator.md`](./03-formspec-operator.md) |
| 4   | **FormSpec Sidecar**  | Binary (embed FormSpec Resource + socket listener)           | FSL (open source) | [`04-formspec-sidecar.md`](./04-formspec-sidecar.md)   |
| 5   | **Engine API Layer**  | Lapisan HTTP runtime engine (`internal/api`)                 | FSL (open source) | [`05-engine-api-layer.md`](./05-engine-api-layer.md)   |

```
                    ┌─────────────────┐
                    │  FormSpec Control   │  ← source of truth (region) /
                    │  (binary)        │    cache proxy (cluster) /
                    └────────┬────────┘    all-in-one (standalone)
                             │ plane protocol (pull-based)
              ┌──────────────┴──────────────┐
              ▼                             ▼
   ┌─────────────────────┐       ┌─────────────────────────┐
   │ App Go native        │       │ App non-Go (PHP/Python)  │
   │  import "formspec"       │       │  ┌─────────────────────┐│
   │  (FormSpec Resource      │       │  │ FormSpec Sidecar        ││
   │   compiled-in)        │       │  │ (FormSpec Resource      ││
   │                       │       │  │  compiled-in + socket)││
   └───────────────────────┘       │  └──────────┬──────────┘│
                                    │             │ socket     │
                                    │  ┌──────────▼──────────┐│
                                    │  │ app.php + lib-formspec-php││
                                    │  └─────────────────────┘│
                                    └─────────────────────────┘

   ┌─────────────────────┐
   │  FormSpec Operator      │  ← closed source, K8s pod terpisah,
   │  (CRD controller)     │    membuat/mengelola Deployment di atas
   └───────────────────────┘    (baik utk app Go native maupun sidecar)
```

**Poin kunci:** FormSpec Resource adalah **satu engine yang sama**, dipakai dua cara — di-compile langsung ke app Go, atau di-embed ke dalam proses FormSpec Sidecar untuk app bahasa lain. Bukan dua implementasi terpisah.

---

## Status Implementasi (Ringkas)

| Komponen          | Status kode hari ini                                                                                                                                                                                                                                                     |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| FormSpec Control  | Ada (`cmd/formspec-ctl`), tapi single-mode (belum region/cluster/standalone split), storage in-memory, dan **pipeline register→deployment belum tersambung** — lihat `01-formspec-ctl.md` §7                                                                             |
| FormSpec Resource | Facade publik nyata di `resource/` (`resource/formspec.go` App, `resource/syncagent.go` SyncAgent — lihat `examples/reference-app`), tapi kedua jalur masih terpisah: `App` (serve) tidak menyambung ke `SyncAgent` (pull artifact) — lihat `02-formspec-resource.md` §7 |
| FormSpec Operator | Implementasi awal ada (`cmd/formspec-operator` + `internal/operator`: tiga reconciler, verifikasi ed25519, reporter) — endpoint reporting sisi `formspec-ctl` belum ada — lihat `03-formspec-operator.md` §7                                                             |
| FormSpec Sidecar  | Implementasi awal ada (`cmd/formspec-sidecar` + `internal/sidecar`: `SidecarExecutor`, socket server `POST /ctx/*`, dua mode runtime, pull artifact) — hot-rebuild & transaksi multi-operasi belum — lihat `04-formspec-sidecar.md` §8                                   |

Tiap dokumen punya bagian **"Status Implementasi Hari Ini"** yang mencatat gap konkret antara desain dan kode, plus urutan pembangunan yang disarankan — supaya dokumen ini berguna sebagai peta kerja teknis, bukan cuma spesifikasi aspirasional.

---

## Kaitan dengan Dokumen Lain

| Kalau Anda ingin tahu...                                                | Baca                 |
| ----------------------------------------------------------------------- | -------------------- |
| Bagaimana semua komponen ini di-deploy & saling terhubung di production | `docs/architecture/` |
| Skema YAML normatif (Document, Entity, Action, dst)                     | `docs/spec/`         |
| Fitur/desain/API internal satu komponen tertentu                        | Dokumen ini          |
