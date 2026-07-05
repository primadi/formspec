# Forma Control Spec v0.1.0 (draft)

**Status:** Draft — awaiting review
**License:** Creative Commons CC0
**Governed by:** Forma Foundation Document v2.0 (esp. D2, D3, D8, D11, D20–D22, D25, D27, D29–D32, D37)
**Resolves:** Q10 (policy language), Q13 (REPL governance)
**Companions:** Core Basic v0.2.0 · `forma-plane-protocol.md` (wire contract, separate doc)
**Supersedes:** Core Extended v0.1.4 §13–15 (moved here per Foundation §11)

> The Control Plane governs *who may change what, where, with whose
> approval, and with what proof*. It never reads business data and never
> executes business logic — that line is the first conformance item.

---

## 1. Scope & Fundamental Principles

`forma-control` is one of the two Forma binaries (D3 — always a separate
process, including development). It owns:

1. **Environments** — identity set by infrastructure, immutable from the
   Resource Plane.
2. **Policy** — deployment, approval, promotion, emergency rules,
   evaluated by an embedded OPA engine (§3).
3. **Keys & signing** — platform keys in HSM/Vault/KMS; owner keys
   registered, never held (§4).
4. **Contracts** — grants, consents, licenses: two-party-signed, portable,
   logged (§6).
5. **Transparency log** — Merkle append-only audit with published
   checkpoints (§7).
6. **Workspace & membership lifecycle** — provisioning, suspension,
   impersonation grants (§8) — *lifecycle only, never contents*.

Hard prohibitions (normative, non-configurable):

- The Control Plane MUST NOT read tenant business data or execute business
  handlers.
- The Resource Plane MUST NOT write to the Control Plane — it pulls policy
  at boot and every 5 minutes over mTLS, and keeps serving on last-known
  policy if Control is unreachable (details: plane-protocol spec).
- Control compute is **stateless**; all state lives in its own storage
  (schema `forma_control`), never shared with app schemas.

## 2. Kinds

This spec defines two kinds, same manifest format as Core §3:

| Kind | Concern |
|---|---|
| `Environment` | one deployment target: identity, endpoints, tier |
| `Policy` | rules evaluated on control-plane decisions |

```yaml
apiVersion: forma.dev/v1alpha1
kind: Environment
metadata: { name: production }
spec:
  tier: enterprise                # standalone | cloud | enterprise (§10)
  resource_planes:
    - url: https://api.myapp.com
  key_ref: kms://prod-signing     # platform signing key location
  policy: prod-policy             # kind: Policy applied to this environment
```

Environment identity reaches the Resource Plane only through
infrastructure (`FORMA_ENVIRONMENT`, or K8s namespace labels) — never
application code.

## 3. `kind: Policy` — structured YAML + Rego escape hatch (resolves Q10)

The D33 pattern applies to governance too: **declared vocabulary for the
common cases, a scripted escape hatch for the rest** — here the script is
Rego, evaluated by **OPA embedded as a Go library** inside `forma-control`
(D2 — no extra process, single-binary philosophy intact).

```yaml
apiVersion: forma.dev/v1alpha1
kind: Policy
metadata: { name: prod-policy }
spec:
  require_signing: true
  require_staging_first: true
  staging_min_duration: 24h
  auto_approve: []                       # none in production

  require_approval:
    - { impl: [script_ref, script], approvers: 2,
        approver_roles: [tech-lead, module-owner] }
    - { impl: [native, compiled, sidecar], approvers: 3,
        approver_roles: [cto, tech-lead, security],
        require_security_scan: true }

  verified_modules_only: true            # D24 — marketplace signature required
  impl_trust_tiers: default              # D46 — unverified: sandboxed impls only;
                                         # verified: + sidecar; scanned+approved: + native

  blocked:                               # non-configurable floor includes:
    - no_signature                       #   self_approval (always blocked)
    - environment_override_attempt

  rego: |                                # escape hatch — full OPA for the rest
    package forma.deploy
    deny[msg] {
      input.time.weekday == "Friday"
      input.time.hour >= 17
      msg := "No production deploys on Friday evening"
    }
    deny[msg] {
      input.artifact.classification == "restricted"
      not input.approvals.security
      msg := "Restricted-data artifacts require security approval"
    }
```

Rules:

