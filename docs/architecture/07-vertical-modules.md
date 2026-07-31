# 07. Vertical Modules — ERP Module Division

**Status:** Draft
**License:** Creative Commons CC0

> This document explains how Forma divides real business verticals (inventory, accounting, billing,
> and the connectors between them) into independent Apps under [`verticals/`](../../verticals/),
> why that shape was chosen, what it borrows from mature ERPs like ERPNext, and — in the spirit of
> `examples/SPEC-COMPATIBILITY-NOTES.md` — where today's spec and engine fall short of what a real
> multi-vertical ERP needs. Like the rest of `docs/architecture/`, this is descriptive, not
> normative; normative spec stays in `docs/spec/`.

## 1. Why `verticals/`, not `examples/`

`examples/Customer`, `examples/General-Ledger`, `examples/Inventory`, and `examples/Order-to-Cash`
were real, independently-installable Apps — `examples/README.md` said so explicitly ("General
Ledger dan Inventory adalah App standalone — bisa di-install ke workspace client secara
independen"). But `examples/` itself is framed as a spec-conformance / DX test-drive corpus, not a
product catalog. Those four have moved to [`verticals/`](../../verticals/README.md); what's left in
`examples/` (`Clinic-UI-Showcase`, `Midtrans-Payment-Gateway`, `reference-app`) are genuine
conformance fixtures, not verticals.

The name `verticals/` matches vocabulary already used in `docs/spec/platform/01-overview.md`: "Vertical
modules (accounting, HRM, inventory) as first-class ecosystem citizens." Not `erp/` (Forma is a
general business-app platform — non-ERP verticals like Clinic could graduate here too, someday) and
not `products/` (doesn't carry the established term).

## 2. Workspace → App → Module → Resource

Forma's tenancy model is singular and explicit: `Workspace → App → Module → Resource`
(`docs/spec/platform/02-workspace-app-module.md` §1). Installing multiple Apps into one Workspace is the *normal*
case, not an edge case — it unifies tenant identity, the basis for cross-app grants
(same document, §1 and §3).

- **App** (§4.4): "Root project manifest. Unit of deployment, trust boundary, and interface
  publication." Declares `modules`, `publishes` (interfaces offered), `consumes` (interfaces
  needed → triggers grant requests). Default private.
- **Module** (§4.5): "Package of manifests — identity, version, dependencies only." No tenant/data
  of its own. `metadata.name` is the permission namespace.
- **Cross-module** (same App) = install consent. **Cross-app** = a signed, revocable **grant**
  approved by the provider's Data Owner (§15.3; D35's "Symmetry" note). Two different tiers of the
  same problem, by design — not an accident of two unrelated mechanisms.

### Each vertical is its own App, not a merged tree

An earlier draft of this reorg tried to physically merge `Customer`/`General-Ledger`/`Inventory`/
`billing` into one composed App, reasoning that the dev-mode loader (`resource.New(cfg)`) only
reads one `SpecPath` (`resource/forma.go`, a single `string` field, no multi-root support). That
was wrong: nothing in the addressing scheme is App-scoped. `internal/entity/registry.go:30` keys
specs by `"module/name"` only, and every vertical already boots standalone today
(`go run ./examples/reference-app --spec ./verticals/inventory/spec` works with zero cross-app
dependencies). So: **each vertical is its own independent `kind: App`** — a developer can take just
`gl`, or just `inventory`, and build everything else themselves. Composing several together for a
demo (see §5) is a separate concern from any one vertical's own independence.

## 3. Module taxonomy for this ERP surface

| Kind | Examples | Owns entities? |
|---|---|---|
| **Vertical App** | `company`, `billing`, `inventory`, `gl` | Yes — the domain's own Documents |
| **Integrator App** | `sales-inventory-integrator`, `sales-gl-integrator` (future: `purchase-inventory-integrator`, `inventory-gl-integrator`) | No — Subscription + script only |

