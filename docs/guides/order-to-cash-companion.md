# Order-to-Cash Technical Companion

**Status:** Draft
**Audience:** Developers evaluating FormSpec — technical deep-dive
**Prerequisites:** [FormSpec Overview](../spec/platform/01-overview.md) · [Core Basic Spec](../spec/backend/01-core-basic.md) · [O2C Tutorial](./order-to-cash-tutorial.md)

> This document is the technical companion to the Order-to-Cash tutorial. It compares building the same application with and without FormSpec, catalogs the test-drive findings from writing the spec against real requirements, and maps every requirement to the exact FormSpec construct that handles it.

---

## 1. Scenario A — Without FormSpec (Plain Go/Gin, Three Review Rounds)

What happens when an AI coding assistant builds this in plain Go, with a reviewer catching issues over three rounds:

| Round | Prompt | Result |
|---|---|---|
| **P1** | "Generate sequential order numbers and a webhook handler for payments" | `SELECT MAX(seq)+1` (race condition), goroutine fire-and-forget for journal (lost on restart), no idempotency |
| **P2** | "Fix the race condition" | Advisory lock added — race fixed. But AI *also* replaced the goroutine with Redis Pub/Sub: financial events now go through a non-durable channel (regression) |
| **P3** | "Add idempotency and ensure journal events survive restart" | Looks correct — queue used, idempotency present. Two subtle bugs survive: (1) idempotency key in Redis with 60s TTL — hidden race window; (2) `UPDATE status` and `Enqueue journal` are **two operations without a shared transaction** — order can be "paid" without the journal job ever being sent. *Exactly the requirement that was supposed to be fixed, just in a different form.* |

| Aspect | P1 | P2 | P3 |
|---|---|---|---|
| Race on order numbers | ❌ | ✅ | ✅ |
| Reliable financial events | ❌ | ❌ (regression) | ⚠️ non-atomic |
| Webhook idempotency | ❌ | ❌ | ⚠️ TTL 60s |
| Audit trail | ❌ | ❌ | ❌ (never requested) |

**The core finding:** the AI fixes exactly what is asked — no more. The final quality is bounded by the reviewer's knowledge of bug classes, not by the AI's capability. The number of rounds needed has no clear limit.

---

## 2. Scenario B — With FormSpec Core Basic

The same requirements, built with FormSpec (see the [tutorial](./order-to-cash-tutorial.md) for the full walkthrough). Every requirement maps to a **declared, framework-enforced construct**:

| Requirement | Construct | Why it's different |
|---|---|---|
| FR1 — Sequential numbers | `natural_key_rule` + `ctx.next_key()` | Rule in schema, locking wrapped in helper — never handwritten |
| FR2 — Payment gateway | `kind: Service` | External integrations MUST be wrapped; mockup in dev via config |
| FR3 — Webhook idempotency | `idempotent: true` | Declared in contract; handler stays clean; framework enforces store + response replay |
| FR4 — Reliable journal | `publish.durable` + `deliver.reliable_event` | Mandatory fields in contract — not a decision that can be forgotten |
| FR5 — Email reliability | `deliver channel: queue` (publisher) | Email is a billing promise → in publisher's deliver |
| FR5 — WA reliability | `kind: Subscription` (D35) | Added later without touching order.yaml |
| FR6 — Live ticker | `deliver channel: websocket` | Deliberately lossy — cannot be confused with FR5 |
| FR7 — Cached discount | `ctx.cache` + invalidation | Cache is a declared, tenant-scoped primitive gated by `uses` — not an ad-hoc Redis client with invalidation left to memory |
| FR8 — Config per workspace | `kind: Config` + `ctx.config()` | Workspace-scoped config is first-class — no hand-rolled settings table, no redeploy to change a prefix or template |
| FR9 — Structured logging | `ctx.log` — tenant/request/user auto | Correlation ID and tenant/request/user context are injected by the framework — not dependent on each handler's discipline |
| FR10 — PDF storage | `ctx.storage.write()` | Object storage is the only storage primitive offered — files cannot end up on an ephemeral container filesystem |

