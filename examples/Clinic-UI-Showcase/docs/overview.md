# Overview — Clinic UI Showcase (`klinik-sehat`)

## Apa Itu Aplikasi Ini

**Clinic UI Showcase** adalah aplikasi klinik (pasien, dokter, kunjungan,
kasir, rekam medis + farmasi: obat, resep, penjualan bebas) yang
dibangun di atas **FormSpec** — dengan tujuan utama **bukan** business
logic, melainkan menjadi **fixture yang meng-exercise semua fitur frontend
renderer** FormSpec: seluruh 12 UI kind, semua pola lifecycle (§1.7 spec
frontend), dan derived-by-default (D17).

## Tujuan Showcase

1. **Derived by Default (D17)** — entity tertentu **sengaja tanpa manifest
   UI sama sekali**; renderer harus men-derive Table, Form, detail Page,
   dan Menu entry-nya sendiri (`polyclinic`, `doctor`, `patient`,
   `medicine`).
2. **12 UI Kinds** — setiap kind punya perwakilan: Page (block + tabs +
   route `:param`), Form (modal/drawer/separate page + wizard step), Table
   (filter, bulk actions), Dashboard (grid + widget lintas module), Widget
   (metric + chart), Report (params + export), Wizard (multi-step + partial
   save), Kanban (drag = update state), Timeline (entity append-only),
   Print (thermal + pdf + html), Theme, Menu.
3. **Pola Lifecycle (§1.7)** — plain CRUD, 2-step + auto-save, 1-step
   `create-submit`, reference (update-only), append-only — semuanya
   diwakili entity nyata.
4. **Fitur lintas-kind** — permission-driven UI, state machine → tombol
   transisi, FormSpecExpr (`visible_when`/`required_when`), natural key
   dengan reset harian, child table jsonb, relation lintas module, CAS
   optimistic concurrency, realtime event.

Matriks lengkap **fitur renderer → file** ada di
[`README.md`](../README.md) di root example.

## Tech Stack

| Layer     | Teknologi                                                           |
| --------- | ------------------------------------------------------------------- |
| Framework | FormSpec (spec-first, declarative YAML)                             |
| Backend   | Go, module `github.com/primadi/formspec`                            |
| Frontend  | React 19 + TypeScript + Vite + shadcn/ui (di-render dari manifest)  |
| Database  | SQLite (dev) / PostgreSQL (produksi)                                |
| Scripting | Starlark (sandboxed, aksi custom + hooks)                           |
| Sidecar   | Node.js/TypeScript (`app/`) — demo polyglot handler `otc-sale.sell` |
| Manifest  | YAML (`apiVersion: formspec.dev/v1`)                                |

## Prinsip Kunci

- **Manifest-first** — seluruh API, UI, permission, dan state machine
  dideklarasikan sebagai YAML; implementasi di-derive oleh engine.
- **Derived by default** — setiap `Entity` otomatis menghasilkan CRUD API,
  Table, Form, dan detail Page; override hanya di titik yang perlu.
- **Permission berbasis resource+action** — `required_permission` dipakai
  di setiap aksi custom, tidak pernah hardcode nama role.
- **Dua surface, satu renderer** — UI di-derive dari manifests via Meta API
  (`/_ui/_meta/ui`); app ini tidak butuh kode frontend sendiri.

## Layout Proyek

```
Clinic-UI-Showcase/
  README.md                    # Matriks coverage fitur renderer → file
  how-to-run.md                # Panduan menjalankan lengkap
  formspec-app.yaml            # Config CLI (dev + Node sidecar + themes)
  app/                         # TypeScript sidecar (demo polyglot)
    src/                       # handler otc_sell.ts dkk.
  clinic_e2e_test.go           # E2E test klinik
  pharmacy_*_e2e_test.go       # E2E test farmasi (OTC & resep)
  docs/                        # Dokumentasi (folder ini)
  spec/
    apps/
      klinik-internal.yaml     # kind: App — staf, menu penuh (clinic+pharmacy)
      klinik-portal.yaml       # kind: App — publik, hanya pendaftaran pasien
    modules/
      clinic/                  # module klinik
        module.yaml            #   menu suggestion (nested 2 level)
        master/                #   polyclinic, doctor, patient
        transaction/           #   visit, payment, medical-record
        reference/setting/     #   setting (config page pattern)
        summary/               #   daily-visit-summary
        config/                #   clinic (kind: Config)
        dashboards/ forms/ tables/ pages/ reports/ wizards/
        kanbans/ timelines/ prints/ themes/
      pharmacy/                # module farmasi
        module.yaml
        master/medicine/       #   obat (derived UI di module kedua)
        transaction/           #   prescription, otc-sale (+ scripts, prints)
```
