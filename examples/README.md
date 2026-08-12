# FormSpec Examples

Kumpulan contoh yang dibangun dengan **FormSpec Core Basic v0.2.0** + **Core Extended stub** — spec-conformance dan DX test-drive fixtures, bukan produk vertikal yang nyata.

> **Vertical modules pindah ke [`verticals/`](../verticals/).** `Customer`, `General-Ledger`, `Inventory`, dan `Order-to-Cash` dulu ada di sini, tapi ternyata itu App standalone yang bisa di-install ke workspace client — bukan sekadar demo (lihat `Order-to-Cash/README.md`'s klasifikasi lama: "App standalone"). Empat-empatnya sudah dipindah + direstrukturisasi jadi App independen (`company`, `billing`, `inventory`, `gl`, `notifications`, `sales-inventory-integrator`, `sales-gl-integrator`) di bawah `verticals/`. Lihat [`verticals/README.md`](../verticals/README.md) dan [`docs/architecture/07-vertical-modules.md`](../docs/architecture/07-vertical-modules.md) untuk rasionalnya dan pembagian modul ERP secara keseluruhan.
>
> Yang tetap tinggal di `examples/` adalah fixture konformansi spec/renderer murni: `Clinic-UI-Showcase` (frontend-renderer coverage), `Midtrans-Payment-Gateway` (payment gateway adapter demo), dan `reference-app` (minimal engine-embedding demo).

## Tujuan

1. **Menguji kelengkapan spec** — apakah semua konstruksi yang dibutuhkan real-world app sudah ada di spec?
2. **Menguji DX (Developer Experience)** — apakah struktur mudah dipelajari, di-maintain, dan apakah butuh helper?

## Daftar Examples

| Example | Klasifikasi | Deskripsi | `formspec.yaml` |
|---|---|---|---|
| [Clinic-UI-Showcase](./Clinic-UI-Showcase/) | **App** (`klinik-sehat`) | Coverage penuh 12 kind frontend renderer | ✅ standalone |
| [Cafe](./cafe/) | **App** (`cafe`) | Aplikasi kafe — master data, pesanan (child items), pembayaran, dan laporan. Dibangun lewat alur agent-assisted (lihat [`docs/guides/agent-assisted-app-development.md`](../docs/guides/agent-assisted-app-development.md)) | ✅ standalone |
| [Midtrans Payment Gateway](./Midtrans-Payment-Gateway/) | **Module** (`billing`) | Service wrapper payment gateway + mockup + webhook | ❌ (murni module) |
| [reference-app](./reference-app/) | Go embedding demo | Minimal `main.go` untuk embed `resource.New(cfg)` | n/a |

Vertical modules yang tadinya di sini: lihat tabel di [`verticals/README.md`](../verticals/README.md).

## Pembagian Module vs App

| Konsep | Definisi | Ciri |
|---|---|---|
| **App** | Unit deployment & trust boundary | Punya `formspec.yaml`, di-install ke Workspace |
| **Module** | Package of manifests | `module.yaml` saja, di-compose oleh App |

## Struktur Setiap Example

```
<nama>/
├── spec/                   ← kontrak deklaratif (selalu di-deploy)
│   ├── [formspec.yaml]        ← hanya untuk App
│   ├── modules/
│   │   └── <module>/
│   │       ├── module.yaml
│   │       ├── entities/   ← kind: Entity
│   │       ├── services/   ← kind: Service
│   │       ├── mockups/    ← kind: Mockup (Extended)
│   │       ├── webhooks/   ← kind: Webhook (Extended)
│   │       ├── subscriptions/ ← kind: Subscription
│   │       ├── scripts/    ← .star (Starlark, hot-updatable)
│   │       └── config/     ← kind: Config
│   └── config/
│
├── impl/                   ← Go source (build-time only, TIDAK di-deploy)
│   └── <module>/
│       └── *.go
│
└── README.md
```

**Aturan main:**
- `spec/` — committed, selalu di-deploy. Berisi YAML + `.star` scripts.
- `impl/` — committed, build-time only. Dihapus saat deploy; hasil kompilasinya sudah di-fuse ke binary.
- `.formspec/` — git-ignored, auto-generated. Hasil kompilasi `impl/` (`.formspec/build/`) + cache.

## Spec yang Diuji

| Spec | Kind yang dipakai |
|---|---|
| **Core Basic v0.2.0** | `Entity`, `Service`, `Module`, `App`, `Config`, `Subscription` |
| **Core Extended stub** | `Mockup`, `Webhook` + environment binding |

## Coverage Matrix

**Historical** — dari `SPEC-COMPATIBILITY-NOTES.md`'s test-drive (2026-07-03), dari saat Customer/GL/Inventory/O2C masih di sini. Kolom-kolom itu sekarang adalah `billing`/`gl`/`inventory` di `verticals/`; tabel ini dibiarkan sebagai catatan konformansi spec, bukan diperbarui per lokasi baru.

| Konsep | Customer | Midtrans | GL | Inventory | O2C |
|---|---|---|---|---|---|
| `characteristics: [master]` | ✅ | — | — | ✅ | ✅ |
| `characteristics: [transaction]` | — | — | ✅ | ✅ | ✅ |
| `characteristics: [reference]` | — | — | ✅ | — | — |
| `characteristics: [summary]` | — | — | ✅ | ✅ | ✅ |
| `natural_key_rule` | — | — | ✅ | ✅ | ✅ |
| `child: { storage: jsonb }` | — | — | — | — | ✅ |
| `child: { storage: table }` | — | — | ✅ | ✅ | — |
| `state_machine` | — | — | ✅ | ✅ | ✅ |
| `guard` condition | — | — | ✅ | ✅ | ✅ |
| `conditions` on action | — | — | ✅ | ✅ | ✅ |
| `required_permission` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `audit: true` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `uses.resources` | — | ✅ | — | — | ✅ |
| `uses.config` | — | ✅ | — | — | ✅ |
| `uses.primitives` | — | — | ✅ | ✅ | ✅ |
| `idempotent: true` | — | ✅ | ✅ | ✅ | ✅ |
| `publish.durable` | — | — | ✅ | ✅ | ✅ |
| `deliver.reliable_event` | — | — | ✅ | ✅ | ✅ |
| `deliver.websocket` | — | — | — | — | ✅ |
| `deliver.queue` | — | — | — | — | ✅ |
| `kind: Service` | — | ✅ | — | — | ✅ |
| `kind: Mockup` | — | ✅ | — | — | ✅ |
| `kind: Webhook` | — | ✅ | — | — | ✅ |
| `kind: Subscription` | — | — | ✅ | ✅ | ✅ |
| `kind: Config` | — | ✅ | ✅ | ✅ | ✅ |
| `ctx.lock` | — | — | ✅ | ✅ | ✅ |
| `ctx.cache` | — | — | — | — | ✅ |
| `ctx.log` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `impl: script_ref` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `impl: native` | — | ✅ | — | — | ✅ |