**Key differences from Scenario A:**
- Auth is mandatory by default + `required_permission` per action (without auth = impossible, not forgotten)
- Order numbers are auto-per-tenant (counter keyed by tenant — lock is never global across tenants)
- Item/total validation = state machine guard + field rules
- Audit = `audit: true`
- Event loss = impossible *by contract* because `mark-paid` changes status AND writes the outbox in one DB transaction. **Implementation status:** the reference jsonb-persist backend does not yet wrap the outbox write in the mutation's transaction — see [jsonb-persist gap notes](../renderers/jsonb-persist/01-architecture.md) §3; until that lands, a crash between commit and enqueue can drop the event (consistent with the tutorial's FR4 note).

---

## 3. Scenario C — FormSpec + Agent Skill

Agent Skills are rules that constrain AI coding assistants to FormSpec's structural requirements:

- Manifest first, `impl` second
- Every action MUST have `required_permission` + `uses` — missing = reject generation
- Financial events MUST have `durable: true` + `reliable_event`
- Sequential numbers MUST use `natural_key_rule` + `ctx.next_key` — `MAX()+1` = reject
- External integrations MUST be `kind: Service`
- Webhooks MUST have `idempotent: true`

| | A (plain Go) | B (FormSpec) | C (FormSpec + Skill) |
|---|---|---|---|
| Rounds to correct | 3+, no clear limit | 1 (structure enforces) | 1 (AI is guardrailed) |
| Subtle bugs survive | 2 proven | Vulnerability points declared | Same |
| Consistency across developers | Person-dependent | Convention | Convention |

The difference: in Scenario A, these rules are knowledge the reviewer must possess. In Scenario C, they are structural requirements the AI cannot bypass.

---

## 4. Primitives → Requirement Map

All six primitives are used naturally in the Order-to-Cash flow:

| Primitive | Used in | Requirement |
|---|---|---|
| `ctx.db` | Discount rule update | FR7 |
| `ctx.cache` | Membership discount + explicit invalidation | FR7 |
| `ctx.lock` | Via `ctx.next_key` (tenant-scoped automatically) | FR1 |
| `ctx.queue` | Via `deliver channel: queue` — email, WA, receipt jobs | FR5 |
| `ctx.pubsub` | Via `deliver channel: websocket` — dashboard ticker | FR6 |
| `ctx.storage` | PDF receipt | FR10 |
| `ctx.kvstore` | Used by framework for idempotency store — handler never touches it | FR3 |
| `ctx.config` | Number prefix per workspace | FR8 |
| `ctx.log` | All key points | FR9 |

The outbox (FR4) is not a 10th primitive — it is the behavior of `publish.durable` backed by the database.

---

## 5. Test-Drive Findings

Writing this companion against the Core Basic v0.2.0 spec surfaced six findings:

| # | Finding | Status |
|---|---|---|
| 1 | `idempotent: true` had no mechanical semantics in the spec | ✅ **Resolved (D32):** Framework-enforced idempotency store with response replay, key from client (header/param — webhook) or server-issued (prepare-step for double-submit); optimistic concurrency via `version` CAS; `updated_at` demoted to audit metadata |
| 2 | Cross-resource validation (customer blacklist check) had no declarative home in Core Basic | ✅ **Acknowledged:** Home is `conditions` + script in Core Basic; validation levels 4–6 are Extended. Spec §13 updated to state this explicitly. |
| 3 | Cross-module `deliver.reliable_event` targets needed fully qualified form | ✅ **Resolved:** `resource: gl.journal-entry` format standardized in spec §12.3 |
| 4 | Webhook signature verification is a gap in Core Basic | ✅ **Acknowledged:** Home is `kind: Webhook` in Extended — gap is known, location is certain |
| 5 | Declarative vs imperative boundary needed normatization (D33) — "blacklist/gateway/queue in YAML or script?" | ✅ **Resolved:** Litmus test — facts/guarantees → YAML, procedures → handler, event consequences → `deliver`. Result: `mark-paid` handler shrank from ~30 lines to 3 lines + the `deliver` block became the complete consequence map. Also found `channel: queue` missing from spec §12.3 and added it. |
| 6 | `kind: Subscription` was born from the "idea arrives after the system is running" scenario (D35) — WA notification was in the wrong home (it's not a billing promise) | ✅ **Resolved:** Subscription created as the consumer-side reaction mechanism; prerequisite for signed-module ecosystem. Fan-out always compiled in `formspec describe`. |