Integrator apps exist so cross-vertical reactions aren't buried inside either side of the
relationship. Before this reorg, `examples/Inventory` reached into `billing.order` via its own
`order-to-movement` Subscription, and `examples/General-Ledger` did the same via
`order-to-journal` — both misplaced by the principle this document is now stating. Both were
extracted into their own apps. A workspace can install `billing` + `inventory` with no integrator
at all, or swap in a different vendor's connector — the integrator is optional, not baked in.

Contrast with ERPNext: its Stock module posts directly to GL via `StockController.make_gl_entries()`
and a warehouse→account map — correct, battle-tested, but tightly coupled inside one codebase.
Forma's `kind: Subscription` (D35, §12.5) already supports decoupling this; the reorg's job was
just to stop burying the decoupled version inside the wrong module.

**A caveat surfaced during this reorg, not resolved by it:** `billing.order`'s `paid` event has
*two* integration mechanisms live simultaneously — an inline `deliver: reliable_event` targeting
`gl.journal-entry` directly (with its own retry/dead-letter/idempotency), **and** a separate
`sales-gl-integrator` Subscription achieving roughly the same outcome via a queued job. Both
predate this reorg; neither was invented by it. Which pattern should be preferred for *future*
integrations is an open question this document doesn't resolve — the inline `reliable_event` is
simpler for a single fixed consumer; the separate integrator app is more optional/swappable. See
`verticals/billing/spec/README.md` for exactly where this shows up.

## 4. ERPNext comparison

| ERPNext module | Forma vertical here | Notes |
|---|---|---|
| Stock | `inventory` | ERPNext: warehouses, valuation (FIFO/LIFO/moving-avg/standard). Forma: simpler today, no valuation method yet. |
| Accounts | `gl` | Both double-entry; ERPNext's GL Entry auto-posts from Stock/Sales/Purchase controllers directly. |
| Selling / part of Accounts Receivable | `billing` | ERPNext splits Selling (quotation→sales order) from Accounts (invoice); Forma's `billing` currently spans order+payment in one module. |
| Buying | *(not built — deferred, see §7)* | ERPNext: purchase order → purchase receipt → landed cost. |
| — | `company` | Net-new; see §5. |

**Branch/multi-location, ERPNext's known pain point:** the Frappe community forum has a
long-running thread literally titled "Multicompany or branch or cost center - confused" — teams end
up choosing between a full separate **Company** (heavy, separate books), a **Cost Center**
(accounting-only, doesn't scope stock), or a **Warehouse** (stock-only, doesn't scope sales/HR),
because ERPNext has no first-class Branch construct (its only "Branch" doctype is HR-scoped,
employee assignment only). Forma avoids this by making `company.branch` one first-class master
entity from the start, referenced explicitly by whichever entities need it (see §5) — not
overloaded onto Warehouse or Cost Center.

## 5. Branch model

Branch is a **business concept**, not a framework one. Evidence: `internal/db/ddl.go` auto-injects
only `tenant_id` (plus version/timestamps/soft-delete) into every table — no `branch_id`/
`company_id`/`org_unit_id` anywhere. `pkg/spec/entity.go`'s field-type enum has no tree/hierarchical
type; relations are only `belongs_to | has_many | has_one`. So branch is modeled exactly like
`warehouse` already is: a plain master entity (`company.branch` — `code`, `name`, optional
self-referencing `parent_id` for a hierarchy, `is_active`) plus explicit `branch_id belongs_to
company.branch` relation fields on whichever entities need branch scoping. First (and, in this
pass, only) consumer: `inventory.warehouse`.

Named `company`, not `core` — every `module.yaml` already declares `depends: [{module: forma/core}]`
for the *framework's* core module; a second, unrelated "core" would collide in name only, but would
still confuse readers.