- The structured keys compile to Rego internally — one evaluation engine,
  one decision log. `forma-ctl policy test` runs table-driven tests against
  sample inputs.
- **Policy floor** (cannot be configured away, any tier): no self-approval
  (submitter ≠ approver, D20-adjacent); no unsigned artifacts in
  non-dev environments; no environment identity override from the Resource
  Plane.
- Policy evaluation `input` includes: artifact metadata + checksum +
  declared footprint (`required_permission` + `uses` aggregate),
  submitter, approvals so far, environment, time, staging history,
  classification (§5.3).
- OPA scope boundary (D2): governance decisions only — **never** business
  data authorization, which stays `required_permission` in the Resource
  Plane (Core §15).

## 4. Identity & Keys

Two key classes, deliberately different custody (D31):

| Class | Held by | Used for |
|---|---|---|
| **Owner keys** (workspace, app vendor, module vendor) | the owner — self-custodied ed25519; Control stores **public keys only** | signing grants, consents, licenses, module releases |
| **Platform keys** (per environment, per tenant on Control Cloud) | HSM / Vault / Cloud KMS via `key_ref` | signing deployment approvals, policy snapshots, log checkpoints |

- `forma-ctl key rotate --environment production` rotates platform keys;
  owner key rotation is owner-initiated (new key signed by old — a chain,
  recorded in the log).
- On **Control Cloud**, compute is shared but per-tenant platform keys and
  audit partitions are never shared — one tenant can never sign another's
  artifacts (Foundation §Control-Cloud answer).
- Recovery for lost owner keys is a Platform Operator procedure (identity
  re-attestation), never a platform-held copy — custody is the point.

### 4.1 Actors & delegation (D40)

Four symmetric owner roles — one per ownable object; one identity may hold
several roles:

| Owner | Owns | Signs |
|---|---|---|
| Workspace Owner | workspace (data, users, billing) | grants, consents, impersonation |
| App Owner | app artifact & versions | app releases |
| Module Owner | module package | module releases, licenses |
| Cloud Owner | the Forma Cloud instance | log checkpoints, platform policy |

- An **owner is exactly one identity**: email + self-custodied key. Two
  key forms, one contract model (D44): **passkey (WebAuthn) as the
  default** — private key in the device secure enclave (platform-synced),
  contract hash bound into the assertion challenge, signing UX =
  notification → tap → biometric — and raw ed25519 for power users via
  CLI. Contract verification (§6) MUST accept both signature envelopes.
  Self-custody (D31) holds in both forms: the platform stores public keys
  only.
- **Admins** act under a **delegation certificate**: the admin has their
  own key; the owner signs a certificate (scope, validity window,
  revocable). Every admin signature MUST carry its certificate — audit
  reads "consented by admin X under delegation of owner Y", and the
  cryptographic chain always terminates at the owner key.
- **Non-delegable (owner-only) acts:** accepting/relinquishing ownership,
  issuing/revoking delegations, owner key rotation.
- `forma-ctl` actors are Cloud-Owner admins under the same mechanism —
  platform operations are delegated signatures too.

### 4.2 Ownership transfer (D40)

- **Consensual transfer:** a contract signed by outgoing AND incoming
  owner (§6 model); the Cloud Owner **facilitates and records** (identity
  verification, billing continuity, transparency-log entry) — it is never
  the granting authority.
- **Recovery transfer** (death, lost key, acquisition dispute):
  operator-mediated with normative due process — identity re-attestation,
  a **mandatory waiting period**, notification to all registered admins
  and contacts, and a publicly visible transparency-log entry.
- Unilateral reassignment by the operator is prohibited (it would be the
  operator backdoor §8 forbids); an implementation offering it is
  non-conforming.
- On transfer, existing delegation certificates are revoked by default;
  the new owner re-issues.

## 5. Artifact Lifecycle

### 5.1 Sign → apply → approve → promote

```bash
forma sign -f order.yaml --key ~/.forma/keys/billing-team.key --environment staging
forma apply -f order.yaml                    # → approval request per policy
forma promote order --from staging --to production
#  → same checksum verified → staging_min_duration verified
#  → production approval request created (policy §3)
```

Approvals are signed statements by approver identities; the full set
(submission, approvals, decision, deploy) is one chain in the transparency
log. Canary strategies (`--strategy canary --canary-percent 5
--auto-rollback-on-error-rate 1.0`) are executed by the Resource Plane;
the Control Plane records the plan and the outcome.

