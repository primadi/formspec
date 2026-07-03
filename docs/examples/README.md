# Forma Examples

Kumpulan contoh aplikasi bisnis yang dibangun dengan **Forma Core Basic v0.2.0** + **Core Extended stub**.

## Tujuan

1. **Menguji kelengkapan spec** — apakah semua konstruksi yang dibutuhkan real-world app sudah ada di spec?
2. **Menguji DX (Developer Experience)** — apakah struktur mudah dipelajari, di-maintain, dan apakah butuh helper?

## Daftar Examples

| Example | Klasifikasi | Deskripsi | `forma.yaml` |
|---|---|---|---|
| [Customer](./Customer/) | **Module** (`billing`) | Data pelanggan & alamat | ❌ (murni module) |
| [Midtrans Payment Gateway](./Midtrans-Payment-Gateway/) | **Module** (`billing`) | Service wrapper payment gateway + mockup + webhook | ❌ (murni module) |
| [General Ledger](./General-Ledger/) | **App** (`general-ledger`) | Double-entry accounting core | ✅ standalone |
| [Inventory](./Inventory/) | **App** (`inventory`) | Multi-warehouse stock tracking | ✅ standalone |
| [Order-to-Cash](./Order-to-Cash/) | **App** (`tokoku`) | Contoh kanonik: order → bayar → jurnal → nota | ✅ (compose 3 module) |

## Pembagian Module vs App

| Konsep | Definisi | Ciri |
|---|---|---|
| **App** | Unit deployment & trust boundary | Punya `forma.yaml`, di-install ke Workspace |
| **Module** | Package of manifests | `module.yaml` saja, di-compose oleh App |

**Customer** dan **Midtrans PG** adalah module di bawah namespace `billing` — mereka tidak berdiri sendiri, melainkan di-compose oleh App `tokoku` (Order-to-Cash).

**General Ledger** dan **Inventory** adalah App standalone — bisa di-install ke workspace client secara independen, atau di-compose sebagai module oleh App lain.

## Struktur Setiap Example

```
<nama>/
├── spec/                   ← kontrak deklaratif (selalu di-deploy)
│   ├── [forma.yaml]        ← hanya untuk App
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
- `.forma/` — git-ignored, auto-generated. Hasil kompilasi `impl/` (`.forma/build/`) + cache.

## Spec yang Diuji

| Spec | Kind yang dipakai |
|---|---|
| **Core Basic v0.2.0** | `Entity`, `Service`, `Module`, `App`, `Config`, `Subscription` |
| **Core Extended stub** | `Mockup`, `Webhook` + environment binding |

## Coverage Matrix

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
