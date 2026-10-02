# Admin Surfaces

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** FormSpec Architecture Overview (D-ARCH-1, D-ARCH-2, D-ARCH-3, D-ARCH-17)

> FormSpec memiliki **tiga admin UI** dengan pemilik, lisensi, dan fungsi yang berbeda. Dokumen ini menjelaskan masing-masing secara detail. **Penting:** spec `spec/frontend/01-visual-hierarchy.md` adalah untuk UI aplikasi bisnis, **bukan** untuk admin UI yang dijelaskan di sini.

---

## 1. Overview

```
┌──────────────────────────────────────────────────────────────┐
│                      Tiga Admin UI                            │
│                                                               │
│  ┌─────────────────┐   ┌─────────────────┐   ┌────────────┐ │
│  │   formspec/ops     │   │  formspec/console  │   │  /_admin   │ │
│  │                 │   │                 │   │            │ │
│  │ Pemilik:        │   │ Pemilik:        │   │ Pemilik:   │ │
│  │ Cloud Owner     │   │ Workspace Owner │   │ App Owner  │ │
│  │                 │   │                 │   │ (end-user) │ │
│  │ Lisensi:        │   │ Lisensi:        │   │            │ │
│  │ Closed source   │   │ Closed source   │   │ Lisensi:   │ │
│  │                 │   │                 │   │ Open source│ │
│  │ Fungsi:         │   │ Fungsi:         │   │            │ │
│  │ Governance &    │   │ Workspace,      │   │ Fungsi:    │ │
│  │ infrastructure  │   │ billing, users, │   │ Business   │ │
│  │ management      │   │ modules         │   │ operations │ │
│  └─────────────────┘   └─────────────────┘   └────────────┘ │
│         ↑                     ↑                    ↑         │
│    Platform Operator     Workspace Owner      Staff Operasional│
│    (Cloud Owner)         (Pemilik Bisnis)     (Kasir, Admin)  │
└──────────────────────────────────────────────────────────────┘
```

---

## 2. formspec/ops — Control Plane Admin

**Audience:** Cloud Owner / Platform Operator — tim infra yang menjalankan FormSpec Cloud.

**Akses:** `formspec/ops.{region}.formspec.dev`

**Lisensi:** Closed source, first-party FormSpec.

### 2.1 Features

| Kategori                   | Fitur                                                                                                                                 |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| **Environment**            | CRUD environment (name, mode dev/prod, tier, resource pool)                                                                           |
| **Node Management**        | Lihat semua K8s node terdaftar, approve/reject pending node, lihat status (active/dead/draining), label & tag node (`formspec.dev/*`) |
| **Cluster Management**     | Lihat semua K8s cluster dalam region, lihat kapasitas (workspace count, CPU/mem usage), health status per cluster                     |
| **ClusterClass**           | Definisikan ClusterClass (premium/standard/economy) — SLA, spesifikasi, harga, fitur, maxWorkspaces                                   |
| **Datastore**              | Lihat semua datastore terdaftar (DB, Valkey, Redis), approve/reject pending, lihat tenant affinity, usage metrics                     |
| **Policy (OPA/Rego)**      | Editor policy deployment, approval rules, trust tier configuration, blocked rules (non-configurable floor)                            |
| **Keys & Signing**         | Key rotation schedule, HSM/KMS status, signing certificate validity, key compromise response                                          |
| **Transparency Log**       | View Merkle tree, verify checkpoint, publish checkpoint to third parties, verify log integrity                                        |
| **Workspace Provisioning** | Provision workspace baru, assign region + ClusterClass, suspend/deactivate workspace                                                  |
| **Emergency Controls**     | Freeze workspace, revoke all sessions, rotate keys, emergency deploy rollback                                                         |
| **Audit Dashboard**        | Full access transparency log, consent history, impersonation grants, evidence trail                                                   |
| **Billing (Operator)**     | Marketplace settlement, metering verification, platform fee collection                                                                |
| **Insiden Management**     | Active incidents, break-glass access queue, post-mortem log                                                                           |

### 2.2 Dogfooding

