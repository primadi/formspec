# Forma Control Plane Spec v0.1.0

**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** Forma Overview · Forma Reference (D2, D3, D8, D11, D20–D22, D25, D27, D29–D32, D37, D39–D46)
**Companions:** Core Basic v0.2.0 · Plane Protocol Spec

> The Control Plane governs *who may change what, where, with whose approval, and with what proof*. It never reads business data and never executes business logic — that line is the first conformance item.

---

## 1. Scope & Fundamental Principles

`forma-control` is one of the two Forma binaries (always a separate process, including development). It owns:

1. **Environments** — identity set by infrastructure, immutable from the Resource Plane.
2. **Policy** — deployment, approval, promotion, emergency rules, evaluated by embedded OPA.
3. **Keys & signing** — platform keys in HSM/Vault/KMS; owner keys registered, never held.
4. **Contracts** — grants, consents, licenses: two-party-signed, portable, logged.
5. **Transparency log** — Merkle append-only audit with published checkpoints.
6. **Workspace & membership lifecycle** — provisioning, suspension, impersonation grants — *lifecycle only, never contents*.

**Hard prohibitions (normative, non-configurable):**
- The Control Plane MUST NOT read tenant business data or execute business handlers.
- The Resource Plane MUST NOT write to the Control Plane — it pulls policy at boot and every 5 minutes over mTLS, and keeps serving on last-known policy if Control is unreachable.
- Control compute is **stateless**; all state lives in its own storage (schema `forma_control`), never shared with app schemas.

## 2. Kinds

| Kind | Concern |
|---|---|
| `Environment` | One deployment target: identity, endpoints, resource pool mapping, tier, mode (dev/prod) |
| `Policy` | Rules evaluated on control-plane decisions |

```yaml
apiVersion: forma.dev/v1alpha1
kind: Environment
metadata: { name: production }
spec:
  mode: prod                        # dev | prod
  tier: enterprise                  # standalone | cloud | enterprise
  resource_pool: shared             # dev: always shared. prod: shared (prod-shared pool) | exclusive
  resource_planes:
    - url: https://api.myapp.com
  key_ref: kms://prod-signing       # platform signing key location
  policy: prod-policy               # kind: Policy applied to this environment
```

Environment identity reaches the Resource Plane only through infrastructure (`FORMA_ENVIRONMENT`, or K8s namespace labels) — never application code. Environment mode and resource pool determine infrastructure: dev uses the shared free pool; prod chooses between shared-prod (isolated from dev, cost-efficient, production guarantees) or exclusive (dedicated per-workspace instances).

Only two modes exist — `dev` and `prod`; "staging" is not a third mode but a conventional name for a prod-mode environment whose Policy profile is relaxed-but-recorded.

## 3. `kind: Policy` — Structured YAML + Rego Escape Hatch

The D33 pattern applied to governance: **declared vocabulary for common cases, scripted escape hatch for the rest** — here the script is Rego, evaluated by **OPA embedded as a Go library** inside `forma-control` (no extra process).

```yaml
apiVersion: forma.dev/v1alpha1
kind: Policy
metadata: { name: prod-policy }
spec:
  require_signing: true
  require_staging_first: true
  staging_min_duration: 24h
  auto_approve: []

  require_approval:
    - { impl: [script_ref, script], approvers: 2,
        approver_roles: [tech-lead, module-owner] }
    - { impl: [native, compiled, sidecar], approvers: 3,
        approver_roles: [cto, tech-lead, security],
        require_security_scan: true }

  verified_modules_only: true
  impl_trust_tiers: default          # unverified: sandbox only;
                                     # verified: + sidecar;
                                     # scanned+approved: + native

  blocked:                           # non-configurable floor
    - no_signature
    - environment_override_attempt

  rego: |                            # escape hatch — full OPA
    package forma.deploy
    deny[msg] {
      input.time.weekday == "Friday"
      input.time.hour >= 17
      msg := "No production deploys on Friday evening"
    }
```

**Rules:**
- Structured keys compile to Rego internally — one evaluation engine, one decision log. `forma-ctl policy test` runs table-driven tests.
- **Policy floor** (cannot be configured away, any tier): no self-approval (submitter ≠ approver); no unsigned artifacts in non-dev environments; no environment identity override from Resource Plane.
- OPA scope boundary (D2): governance decisions only — **never** business data authorization, which stays `required_permission` in the Resource Plane (Core §15).

## 4. Identity & Keys

Two key classes, deliberately different custody (D31):

