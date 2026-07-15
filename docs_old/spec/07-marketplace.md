# Forma Marketplace Spec v0.1.0

**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** Forma Overview · Forma Reference (D11, D22, D24, D27, D30, D31, D42)
**Requires:** Core Basic v0.2.0 · Control Spec v0.1.0

> The marketplace is the economic layer of the Forma ecosystem — where modules, apps, themes, and widgets are listed, priced, metered, and settled. All artifacts are signed and distributed through the registry. Commercial terms live in listings, never in manifests.

---

## 1. Single Marketplace

One marketplace infrastructure for all distributable artifacts: **modules, apps, themes, and widgets**. All are signed artifacts in the same registry. The marketplace is a **listing and metering layer** on top of the registry — it does not replace it.

Commercial terms (price, revenue share, trial periods) live in the marketplace **listing**, never in the manifest (D22). The same module can be:
- Free and self-hosted (via the FSL-licensed binary)
- Paid on the marketplace (with revenue sharing)
— without any change to its YAML manifests.

---

## 2. Pricing Vocabulary (Closed)

All pricing fits into a fixed vocabulary. The marketplace MUST NOT allow custom pricing models beyond these:

| Model | Unit | Example |
|---|---|---|
| `free` | — | Community modules |
| `one_time` | Per license | Perpetual license |
| `subscription` | Per period (monthly/yearly) | Ongoing access |
| `per_seat` | Per active user membership (D37) | Team-based pricing |
| `per_call` | Per cross-app grant call (D25) | API-style pricing |
| `per_transaction` | Per metered event (count only) | Usage-based pricing |
| `metered_passthrough` | Raw metering data, vendor sets rates | Complex billing |

---

## 3. Verified Badge & Signed Provenance

The **Verified Badge** (D11) is mandatory for any paid listing. Requirements:
- Artifact signed with the vendor's ed25519 key
- Signature chain verifiable against the vendor's public key in the registry
- Manual or automated review process (policy defined by the Platform Operator)

Manifests are never encrypted (D24: readability is a feature). Protection of commercial value comes from:
1. Real IP may be binary via `impl` native/compiled without source
2. Signed provenance — a renamed copy cannot forge the signature and cannot enter a Verified-Only governed environment
3. The economics of update + support + liability do not transfer with a copy
4. Legal protection via module license

---

## 4. Verifiable Metering

Metering is the substrate of marketplace billing. Three guarantees:

1. **Counts only.** Metering records contain counts — never business data payloads. The Control Plane never sees what was bought, only that N transactions occurred.
2. **Signed by the Resource Plane.** Every metering record is signed with the instance key.
3. **Anchored in the transparency log.** Metering batches are appended to the evidence channel and anchored in the Control Plane's transparency log (D30).

This makes metering **verifiable by all parties against the operator**: the vendor verifies their sales, the Workspace Owner verifies their invoice. No trust in the Platform Operator is required.

---

## 5. Ledger & Settlement

One **ledger per owner** (Workspace Owner and Module Vendor are separate ledgers):

| Side | Contents |
|---|---|
| **Debit** | Infrastructure usage, module/app subscriptions, per-seat charges, per-call metering |
| **Credit** | Prepaid top-ups, marketplace payouts (for vendors) |

Income offsets charges — a vendor's sales revenue reduces their infrastructure bill.

### Settlement Models

| Model | How it works | Who qualifies |
|---|---|---|
| **Prepaid (default)** | Top up balance locally; balance exhausted → grace period → read-only degradation (D27 — `list/find/export/backup` never gated) | All accounts |
| **Postpaid (trust tier)** | Invoice/PO-based, net terms | Enterprise, verified vendors |

### Budget Cap

The Workspace Owner sets a **budget cap**. Within the cap, recurring charges auto-approve (prepaid top-up = billing approval itself, D44). Above the cap → requires explicit approval.

### Resource Plan Tiers

Infrastructure pricing reflects the resource pool:

| Tier | Compute | Data Store | Backup | Use Case |
|---|---|---|---|---|
| **Dev** (free) | Shared dev pool (capped) | Shared dev entity-store/kv-store | None | Development, testing |
| **Prod Shared** | Shared prod pool (isolated from dev) | Shared prod entity-store/kv-store/Postgres/Valkey | Included | Small-to-medium production systems, cost-efficient |
| **Prod Exclusive** | Dedicated per-workspace instance | Dedicated entity-store/kv-store/Postgres/Valkey | Included | High performance, regulatory isolation, large scale |

---

## 6. Revenue Sharing

Revenue sharing is based on the **dependency graph** (D22). When App A composes Module B (by vendor X), and App A earns revenue:

- The dependency edges are known from `depends` declarations in Module manifests
- Metering tracks usage per dependency
- Revenue share is calculated per the marketplace listing terms
- Payouts are credited to the vendor's ledger

Revenue-share percentages are part of the **listing**, not the manifest.

---

## 7. License Token Lifecycle

All pricing models resolve to a **license token** (D8) with a type and validity period:

| Pricing model | Token type |
|---|---|
| `free` | Perpetual, no expiration |
| `one_time` | Perpetual |
| `subscription` | Rolling, expires if not renewed |
| `per_seat` / `per_call` / `per_transaction` | Valid while balance/prepaid covers usage |

**Token properties (normative):**
- Validated **locally** by the Resource Plane — no call-home required
- **Air-gap safe** — works in fully isolated environments
- MUST NOT gate `list/find/export/backup` — implementations MUST reject tokens that attempt it (D27)
- Portable — the token is a signed document verifiable by any conforming implementation (D31)

---

## 8. Marketplace Governance

### Listing Requirements
- **Free listings:** valid ed25519 signature on the artifact
- **Paid listings:** Verified Badge + valid signature
- All listings: public artifact metadata (name, vendor, version, description, dependency graph, permission footprint)

### The Platform Operator
- Operates the marketplace infrastructure
- Sets platform fee percentages per category (business decision)
- Facilitates settlement between vendors and consumers
- Does **not** set individual module prices — vendors control pricing

### Fixed Guardrails (non-negotiable)
- Read-only degradation + export never gated (D27)
- Verified Badge mandatory for paid listings (D11)
- Token portable & air-gap safe (D8)
- Free/self-hosted FSL path remains intact
- The same module can be free self-hosted and paid on the marketplace without manifest changes (D22)
