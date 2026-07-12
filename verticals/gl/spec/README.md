# gl (General Ledger) — Spec

**Klasifikasi:** **App** standalone, independently installable — see [`docs/architecture/07-vertical-modules.md`](../../../docs/architecture/07-vertical-modules.md).
**Spec target:** Forma Core Basic v0.2.0.

> Formerly `examples/General-Ledger`, module name `general-ledger`. Moved to `verticals/gl` and the module renamed to `gl` (matching the `gl.journal-entry` references already used by `billing.order`'s event target and by `internal/manifest/examples_roundtrip_test.go`). The `order-to-journal` Subscription that used to live here was extracted to its own app, `verticals/sales-gl-integrator` — gl no longer reaches into `billing` itself.

## Struktur

```
verticals/gl/
├── spec/
│   ├── README.md
│   ├── forma.yaml                                    # kind: App "gl", publishes: journal-entries
│   ├── menus/, widgets/, reports/, tables/, dashboards/  # App-level UI
│   ├── modules/
│   │   └── gl/
│   │       ├── module.yaml                           # kind: Module
│   │       ├── entities/
│   │       │   ├── account.yaml                      # [reference] — chart of accounts
│   │       │   ├── journal-entry.yaml                 # [transaction] — double-entry journal
│   │       │   └── gl-balance.yaml                    # [summary] — saldo per akun
│   │       ├── scripts/
│   │       │   ├── journal_post.star
│   │       │   ├── journal_reverse.star
│   │       │   └── gl_balance_update.star
│   │       └── config/
│   │           └── gl.yaml                           # kind: Config
│   └── config/
│       └── app.yaml
│
└── impl/                                             # (none currently — the one Go stub moved to
                                                        #  sales-gl-integrator/impl/ with the subscription)
```

## App Identity

- **Name:** `gl`
- **Vendor:** `forma-dev`
- **Modules:** `gl`
- **Permission namespace:** `gl.*` (e.g. `gl.journal-entries.post`)
- **Publishes:** `journal-entries` service (`create`, `post`) — consumed by `sales-gl-integrator`

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
| `idempotent: true` + `idempotency_key: { from: server }` | journal-entry.post | Core Basic §11.8/D32 |

## Relasi dengan vertical lain

gl does not depend on any other vertical. `billing.order`'s `paid` event targets `gl.journal-entry` directly via `deliver.reliable_event` (see `verticals/billing/spec/modules/billing/entities/order.yaml`) — that's an existing direct cross-app reference, distinct from the `sales-gl-integrator` app, which additionally creates the sales journal entry itself via its own Subscription (`order-to-journal`) + job handler. See `docs/architecture/07-vertical-modules.md` for why both patterns currently coexist and which is preferred going forward.