| Class | Held by | Used for |
|---|---|---|
| **Owner keys** (workspace, app vendor, module vendor) | The owner — self-custodied ed25519; Control stores **public keys only** | Signing grants, consents, licenses, module releases |
| **Platform keys** (per environment, per tenant on Control Cloud) | HSM / Vault / Cloud KMS via `key_ref` | Signing deployment approvals, policy snapshots, log checkpoints |

### 4.1 Actors & Delegation (D40)

Four symmetric owner roles — one per ownable object; one identity may hold several:

| Owner | Owns | Signs |
|---|---|---|
| **Workspace Owner** | workspace (data, users, billing) | grants, consents, impersonation |
| **App Owner** | app artifact & versions | app releases |
| **Module Owner** | module package | module releases, licenses |
| **Cloud Owner** | the Forma Cloud instance | log checkpoints, platform policy |

- An **owner is exactly one identity**: email + self-custodied key. Two key forms, one contract model (D44): **passkey (WebAuthn) as the default** — private key in device secure enclave (platform-synced), contract hash bound into assertion challenge, signing UX = notification → tap → biometric — and raw ed25519 for power users via CLI. Contract verification MUST accept both signature envelopes.
- **Admins** act under a **delegation certificate**: admin has own key; owner signs certificate (scope, validity window, revocable). Every admin signature MUST carry its certificate — audit reads "consented by admin X under delegation of owner Y."
- **Non-delegable (owner-only):** accepting/relinquishing ownership, issuing/revoking delegations, owner key rotation.

### 4.2 Ownership Transfer (D40)

- **Consensual:** contract signed by outgoing AND incoming owner. Cloud Owner **facilitates and records** — never the granting authority.
- **Recovery** (death, lost key): operator-mediated with normative due process — identity re-attestation, **mandatory waiting period**, notification to all registered admins, public transparency-log entry.
- Unilateral reassignment by the operator = prohibited (operator backdoor).
- On transfer, existing delegation certificates revoked by default; new owner re-issues.

## 5. Artifact Lifecycle

### 5.1 Sign → Apply → Approve → Promote

```bash
forma sign -f order.yaml --key ~/.forma/keys/billing-team.key --environment staging
forma apply -f order.yaml            # → approval request per policy
forma promote order --from staging --to production   # same checksum verified
```

Here `staging` and `production` are environment *names*: both are `mode: prod` environments (§2); what distinguishes them is the Policy each one applies.

Approvals are signed statements by approver identities; the full set is one chain in the transparency log.

### 5.2 Consent Gates (D20/D21/D38)

Deployment evaluation includes footprint deltas: if a new version expands `uses`/`required_permission` aggregates, adds Subscriptions, or changes Page capability footprints, the affected **Data Owner's re-consent is a required approval** — the same mechanism as human approvers.

Consent presentation is normative (D44): screens MUST render footprints in **plain human language** ("this app version will be able to read your billing data"), localized, with technical detail expandable. Within a configured budget cap, recurring charges do not re-prompt (prepaid top-up constitutes billing approval, D42).

### 5.3 Data Classification

Resources MAY declare `classification: public | internal | confidential | restricted`. Classification is governance metadata: policies key on it, and implementations MUST block artifacts from reading resources above their own declared classification.

## 6. Contracts — Grants, Consents, Licenses

One document model for all three (D25, D27, D30, D31):

- A contract is a **portable signed document**: content + signature of both parties (owner keys) + timestamps. Both parties hold copies; Control Plane holds one too and anchors it in the log with an inclusion proof either party can fetch.
- **Grants** (cross-app): requested by consumer, signed by provider's Data Owner; revocation is itself a signed, logged contract. Metering records reference the grant ID (D22).
- **Consents**: install-time footprints and footprint-delta re-consents.
- **License tokens** (D8): validated locally by the Resource Plane, no call-home, air-gap safe. Normative floor from D27: a license token MUST NOT gate `list/find/export/backup` — implementations MUST reject tokens that attempt it.
- Portability is the Credible Exit spine (D31): a contract proves itself to a *different* conforming operator without the old operator's cooperation.

## 7. Transparency Log (D30)

- Append-only **Merkle tree** over: applies, approvals, rejections, promotions, key rotations, contracts, emergency actions, impersonation sessions, production REPL sessions.
- Every entry yields an **inclusion proof**; the tree head (checkpoint) is signed by the platform key and **published outside the operator's control** on a fixed cadence — third-party mirror or public endpoint.
- Verification: `forma-ctl log verify --checkpoint <file>` proves no history rewrite since that checkpoint.
- Blockchain rejected (D30); re-evaluate only under multi-operator federation.