### 5.2 Consent gates (D20/D21/D38)

Deployment evaluation includes footprint deltas: if a new version expands
a module's `uses`/`required_permission` aggregate, adds Subscriptions, or
changes a Page footprint, the affected **Data Owner's re-consent is a
required approval** — the same mechanism as human approvers, recorded the
same way.

Consent presentation is normative (D44): screens MUST render footprints in
**plain human language** ("this app version will be able to read your
billing data"), localized, with the technical permission detail expandable
— never permission strings as the primary text. Owner approval UX is
notification → tap → biometric (passkey signing, §4.1). Within a
configured budget cap, recurring charges do not re-prompt (prepaid top-up
constitutes billing approval, D42).

### 5.3 Data classification

Resources MAY declare `classification: public | internal | confidential |
restricted`. Classification is governance metadata: policies key on it
(§3 example), and implementations MUST block an artifact from reading
resources above its own declared classification.

## 6. Contracts — grants, consents, licenses

One document model for all three (D25, D27, D30, D31):

- A contract is a **portable signed document**: content + signature of
  both parties (owner keys, §4) + timestamps. Both parties hold copies;
  the Control Plane holds one too and anchors it in the log (§7) with an
  inclusion proof either party can fetch.
- **Grants** (cross-app): requested by consumer, signed by provider's Data
  Owner; revocation is itself a signed, logged contract. Metering records
  reference the grant ID (D22).
- **Consents**: install-time footprints and §5.2 deltas.
- **License tokens** (D8): validated locally by the Resource Plane, no
  call-home, air-gap safe. Normative floor from D27: a license token MUST
  NOT gate `list/find/export/backup` — implementations MUST reject tokens
  that attempt it.
- Portability is the Credible Exit spine (D31): a contract proves itself
  to a *different* conforming operator without the old operator's
  cooperation.

## 7. Transparency Log (D30)

- Append-only **Merkle tree** over: applies, approvals, rejections,
  promotions, key rotations, contracts, emergency actions, impersonation
  sessions, production REPL sessions (§9).
- Every entry yields an **inclusion proof**; the tree head (checkpoint) is
  signed by the platform key and **published outside the operator's
  control** on a fixed cadence — third-party mirror, RFC 3161 timestamp,
  or public endpoint monitored by others.
- Verification tooling ships in the CLI: `forma-ctl log verify
  --checkpoint <file>` proves no history rewrite since that checkpoint.
- Blockchain remains rejected (D30); re-evaluate only under multi-operator
  federation (Q19).

## 8. Workspace & Membership Lifecycle

The Control Plane manages workspace **lifecycle**, never contents (D29):
provisioning (create → seed roles/reference → active), suspension,
termination, and billing hooks for the Platform Operator.

- Identity is workspace-level; **app membership and role assignment are
  per-app** (D37). Control stores the membership graph; the Resource
  Plane enforces it per request.
- **Impersonation** (support access, D21): a time-boxed grant signed by
  the Data Owner naming the App Owner identity, scope, and duration; every
  impersonated request is tagged in audit; the session start/end is
  logged in the transparency log. No grant, no access — there is no
  operator backdoor to tenant data in a conforming implementation.

### 8.1 Backup & restore governance (D41)

Core §26 defines the format and CLI; this section defines *who governs
what*:

| Object | Logical backup | Governed by |
|---|---|---|
| Workspace data (entities, storage files, preferences, config values) | customizable: schedule, retention, scope, target | Workspace Owner / delegated admin |
| App & module artifacts | none needed — reproducible from git + signed registry releases | App/Module Owner via source control; registry retention = Cloud Owner |
| Contracts, transparency log, membership graph | Cloud Owner — parties additionally hold their own contract copies (D31) | Cloud Owner |
| Infra durability (replication, snapshots) | not a logical backup; invisible to owners | Cloud Owner, per SLA |

Normative rules:

- `backup create` is delegable; a **restore that overwrites data requires
  the owner's signature or an explicit `backup.restore`-scoped
  delegation**, and is always recorded in the transparency log.
- Workspace Owners MAY set an **external backup target** they control
  (e.g. their own S3 bucket) for scheduled backups — the living form of
  Credible Exit (D31); per D27 this capability MUST NOT be license-gated.
- Backups are encrypted: per-tenant platform key by default,
  **owner-supplied key** as an option (custody symmetry with §4).
- Consistency is guaranteed **per app** (transaction boundaries);
  cross-app consistency within a workspace is near-point-in-time —
  restore MUST be followed by outbox reconciliation.
- Provider-app vendors back up their own workspaces with this same
  mechanism (they are Workspace Owners of their own data, D26) — no
  special case exists.

## 9. REPL Governance (resolves Q13)

`forma repl` scope is environment policy, with this normative default:

| Environment | Default scope | Extras |
|---|---|---|
| dev | full `ctx.*` | — |
| staging | read-write | session recorded in audit |
| production | **read-only** (`ctx.db` read tier, no mutations, no queue/pubsub emit) | write scope requires an explicit, time-boxed policy approval; **every production session is recorded in the transparency log** (commands + identity) |

The REPL always runs under a real user identity with that identity's
permissions — it is never a superuser shell.

## 10. Deployment Tiers

| Tier | Control Plane | Notes |
|---|---|---|
| **Standalone** (free) | same binary, relaxed dev-style policy available; workspace primitive included (D29) | single org self-host; Credible Exit applies to itself too |
| **Control Cloud** | shared stateless compute; **per-tenant keys + audit partitions** (§4) | uptime target stricter than any Resource Plane; outage tolerated via last-known policy |
| **Control Enterprise** | self-hosted by the customer under enterprise license — the customer becomes Platform Operator (D29) | premium = location, not features |

Enterprise governance features gated by license token (D8): HSM
integration, SSO/SCIM, advanced approval chains, log-mirror tooling — the
open-core line of Foundation D8.

## 11. Emergency Controls

```bash
# Resource Plane side (authorized app admins)
forma freeze --reason "..."            # stop deploys, keep serving
forma rollback --since 1h --all
forma suspend scripts --all
forma lock workspace <name> --reason "..."

# Control Plane side (Platform Operator / security)
forma-ctl freeze --reason "..."
forma-ctl revoke sessions --all --environment production
forma-ctl key rotate --environment production
```

Every emergency action requires a reason, is signed by the actor, and is
logged in the transparency log — emergencies are the moments proofs matter
most.

## 12. Conformance

1. Never-read/never-execute prohibition (§1) with no configuration to
   disable it; stateless compute, isolated storage.
2. Environment + Policy kinds; embedded OPA evaluation of structured keys
   compiled to Rego + `rego:` escape hatch; policy floor unremovable;
   `forma-ctl policy test`.
3. Two key classes with stated custody; public-key-only storage for owner
   keys; per-tenant key/audit isolation on shared deployments; delegation
   certificates with owner-terminated chains; two-path ownership transfer
   with no unilateral operator reassignment (§4.1–4.2).
4. Full artifact lifecycle with signed approvals, checksum-verified
   promotion, consent-delta gates with plain-language rendering and
   passkey approval UX (§5.2/D44), classification blocking.
5. Contract model: two-party signatures, portable documents, inclusion
   proofs; license tokens local-validated and unable to gate
   list/find/export/backup.
6. Merkle transparency log with externally published checkpoints and
   `log verify` tooling; impersonation and production-REPL sessions logged.
7. Workspace lifecycle + per-app membership graph; impersonation only via
   signed, time-boxed Data Owner grants; backup governance per §8.1 incl.
   owner-signed restore, external targets never license-gated, and
   owner-supplied encryption keys.
8. REPL scopes per §9 defaults.

## Open questions (this spec)

| # | Question |
|---|---|
| C1 | Checkpoint publication cadence + minimum mirror requirements per tier |
| C2 | Prosedur recovery & transfer pemulihan: langkah re-attestation standar, durasi masa tunggu per tier, format notifikasi |
| C3 | Approval delegation & vacation rules (approver_roles at scale) |
| C4 | Security-scan integration contract for `require_security_scan` |

## Changelog

### v0.1.0
- Initial draft: extracted from Core Extended §13–15 and rewritten under
  Foundation D1–D38; adds OPA-embedded policy with YAML+Rego hybrid (Q10),
  two-class key custody (D31), contract model with transparency log (D30),
  consent gates as approvals, REPL governance (Q13), tier matrix,
  license-token floor (D27).
