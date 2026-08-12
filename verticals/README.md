# FormSpec Verticals

Real, independently-installable ERP vertical Apps — not spec-conformance demos (those stay in
[`examples/`](../examples/)). Full rationale, the App/Workspace composition model, ERPNext
comparison, and the gaps this exercise surfaced: **[`docs/architecture/07-vertical-modules.md`](../docs/architecture/07-vertical-modules.md)**.

## Apps

| App | `kind: App` name | Publishes | Consumes | Description |
|---|---|---|---|---|
| [`company/`](./company/) | `company` | `branch-directory` | — | Shared org structure — branch directory used by other verticals |
| [`billing/`](./billing/) | `billing` | `order-events`* | — | Customer, order, checkout, payment gateway (formerly `examples/Customer` + `examples/Order-to-Cash`'s billing module — see its README) |
| [`inventory/`](./inventory/) | `inventory` | `stock-movements` | `company` | Multi-warehouse stock tracking |
| [`gl/`](./gl/) | `gl` | `journal-entries` | — | Double-entry accounting core (formerly `examples/General-Ledger`) |
| [`notifications/`](./notifications/) | `notifications` | — | `billing`* | WhatsApp notification reactions |
| [`sales-inventory-integrator/`](./sales-inventory-integrator/) | `sales-inventory-integrator` | — | `billing`*, `inventory` | Optional connector: `billing.order.paid` → outbound stock movement |
| [`sales-gl-integrator/`](./sales-gl-integrator/) | `sales-gl-integrator` | — | `billing`*, `gl` | Optional connector: `billing.order.paid` → sales journal entry |
| [`reference-app/`](./reference-app/) | `erp-reference` | — | all of the above | Dev-mode composition of everything above, with one small cross-module dashboard |

\* `consumes`/`publishes` is only spec'd for `kind: Service` interfaces (`docs/spec/02-core-basic.md` §4.4); these apps react to plain entity events (`billing.order`'s `paid` event), which the spec doesn't have a blessed cross-app declaration syntax for yet. See the architecture doc's gap list.

## Each App is independent

Every folder above is a complete, standalone `kind: App` (own `formspec.yaml` + `module.yaml`) —
none require any other vertical to load and validate. Prove it yourself:

```sh
go run ./examples/reference-app --spec ./verticals/company/spec
go run ./examples/reference-app --spec ./verticals/gl/spec
go run ./examples/reference-app --spec ./verticals/inventory/spec
```

Each boots on its own. The two `*-integrator` apps are the only ones that need others
installed alongside them to do anything useful (that's their whole purpose) — a workspace
can run `billing` + `inventory` without `sales-inventory-integrator` at all, or swap in a
different vendor's connector.

## Composing them together

Two ways, with very different maturity levels today:

1. **Production (intended, not yet fully wired):** `formspec apply` each App into one workspace
   (`docs/spec/02-core-basic.md` §6.0's two-stage Registration/Deployment pipeline). This is
   what makes the `publishes`/`consumes` cross-app grant model in §4.4 meaningful. Blocked
   today by a real engine gap: the Resource Plane's SyncAgent registry sync isn't wired to the
   live HTTP router yet (`docs/runtimes/02-formspec-resource.md:138`), so a workspace can *accept*
   multiple Apps' manifests but can't yet *serve* them all through one running API.
2. **Dev convenience (what actually works today):** [`reference-app/`](./reference-app/)'s
   `compose.sh` aggregates every vertical's manifests into one filesystem tree and loads it
   with the single-`SpecPath` dev-mode loader — the same shortcut every pre-existing example
   in this repo already used, and one the spec itself calls "non-conformant" (§6.0) but tolerates
   for exactly this purpose.

## Multi-branch

Branch is a business concept, not a framework one — see the architecture doc §"Branch model"
for why. In short: a plain `company.branch` master entity, referenced by `branch_id belongs_to
company.branch` wherever an entity needs branch scoping (currently just `inventory.warehouse`).
Not multi-tenancy — `tenant_id` remains the only framework-level isolation boundary; one
workspace = one tenant, optionally split into branches internally.
