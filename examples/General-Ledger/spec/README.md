# General Ledger — Spec

**Klasifikasi:** **App** standalone. Bisa di-install ke workspace client, atau di-compose sebagai module oleh App lain (misal O2C `tokoku`).
**Spec target:** Forma Core Basic v0.2.0.

## Struktur

```
General-Ledger/
├── spec/
│   ├── README.md
│   ├── forma.yaml                                    # kind: App "general-ledger"
│   ├── modules/
│   │   └── general-ledger/
│   │       ├── module.yaml                           # kind: Module
│   │       ├── entities/
│   │       │   ├── account.yaml                      # [reference] — chart of accounts
│   │       │   ├── journal-entry.yaml                # [transaction] — double-entry journal
│   │       │   └── gl-balance.yaml                   # [summary] — saldo per akun
│   │       ├── subscriptions/
│   │       │   └── order-to-journal.yaml             # kind: Subscription → order.paid
│   │       ├── scripts/
│   │       │   ├── journal_post.star
│   │       │   ├── journal_reverse.star
│   │       │   └── gl_balance_update.star
│   │       └── config/
│   │           └── gl.yaml                           # kind: Config
│   └── config/
│       └── app.yaml
│
└── impl/
    └── general-ledger/
        └── create_sales_journal.go                   # native Go handler untuk Subscription job
```

## App Identity

- **Name:** `general-ledger`
- **Vendor:** `forma-dev`
- **Modules:** `general-ledger` (self-titled module)
- **Permission namespace:** `general-ledger.*` (contoh: `general-ledger.journal-entries.post`)

## Konsep yang di-cover

| Konsep | Lokasi | Spec Source |
|---|---|---|
| `kind: App` — standalone deployable | forma.yaml | Core Basic §4.4 |
| `characteristics: [reference]` — seeded, read-only | account | Core Basic §4.1, §9.1 |
| `characteristics: [transaction]` — append-heavy | journal-entry | Core Basic §9.1 |
| `characteristics: [summary]` — system-managed | gl-balance | Core Basic §9.1 |
| `natural_key_rule: { strategy: sequence }` | journal-entry.number | Core Basic §10.4 |
| `child: { storage: table }` — large child data | journal-entry.lines | Core Basic §10.3 |
| Double-entry validation via `guard` | journal-entry state_machine | Core Basic §14 |
| `deliver.reliable_event` — tak boleh hilang | journal-entry events | Core Basic §12.3 |
| Cross-module `kind: Subscription` | order-to-journal.yaml | Core Basic §12.5/D35 |
| `idempotent: true` + `idempotency_key: { from: server }` | journal-entry.post | Core Basic §11.8/D32 |

## Relasi dengan Example Lain

| Example | Hubungan |
|---|---|
| **Order-to-Cash** | `order.paid` → reliable_event → `gl.journal-entry.create`; Subscription `order-to-journal` |
| **Inventory** | Tidak langsung — GL bisa menerima journal dari modul lain via Subscription |

## impl/

Satu Go stub: `impl/general-ledger/create_sales_journal.go` — handler untuk Subscription job `create-sales-journal`.

## Catatan: GL sebagai Module vs App

GL bisa di-deploy dalam dua mode:
1. **Standalone App** (ditunjukkan di sini) — punya `forma.yaml`, di-install ke workspace client.
2. **Module di App lain** — `forma.yaml` dihapus, `modules/general-ledger/` di-copy ke App target (seperti di O2C).