`formspec/ops` adalah **aplikasi FormSpec yang dibangun dengan FormSpec sendiri**. Ia berjalan di dedicated workspace di Resource Plane operations dan mengakses Control Plane melalui bridge `kind: Service`.

**Bootstrap (chicken-and-egg):** approve cluster/resource membutuhkan formspec/ops, tapi formspec/ops sendiri berjalan di atas cluster. Solusinya: region baru di-bootstrap via **CLI admin** — Cloud Owner men-seed key pertama dan meng-approve cluster + workspace operations pertama langsung lewat API `formspec-ctl --mode=region` (CLI, tanpa UI). Setelah formspec/ops ter-deploy di workspace operations tersebut, seluruh approval berikutnya berjalan lewat UI. Bootstrap path ini adalah operasi one-time per region dan tercatat di transparency log seperti approval lainnya.

---

## 3. formspec/console — Resource Plane Admin (Workspace Owner)

**Audience:** Workspace Owner — pemilik bisnis (bengkel, klinik, enterprise) yang menggunakan aplikasi di atas FormSpec.

**Akses:** `console.{region}.formspec.dev`

**Lisensi:** Closed source, first-party FormSpec.

### 3.1 Features

| Kategori                     | Fitur                                                                                                                                                      |
| ---------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Workspace Dashboard**      | Overview: resource usage, active users, app status, recent activity                                                                                        |
| **Billing & Subscription**   | Pilih tier, lihat tagihan, riwayat pembayaran, top-up prepaid, budget caps                                                                                 |
| **App & Module Management**  | Install/uninstall app dan module dari marketplace, lihat modul aktif + Verified Badge status, review & approve permission footprint (consent) saat install |
| **User & Role Management**   | Kelola workspace users, assign roles per app, invitation & removal                                                                                         |
| **Datasource Configuration** | Install dan konfigurasi datasource: entity-store, kv-store, Postgres, Valkey                                                                               |
| **Grants Management**        | Approve/reject cross-app grant requests, revoke grants, grant history                                                                                      |
| **Backup & Restore**         | Schedule backup, set retention, external target (S3), restore (requires owner signature)                                                                   |
| **Logs & Audit**             | Workspace audit log, consent history, grant history, access log                                                                                            |
| **Data Export**              | Export data — guaranteed never license-gated                                                                                                               |
| **ClusterClass Selection**   | Pilih ClusterClass + region untuk workspace (premium/standard/economy), lihat perbandingan SLA & harga                                                     |

### 3.2 Bukan Pengganti Admin Panel Bisnis

> **Penting:** `formspec/console` **bukan** pengganti admin panel bisnis (`/_admin`). Workspace Owner menggunakan **dua** antarmuka berbeda:
>
> - **formspec/console** — untuk hal-hal level "hosting/langganan" (billing, user management, backup)
> - **Admin panel bisnis** (`/_admin`) — untuk operasional harian (input data, lihat laporan, proses transaksi)
>
> Analogi: formspec/console adalah **cPanel**, admin panel bisnis adalah **aplikasi WordPress** yang di-hosting di atasnya.

### 3.3 Dogfooding

Sama seperti `formspec/ops`, `formspec/console` adalah aplikasi FormSpec yang dibangun dengan FormSpec sendiri.

---

## 4. Business Admin Panel (`_admin`) — **dipensiunkan**

> **Status (2026-10-02): permukaan panel admin derived `/_admin` DICABUT.**
> Login adalah per-App dan administrasi bisnis dilakukan **di dalam App** yang
> bersangkutan, memakai permukaan App yang sudah di-scope permision — bukan
> lewat bundle unscoped bergerbang biner `_admin.access`
> (`docs_internal/plan/app-scoped-login.md` D4/D6). Alasannya terukur: satu
> permission biner itu membuka **seluruh** module beserta entity-nya (kafe
> **5.22.7**), dan role `app-owner` (wildcard `"*"`) otomatis memegangnya.
>
> Yang **tetap ada** di path `/_admin/*` hanyalah **rute framework**, bukan
> panel: first-run setup (`/{ws}/_admin/setup`), OAuth callback/link, dan
> change-password. Itu bootstrap yang harus tetap dapat dijangkau saat
> workspace belum punya App yang bisa dipakai.
>
> Yang menggantikan fitur di §4.1: **derivasi di dalam App** — setiap App
> memasang entity yang relevan lewat `App.spec.modules` + menu/`registered_views`,
> dan Table/Form/Detail derived tetap ada per entity di App itu. Tidak ada lagi
> permukaan lintas-module tanpa scope App.

