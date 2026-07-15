# Forma Runtimes

**Status:** Draft
**License:** Creative Commons CC0

> Forma terdiri dari **4 komponen runtime**: 3 binary + 1 Go library. Dokumen di folder ini menjelaskan tiap komponen secara individual — fitur, desain internal, dan API/protokol — sebagai pelengkap gambaran topologi besar di `docs/architecture/`.
>
> **Perbedaan dengan `docs/architecture/`:** folder itu menjelaskan bagaimana komponen-komponen ini **saling berhubungan** di production (topologi, deployment, failover). Folder ini menjelaskan **isi tiap komponen itu sendiri** — cocok dibaca kalau Anda akan mengimplementasikan atau memodifikasi salah satu dari mereka.
>
> Untuk referensi CLI (`forma`, `forma-ctl`), lihat **[`docs/cli-tools/`](../cli-tools/README.md)**.

---

## Komponen

| # | Komponen | Wujud | Lisensi | Dokumen |
|---|---|---|---|---|
| 1 | **Forma Control** | Binary (3 mode: region/cluster/standalone) | FSL (open source) | [`01-forma-ctl.md`](./01-forma-ctl.md) |
| 2 | **Forma Resource** | Go library (`import "github.com/primadi/forma/resource"`) | FSL (open source) | [`02-forma-resource.md`](./02-forma-resource.md) |
| 3 | **Forma Operator** | Binary (K8s CRD controller) | **Closed source** | [`03-forma-operator.md`](./03-forma-operator.md) |
| 4 | **Forma Sidecar** | Binary (embed Forma Resource + socket listener) | FSL (open source) | [`04-forma-sidecar.md`](./04-forma-sidecar.md) |

```
                    ┌─────────────────┐
                    │  Forma Control   │  ← source of truth (region) /
                    │  (binary)        │    cache proxy (cluster) /
                    └────────┬────────┘    all-in-one (standalone)
                             │ plane protocol (pull-based)
              ┌──────────────┴──────────────┐
              ▼                             ▼
   ┌─────────────────────┐       ┌─────────────────────────┐
   │ App Go native        │       │ App non-Go (PHP/Python)  │
   │  import "forma"       │       │  ┌─────────────────────┐│
   │  (Forma Resource      │       │  │ Forma Sidecar        ││
   │   compiled-in)        │       │  │ (Forma Resource      ││
   │                       │       │  │  compiled-in + socket)││
   └───────────────────────┘       │  └──────────┬──────────┘│
                                    │             │ socket     │
                                    │  ┌──────────▼──────────┐│
                                    │  │ app.php + lib-forma-php││
                                    │  └─────────────────────┘│
                                    └─────────────────────────┘

   ┌─────────────────────┐
   │  Forma Operator      │  ← closed source, K8s pod terpisah,
   │  (CRD controller)     │    membuat/mengelola Deployment di atas
   └───────────────────────┘    (baik utk app Go native maupun sidecar)
```

**Poin kunci:** Forma Resource adalah **satu engine yang sama**, dipakai dua cara — di-compile langsung ke app Go, atau di-embed ke dalam proses Forma Sidecar untuk app bahasa lain. Bukan dua implementasi terpisah.

---

## Status Implementasi (Ringkas)

| Komponen | Status kode hari ini |
|---|---|
| Forma Control | Ada (`cmd/forma-ctl`), tapi single-mode (belum region/cluster/standalone split), storage in-memory, dan **pipeline register→deployment belum tersambung** — lihat `01-forma-ctl.md` §7 |
| Forma Resource | Facade publik nyata di `resource/` (`resource/forma.go` App, `resource/syncagent.go` SyncAgent — lihat `examples/reference-app`), tapi kedua jalur masih terpisah: `App` (serve) tidak menyambung ke `SyncAgent` (pull artifact) — lihat `02-forma-resource.md` §7 |
| Forma Operator | **Belum ada kode sama sekali** — dokumen adalah spesifikasi desain awal — lihat `03-forma-operator.md` §7 |
| Forma Sidecar | **Belum ada kode sama sekali**, stub-nya di `internal/action/sidecar.go` — lihat `04-forma-sidecar.md` §8 |

Tiap dokumen punya bagian **"Status Implementasi Hari Ini"** yang mencatat gap konkret antara desain dan kode, plus urutan pembangunan yang disarankan — supaya dokumen ini berguna sebagai peta kerja teknis, bukan cuma spesifikasi aspirasional.

---

## Kaitan dengan Dokumen Lain

| Kalau Anda ingin tahu... | Baca |
|---|---|
| Bagaimana semua komponen ini di-deploy & saling terhubung di production | `docs/architecture/` |
| Skema YAML normatif (Document, Entity, Action, dst) | `docs/spec/` |
| Fitur/desain/API internal satu komponen tertentu | Dokumen ini |