## 8. Workspace & Membership Lifecycle

The Control Plane manages workspace **lifecycle**, never contents (D29): provisioning (create → seed roles/reference → active), suspension, termination, and billing hooks for the Platform Operator.

- Identity is workspace-level; **app membership and role assignment are per-app** (D37). Control stores the membership graph; Resource Plane enforces it per request.
- **Impersonation** (support access, D21): time-boxed grant signed by Data Owner naming the App Owner identity, scope, and duration; every impersonated request tagged in audit; session start/end logged in transparency log. No grant, no access — there is no operator backdoor.

### 8.1 Backup & Restore Governance (D41)

| Object | Governed by |
|---|---|
| Workspace data (entities, storage, preferences, config) | Workspace Owner — schedule, retention, scope, **external target** |
| App & module artifacts | Reproducible from git + registry; registry retention = Cloud Owner |
| Contracts, transparency log, membership | Cloud Owner + each party holds own copies (D31) |

Normative rules: `backup create` is delegable; **`restore` that overwrites requires owner signature or explicit `backup.restore`-scoped delegation**, always recorded in transparency log. Workspace Owners MAY set an **external backup target** they control — the living form of Credible Exit (D31); this MUST NOT be license-gated. Backups encrypted: per-tenant platform key by default, **owner-supplied key** as option.

## 9. REPL Governance

`forma repl` scope is environment policy. Since `mode` is binary (§2), the tiers below key off the environment's Policy profile — not a three-value mode; "staging" means a prod-mode environment designated as staging by its policy:

| Policy profile | Default scope | Extras |
|---|---|---|
| dev-mode environment | full `ctx.*` | — |
| prod-mode environment, staging profile | read-write | session recorded in audit |
| prod-mode environment, production profile | **read-only** | write requires explicit, time-boxed policy approval; every session recorded in transparency log |

The REPL always runs under a real user identity with that identity's permissions — it is never a superuser shell.

## 10. Deployment Tiers

| Tier | Control Plane | Notes |
|---|---|---|
| **Standalone** (free) | Same binary, relaxed dev policy; workspace primitive included | Single org self-host |
| **Control Cloud** | Shared stateless compute; **per-tenant keys + audit partitions** | Uptime target stricter than any Resource Plane |
| **Control Enterprise** | Self-hosted by customer under enterprise license — customer becomes Platform Operator (D29) | Premium = location, not features |

The display names map onto the `Environment.spec.tier` enum (§2): **Standalone** = `standalone`, **Control Cloud** = `cloud`, **Control Enterprise** = `enterprise`.

Enterprise governance features gated by license token (D8): HSM integration, SSO/SCIM, advanced approval chains, log-mirror tooling.

## 11. Emergency Controls

```bash
# Resource Plane side (authorized app admins)
forma freeze --reason "..."
forma rollback --since 1h --all
forma lock workspace <name> --reason "..."

# Control Plane side (Platform Operator / security)
forma-ctl freeze --reason "..."
forma-ctl revoke sessions --all --environment production
forma-ctl key rotate --environment production
```

Every emergency action requires a reason, is signed by the actor, and is logged in the transparency log.

## 12. Conformance

1. Never-read/never-execute prohibition (§1) with no configuration to disable it; stateless compute, isolated storage.
2. Environment + Policy kinds; embedded OPA evaluation of structured keys compiled to Rego + `rego:` escape hatch; policy floor unremovable; `forma-ctl policy test`.
3. Two key classes with stated custody; public-key-only storage for owner keys; per-tenant key/audit isolation on shared deployments; delegation certificates with owner-terminated chains; two-path ownership transfer with no unilateral operator reassignment.
4. Full artifact lifecycle with signed approvals, checksum-verified promotion, consent-delta gates with plain-language rendering, classification blocking.
5. Contract model: two-party signatures, portable documents, inclusion proofs; license tokens local-validated and unable to gate list/find/export/backup.
6. Merkle transparency log with externally published checkpoints and `log verify` tooling; impersonation and production-REPL sessions logged.
7. Workspace lifecycle + per-app membership graph; impersonation only via signed, time-boxed Data Owner grants; backup governance per §8.1 incl. owner-signed restore, external targets never license-gated, and owner-supplied encryption keys.
8. REPL scopes per §9 defaults.