**Not multi-tenancy.** `tenant_id` remains the sole framework isolation boundary (one workspace =
one tenant, per the user's own framing). Branch is a dimension *inside* one tenant.

**Should `branch_id` be framework-recognized, like `tenant_id`?** Not yet, deliberately.
`pkg/spec/entity.go` already has a `TenantDecl{Isolated bool}` struct sitting on `EntitySpec.Tenant`
that looks exactly like what a framework-level branch flag would need — but it has **zero
consumers** anywhere in `internal/db` or `internal/entity` (confirmed by grep). It's an aspirational
field the engine never wired up, the same class of gap as the natural-key `scope` parameter before
this reorg (see §6). Building real framework-level branch scoping now (auto-injected column,
automatic row filtering, permission integration) would be invasive engine surgery for a guarantee
nothing currently needs — there is no permission/auth enforcement at all yet to hook it into. So
`branch_id` ships as an ordinary relation field, standardized purely as a **naming convention**
(always that name, always `belongs_to company.branch`) so that (a) the natural-key `scope_field`
mechanism below can read it generically, and (b) a future framework-level mechanism has one
consistent field to adopt — most naturally by finally wiring up the dormant `TenantDecl`, rather
than inventing a second, parallel mechanism.

## 6. Natural-key `scope_field`

The natural-key counter table was already keyed by `(tenant_id, resource, field, scope, period)`
(`internal/db/counter.go`) — `scope` a free-form string — but nothing in the YAML spec could set it;
both call sites hardcoded `""`. Fixed as a small, additive, backward-compatible change:

- `pkg/spec/entity.go`'s `NaturalKeyRuleDecl` gained `ScopeField string` (`scope_field` in YAML) —
  names a field on the same entity (e.g. `branch_id`) whose value becomes the counter's scope.
- `internal/db/crud.go`'s automatic natural-key generation on `create` now resolves it from the
  row's own data when declared; unset reproduces prior behavior exactly.
- **Known limitation, not fixed here:** `internal/entity/registry.go`'s `GenerateNaturalKey` (the
  backing for an explicit `ctx.next_key(field)` call from a script) has no resource data in scope to
  resolve `scope_field` from — it only ever sees `tenantID, module, name, fieldName`. Wiring that
  path the same way would mean threading resource data through `internal/action/dispatcher.go`,
  `internal/starlark/executor.go`, and `resource/forma.go`'s `NextKeyHandler` chain — out of scope
  for this pass. `ctx.next_key()` always uses the tenant-wide scope regardless of `scope_field`
  today; the automatic on-create path (what document numbering actually needs) is what's fixed.
- Documented in `docs/spec/backend/01-core-basic.md` §2, alongside `strategy`/`format`/`prefix`/`reset`.

## 7. UI ownership: vertical vs. `reference-app`

Each vertical **keeps its own existing frontend artifacts** — `inventory` already ships
`menus/tables/reports/widgets`; `billing` already ships `forms/pages/tables/widgets/menus`. This is
what makes a vertical usable when installed standalone; it is not "backend-only." Where a module
has no explicit `Form`/`Page`/`Table` for an entity, the frontend auto-derives one (the D17
"Derived by Default" pattern demonstrated in `examples/Clinic-UI-Showcase`) — that fallback already
applies *within* a vertical, same as anywhere else.

`verticals/reference-app/` adds exactly one thing no single vertical owns:
`spec/dashboards/erp-overview.yaml`, a small dashboard combining billing's revenue widget with
inventory's low-stock widget — proof that cross-module UI composition works end-to-end, nothing
more. No new wizards were added; those are deferred with the rest of §8's roadmap.

## 8. Gaps this exercise surfaced

Mirroring `examples/SPEC-COMPATIBILITY-NOTES.md`'s format — gap, evidence, status:

