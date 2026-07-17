# Project Layout

**Version:** 0.1.0 · **Status:** Draft (§3–§5 adalah target desain, belum diimplementasikan — lihat §5)

> Dokumen ini mengontrakkan struktur folder project aplikasi Forma di disk, dan
> bagaimana satu workspace dengan banyak Module bisa memiliki handler
> implementasi dalam bahasa yang berbeda-beda per Module.

---

## 1. Direktori Standar

```
myapp/
  forma-app.yaml                 # kind: Config — konfigurasi root (dsn, addr, dst.)
  apps/
    internal.yaml                # kind: App
    public.yaml                  # kind: App (App kedua, workspace yang sama)
  modules/
    billing/
      module.yaml                # kind: Module — termasuk deklarasi runtime (§3)
      documents/invoice.yaml      # kind: Document
      services/tax-calculator.yaml
      scripts/invoice_send.star  # script Starlark
      assets/                    # custom UI component (asset escape hatch)
      impl/                      # kode handler Module ini — bahasa BEBAS per Module (§3)
        src/invoice-handler.ts   # mis. TypeScript, kalau module.yaml runtime: typescript
    pharmacy/
      module.yaml                # runtime: php
      documents/prescription.yaml
      impl/
        composer.json
        src/PrescriptionHandler.php
```

Nama folder adalah konvensi, bukan kontrak keras. Loader **wajib** menemukan
manifest dengan men-scan `*.yaml`, bukan berdasarkan path tetap — tidak ada
yang melarang workspace kecil menyimpan satu `forma.yaml` di root alih-alih
`apps/<name>.yaml`; struktur `apps/` direkomendasikan begitu workspace punya
lebih dari satu App.

## 2. Tiga Jenis File, Plus `impl/` Per Module

Satu-satunya aturan keras: tiga jenis file dalam manifest —`.yaml` (deskripsi),
`.star` (logika, Starlark), `assets/*` (statis/custom UI). `impl/` adalah kode
sumber build-time (Go native, atau bahasa lain untuk `impl.type: sidecar`) —
di-commit, tidak masuk artifact deployment. **`impl/` bersifat per-Module**,
bukan satu folder global di root — setiap Module membawa kode handlernya
sendiri, dalam bahasa yang ia deklarasikan sendiri (§3).

**Git adalah sumber kebenaran.** Manifest selalu berupa file teks di
repositori — tidak pernah format biner proprietary maupun state tersembunyi
di database. Tooling authoring (scaffold `forma new <kind>`, editor visual
di admin panel) **menulis kembali YAML ke file/PR**, bukan ke DB
tersembunyi; git tetap satu-satunya sumber kebenaran. Skema JSON per kind
memberi validasi/autocomplete editor (LSP), tapi tidak menggantikan file
sebagai artifact otoritatif.

## 3. Runtime Per Module

Satu workspace = multi App + multi Module (lihat
[`02-workspace-app-module.md`](02-workspace-app-module.md)), dan tiap Module
bisa dimiliki tim berbeda dengan preferensi bahasa berbeda. Karena itu **setiap
Module mendeklarasikan runtime implementasinya sendiri** — bukan satu runtime
global untuk seluruh project:

```yaml
# modules/billing/module.yaml
apiVersion: forma.dev/v1alpha1
kind: Module
metadata: { name: billing }
spec:
  runtime: typescript        # module ini dilayani proses Node terpisah
```

```yaml
# modules/pharmacy/module.yaml
apiVersion: forma.dev/v1alpha1
kind: Module
metadata: { name: pharmacy }
spec:
  runtime: php                # module ini dilayani proses PHP terpisah
```

Nilai `runtime` yang valid: `local` (default — native compiled-in, tanpa proses
sidecar), `typescript` (alias `node`), `php`, `python`, `go`, `java`, `dotnet`,
`ruby`, `rust` — satu per bahasa yang punya `lib-forma-*` (lihat
[`docs/runtimes/04-forma-sidecar.md`](../../runtimes/04-forma-sidecar.md) §4.4).
Action ber-`impl.type: sidecar` di dalam sebuah Module secara implisit dilayani
runtime yang dideklarasikan Module tersebut — tidak perlu diulang per-action.

## 4. Orkestrasi Multi-Proses

Engine (`forma dev` untuk dev, pod Resource Plane untuk produksi) **wajib**
menjalankan **satu proses sidecar per runtime unik** yang muncul di seluruh
Module workspace — bukan satu proses global untuk seluruh project:

```
Workspace dengan 2 Module, runtime berbeda:
  billing  → runtime: typescript
  pharmacy → runtime: php

Engine start:
  → proses Node untuk billing   (socket: {state-dir}/sidecar/billing.sock)
  → proses PHP untuk pharmacy   (socket: {state-dir}/sidecar/pharmacy.sock)
  → Module dengan runtime: local (atau tidak dideklarasikan) tetap native,
    tidak butuh proses sidecar sama sekali
```

**Kontrak dispatch:** `SidecarExecutor` (lihat
[`docs/runtimes/04-forma-sidecar.md`](../../runtimes/04-forma-sidecar.md)
§3.2/§4.2) me-resolve socket tujuan dari **Module pemilik action**, bukan dari
satu `--app-endpoint` global — rantai resolusi: `action → Module pemilik →
runtime Module → socket proses yang menjalankan runtime itu`. Dua Module
dengan runtime yang sama (mis. dua Module TypeScript) BOLEH berbagi satu
proses sidecar atau dipisah — keputusan ini bukan bagian kontrak, hanya
kebijakan implementasi engine.

## 5. Status Implementasi Hari Ini (Gap)

Model saat ini di `forma dev`/`forma-sidecar`
([`docs/runtimes/04-forma-sidecar.md`](../../runtimes/04-forma-sidecar.md)
§5–§6) hanya mendukung **satu runtime untuk seluruh project**, lewat flag
global `--runtime` (auto-detect dari file marker di root: `composer.json` →
php, `package.json` → node, dst.) dan satu proses app di satu socket. Model
multi-runtime-per-Module di §3–§4 dokumen ini adalah **target desain, belum
diimplementasikan**. Perubahan yang dibutuhkan sebelum ini berjalan:

1. Schema `kind: Module` bertambah field `spec.runtime`.
2. Engine (`forma dev` dan pod Resource Plane produksi) spawn N child process
   sesuai jumlah runtime unik, bukan 1 proses tunggal.
3. `SidecarExecutor` melakukan routing per-Module, bukan satu
   `--app-endpoint` global.

Dicatat normatif di sini — bukan sekadar wishlist — supaya keputusan desain
ini tidak hilang sebelum implementasinya menyusul di fase restrukturisasi
kode.

## 6. Referensi

| Dokumen | Isi |
|---|---|
| [`02-workspace-app-module.md`](02-workspace-app-module.md) | Model workspace/App/Module yang jadi dasar §3 |
| [`docs/runtimes/04-forma-sidecar.md`](../../runtimes/04-forma-sidecar.md) | Protokol sidecar per proses (§4), mode eksekusi (§5), gap implementasi (§8) |
| [`docs/architecture/08-repo-structure.md`](../../architecture/08-repo-structure.md) | Bagaimana `sdk/*` dan `internal/sidecar` merealisasikan kontrak ini di kode |