**Audience:** Staf operasional Workspace Owner (kasir, resepsionis, admin) dan App Owner.

**Akses:** sebelumnya `{workspace}.formspec.dev/_admin`; kini **tidak ada**.

### 4.1 Features (Auto-Generated) — pindah ke dalam App

| Fitur                         | Sumber                                                                  |
| ----------------------------- | ----------------------------------------------------------------------- |
| **CRUD Table** per Document   | Derived by default — tetap ada, di dalam App (bukan lintas module)      |
| **Create/Edit Forms**         | Derived by default                                                      |
| **Detail Page**               | Derived by default                                                      |
| **Menu entries** per module   | `App.spec.menu` — curated per App, bukan di-generate dari daftar entity |
| **Permission-driven UI**      | `permissions` per-entity berlaku penuh (dulu diabaikan di `_admin`)     |
| **State machine transitions** | Action buttons sesuai state saat ini                                    |
| **Realtime updates**          | WebSocket subscription via `ctx.pubsub`                                 |

### 4.2 Override dengan Frontend Kinds

Di dalam App, default bisa di-override dengan UI kinds dari
`docs/spec/frontend/06-page-kinds.md` dan `docs/spec/frontend/07-component-kinds.md`:

- `kind: Form` — custom form layout
- `kind: Table` — custom column, filter, sort
- `kind: Page` — custom page composition
- `kind: Dashboard` — widget canvas
- Navigasi custom lewat `App.spec.menu`/`Module.spec.menu` — tidak ada
  `kind: Menu` standalone, lihat `docs/spec/platform/02-workspace-app-module.md` §4

### 4.3 Escape Hatch: Asset Components

~20% UI yang tidak bisa di-pattern-kan menggunakan `asset` custom component (JS/TS).

### 4.4 Bukan Milik FormSpec

Admin panel bisnis **bukan** aplikasi FormSpec. FormSpec hanya menyediakan mesin generatornya. Data, UI, dan logic adalah milik App Owner/Workspace Owner sepenuhnya.

---

## 5. Permissions & Access Control

| User                              | formspec/ops     | formspec/console                        | App (permukaan)        |
| --------------------------------- | ---------------- | --------------------------------------- | ---------------------- |
| **Cloud Owner**                   | ✅ Full access   | ❌                                      | ❌                     |
| **Cloud Owner Admin** (delegated) | ✅ Scoped access | ❌                                      | ❌                     |
| **Workspace Owner**               | ❌               | ✅ Own workspace                        | ✅ Sesuai grant        |
| **Workspace Admin** (delegated)   | ❌               | ✅ Scoped                               | ✅ Scoped              |
| **App Owner**                     | ❌               | 🔍 Read-only (metrics apps mereka saja) | ✅ Apps mereka         |
| **Staff Operasional**             | ❌               | ❌                                      | ✅ Role-based, per App |

---

## 6. Future: formspec/studio (Roadmap)

**formspec/studio** adalah UI low-code untuk App Owner — direncanakan sebagai alat bantu:

- Natural language → draft resource YAML (AI-assisted)
- Commit ke git dari GUI
- Preview visual sebelum `formspec apply`
- Template vertikal siap pakai

Status: roadmap, bukan bagian dari spec saat ini.

---

## 7. References

| Dokumen                                                        | Isi                                 |
| -------------------------------------------------------------- | ----------------------------------- |
| `docs/spec/platform/01-overview.md` §4                         | Persona + tier developer            |
| `docs/spec/platform/04-control-plane.md` §4                    | Empat peran owner + tools           |
| `docs/spec/frontend/06-page-kinds.md`, `07-component-kinds.md` | Spec UI kinds untuk aplikasi bisnis |
| `docs/architecture/01-architecture-overview.md`                | Multi-region topology               |