| Gap | Evidence | Status |
|---|---|---|
| No branch/multi-location construct | No `branch_id` in `ddl.go`; no tree field type in `pkg/spec/entity.go` | Modeled as a plain master entity + convention (§5); only viable path given the spec today |
| `App.consumes`/`publishes` only demonstrated for `kind: Service` | `spec/platform/02-workspace-app-module.md` §3's only example is `service: icd-lookup` | This reorg approximates it for plain entity events (`billing.order.paid`) with an invented `service:` name in each `consumes`/`publishes` block — now spec-blessed: `AppSpec.Publishes`/`Consumes` are structured `{service, actions}` / `{app, service, actions}` (`pkg/spec/resources.go`) |
| Cross-app grant enforcement unimplemented | No App-scoped grant-checking anywhere in `internal/permission`/`internal/action` | Spec'd (D25, §15.3), zero runtime implementation — not attempted here |
| SyncAgent registry sync not wired to the live HTTP router | `docs/runtimes/02-forma-resource.md:138` | A real multi-App workspace can *accept* manifests via `forma apply` per App but can't yet *serve* them together end-to-end; `reference-app` uses a dev-mode filesystem-aggregation workaround instead (§9), same shortcut every pre-existing example already used |
| `impl: compiled` (WASM/Go plugin) unimplemented | `pkg/spec/spec.go:96` is a bare string constant; no executor registered in `internal/action` | Spec'd (`spec/backend/01-core-basic.md` §5), not built — noted for future closed-source vendor modules (see §10) |
| `EntitySpec.Tenant *TenantDecl` unwired | Zero consumers in `internal/db`/`internal/entity` | Same class of "spec got ahead of the engine" gap as the natural-key `scope` parameter was, before §6's fix |
| `natural_key_rule.scope_field` doesn't reach `ctx.next_key()` | See §6 | Automatic on-create path fixed; explicit script path is a known follow-up |
| Dashboard `widget.ref` is matched by bare `metadata.name`, not module-qualified | `internal/ui/registry.go`'s `Widgets` map is keyed by name only (`registerInto`, `raw.Metadata.Name`) | Confirmed while building `reference-app/spec/dashboards/erp-overview.yaml` — a cross-module dashboard must use each widget's bare name (`today-revenue`, not `billing.today-revenue`); harmless here since names are unique across the composed set, but two modules reusing the same widget name would silently collide |

## 9. `reference-app` — dev-mode composition, not the production path

See [`verticals/reference-app/README.md`](../../verticals/reference-app/README.md) for usage.
Summary: `compose.sh` copies each vertical's `spec/modules/<name>/` (plus, for `inventory`/`gl`,
their App-level UI folders, namespaced per-app to avoid collisions like both shipping their own
`config/app.yaml`) into one aggregated tree, loaded via the ordinary single-`SpecPath` dev loader.
This is explicitly the same shortcut `spec/platform/02-workspace-app-module.md` §6 calls
"non-conformant" for direct filesystem loading — tolerated here because the *conformant* path (per-App `forma apply` into one
workspace) can't yet serve a converged result end-to-end (§8's SyncAgent gap). When that's fixed,
`reference-app` should be replaced by real per-App `forma apply` calls against one workspace.

## 10. Roadmap (explicitly deferred)

Not built in this pass:

- A real `purchase` vertical (purchase order → goods receipt).
- `purchase-inventory-integrator`, `inventory-gl-integrator`.
- The Inventory features that originally motivated this reorg: stock opname (physical count),
  a `movement-type` master-data entity for transaction categorization, adjustment handling, and
  fixing `stock-movement`'s `transfer` type (today `movement_apply.star`/`stock_level_update.star`
  only branch on `in`/`out` — a `transfer` movement is accepted by the state machine but never
  actually moves stock between warehouses).
- Cross-app grant enforcement, and wiring SyncAgent to the live router (§8) — both real engine
  work, not module-authoring work.
- Closed-source 3rd-party vertical modules: the mechanism already exists (`impl: native`, compiled
  into a vendor's own binary with source never distributed; or `impl: sidecar`, a separate
  container/process over Unix socket — SDKs exist for PHP/Python/TypeScript in `sdk/`) — YAML/
  Starlark manifests stay open regardless ("readability is a feature",
  `docs/spec/platform/07-marketplace.md` §2). Not relevant to this repo's own reference verticals, which
  should stay `script_ref` throughout, but worth knowing for whoever builds the next one commercially.
