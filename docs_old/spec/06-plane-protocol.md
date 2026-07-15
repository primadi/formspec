# Forma Plane Protocol Spec v0.2.0

**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** Forma Overview · Forma Reference (D3, D29–D32, D39, D42, D46, D47)
**Parties:** `forma-control` (server side) ↔ `forma-resource` (client side)
**Companions:** Core Basic v0.2.0 · Control Spec v0.1.0
**Architecture:** See `docs/architecture/03-deployment-flow.md` for the full deployment pipeline with multi-cluster topology, and `docs/architecture/01-architecture-overview.md` for the three-level control architecture (Region → Cluster → Resource).

> This is the contract that makes two independent implementations interoperable: how a Resource Plane learns what it may run, proves what it did, and keeps serving when the Control Plane is unreachable.

---

## 0. YAML Registration Pipeline (Developer Workflow)

This section describes the **two-stage pipeline** that every YAML manifest must traverse before it becomes a running resource. The pipeline is mandatory in all environments, including development — only the transport and strictness differ between dev and prod.

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ Developer     │     │  Control Plane   │     │  Resource Plane  │
│ (YAML files)  │     │  (forma-control) │     │ (forma-resource) │
└──────┬───────┘     └────────┬─────────┘     └────────┬─────────┘
       │                      │                        │
       │ 1. forma apply       │                        │
       │─────────────────────►│                        │
       │                      │ 2. Validate YAML       │
       │                      │ 3. Compute sha256      │
       │                      │ 4. Sign artifact       │
       │                      │ 5. Store in DB         │
       │◄─────────────────────│ 6. Response: OK        │
       │                      │     {id, version, hash}│
       │                      │                        │
       │                      │      ─── Stage 2 ───   │
       │                      │                        │
       │                      │ 7. GET /v1/snapshot    │
       │                      │    If-None-Match: v1   │
       │                      │◄───────────────────────│
       │                      │ 8. 304 / 200+snapshot  │
       │                      │───────────────────────►│
       │                      │                        │ 9. Compare sha256
       │                      │                        │    vs local manifest
       │                      │ 10. GET /v1/artifacts  │
       │                      │    (only if hash !=)   │
       │                      │◄───────────────────────│
       │                      │ 11. signed envelope    │
       │                      │───────────────────────►│
       │                      │                        │12. Verify → Load
       │                      │                        │13. Sync schema
       │                      │◄───────────────────────│14. POST /v1/evidence
       │                      │                        │    deploy_status
```

### Stage 1 — Registration (Developer → Control Plane)

`forma apply` is the **only** way to register YAML manifests. The Resource Plane MUST NOT load YAML directly from the filesystem.

Process:
1. Developer runs `forma apply -f path/to/spec/` (or `forma apply --watch` for hot-reload)
2. Control Plane receives YAML files, parses multi-document manifests
3. Runs validation (schema, kind, metadata, spec per kind)
4. Computes **sha256** of each file and aggregate envelope hash
5. Signs the artifact envelope with the platform key (or self-signed in dev)
6. Stores the artifact in the Control DB (`forma_control.artifacts`)
7. Updates desired `deployments` state with the new artifact version + hash
8. Returns `{ artifact_id, version, sha256 }`

### Stage 2 — Deployment (Control Plane → Resource Plane)

The Resource Plane **pulls** desired state from the Control Plane; the Control Plane never initiates deployment over the network.

1. Resource Plane calls `GET /v1/snapshot` with `If-None-Match: <current_version>`
2. Control Plane compares version:
   - **Same** → `304 Not Modified` (no body, minimal overhead)
   - **Different** → `200` + snapshot with `deployments` section
3. Resource Plane computes diff against its local `deployment_manifest.json`:
   - For each desired artifact: compare `sha256(desired)` vs `sha256(local)`
   - **Hash matches** → skip (emit `deploy_status: up_to_date`)
   - **Hash differs** → fetch artifact, verify, load, deploy
4. After loading, emit deploy evidence to Control Plane

### 0.1 Dev Mode Simplification

In development (`--dev`), all planes run on the same machine. The pipeline is structurally identical but simplified:

| Aspect | Development | Production |
|---|---|---|
| Transport | HTTP (localhost, no TLS) | gRPC + mTLS |
| Signing | Self-signed ed25519 | Platform key (HSM/KMS) |
| Approval | None (auto-approved) | Policy-based approval chain |
| Pull interval | **10 seconds** | 5 minutes |
| `forma apply` | `--watch` for hot-reload | Run-once per deployment |
| Evidence | Optional (logged locally) | Mandatory + transparency log |
| Resource trigger | `POST /v1/poll` (local optimization) | Wait for next pull cycle |

The **local poll trigger** (`POST /v1/poll`) is a dev-only endpoint: after `forma apply --watch` registers a new artifact, it sends a local HTTP call to the Resource Plane asking it to pull immediately. This reduces latency from 10s to ~100ms. This is NOT a Control→Resource push — it is a local orchestration on the developer's machine.

---

## 1. Direction of Authority

Two channels, strictly asymmetric:

1. **Desired-state channel (Control → Resource, pull-only).** The Resource Plane pulls a signed **snapshot** of everything governance decides: policy, desired deployments, trust anchors, revocations. The Resource Plane can never modify any of it.
2. **Evidence channel (Resource → Control, append-only).** The Resource Plane submits signed, write-once **evidence**: deploy outcomes, metering records, audit anchors, violation incidents. Evidence can be appended, never edited — and evidence never changes governance state by itself.

This refines the "no write-back" rule (D47): *the Resource Plane cannot mutate governance state; it can only append evidence.* Without the evidence channel, canary outcomes, verifiable metering (D42), and violation incidents (D46) would have no path.

---

## 2. Transport & Identity

- **mTLS is mandatory** on both channels. gRPC is the normative transport; an HTTPS/JSON binding MUST be offered for constrained environments.
- **Plane bootstrap:** a new `forma-resource` instance is enrolled with a one-time bootstrap token (issued by a Cloud-Owner admin). It generates a keypair locally and receives an **instance certificate** binding: instance ID, environment, workspace scope. Private keys never leave the instance.
- Certificates are short-lived (default 24h) and renewed over the existing session; revocation flows through the snapshot (§3.3).
- Development (`forma dev`): self-signed pair generated by the tooling — same protocol, relaxed policy (D3: two processes even in dev).

---

## 3. Desired-State Channel

### 3.1 Snapshot

`GET /v1/snapshot` uses **ETag-based conditional pull**. The Resource Plane sends `If-None-Match` with the last known snapshot version. The Control Plane responds:

- `304 Not Modified` — version unchanged, no body, no data transfer
- `200` + signed bundle — version changed

| Section | Contents |
|---|---|
| `meta` | Monotonic `version`, issued-at, environment, signature (platform key) |
| `policy` | Compiled policy bundle (structured keys + Rego, Control §3) |
| `deployments` | Desired set: app/module artifact IDs + **sha256** + versions per workspace |
| `trust` | Public keys: owner keys, delegation certs, vendor keys, platform keys |
| `grants` | Active cross-app grants relevant to this plane's workspaces |
| `licenses` | License-token state deltas (tokens themselves validate locally, D8) |
| `revocations` | Sessions, delegations, grants, certificates revoked since last version |
| `memberships` | Per-app membership/role graph for this plane's workspaces (D37) |

**Design rationale — why conditional pull instead of persistent stream:**
- Control Plane is **stateless** (Control Spec §1) — persistent streams create in-memory state incompatible with horizontal scaling
- ETag comparison is O(1) per request — server cost is negligible even for thousands of Resource Planes
- `304` response is ~250 bytes — bandwidth minimal when majority of polls return "no change"
- No sticky session requirement for load-balanced Control instances
- See D47 and §0.1 for dev-mode poll interval

**Rules:**
- The Resource Plane MUST verify the signature and MUST reject any snapshot whose `version` is not strictly greater than the last applied one (**rollback/downgrade attack protection**).
- Pull cadence: at boot, then every **5 minutes** (prod) or **10 seconds** (dev). No persistent stream or long-poll is required — the conditional pull alone is sufficient even at scale.
- Snapshots are **complete or delta**: a delta references its base version; a plane missing the base MUST fetch complete.

### 3.2 Artifacts

`GET /v1/artifacts/{id}` returns the **signed artifact envelope**: manifest bundle (yaml) + scripts + assets + binaries, a content manifest with per-file sha256, and the **signature chain**: author signature → approval signatures (per policy) → deploy authorization (platform key).

**Envelope schema:**

```json
{
  "artifact_id": "uuid-...",
  "version": 5,
  "sha256": "abc123...",
  "files": [
    {"path": "billing/invoice.yaml", "sha256": "def...", "content": "<base64>"},
    {"path": "billing/scripts/invoice_send.star", "sha256": "ghi...", "content": "<base64>"}
  ],
  "signature_chain": [
    {"identity": "author@example.com", "signature": "..."},
    {"identity": "platform-key", "signature": "..."}
  ]
}
```

Before loading anything, the Resource Plane MUST verify, in order: envelope integrity (hashes) → author identity against `trust` → approval chain against the policy in force → deploy authorization → impl-type trust-tier gate (D46). Any failure = artifact rejected + violation evidence emitted (§4).

#### Hash-Based Deployment Optimization

The `deployments` section in the snapshot includes `sha256` per artifact. The Resource Plane maintains a local **deployment manifest** (`<data-dir>/deployment_manifest.json`):

```json
{
  "control_version": 42,
  "artifacts": {
    "billing/invoice": {
      "artifact_id": "uuid-...",
      "version": 5,
      "sha256": "abc...",
      "loaded_at": "2026-07-10T10:00:00Z",
      "status": "active"
    }
  }
}
```

**Convergence algorithm per desired deployment:**
1. Lookup `sha256` of desired artifact from snapshot
2. Compare with `sha256` in local deployment manifest
3. **Match** → emit `deploy_status: up_to_date` (zero network transfer for artifacts)
4. **Mismatch** → `GET /v1/artifacts/{id}` → verify → load → update local manifest

This ensures that re-deploying the same YAML (common in dev with `--watch` when saving unchanged files) is a no-op.

### 3.3 Convergence

The Resource Plane converges toward `deployments` GitOps-style: compute diff → compare sha256 → fetch missing/changed artifacts → verify → load/unload → emit deploy evidence. Canary plans arrive as part of the desired deployment entry; execution and rollback are local, outcomes are evidence.

---

## 4. Evidence Channel

`POST /v1/evidence` accepts signed batches. Each record: `{ type, instance_id, sequence, payload, signature }` — signed with the instance key; `sequence` is per-instance monotonic (gap detection).

| Type | Payload (NEVER contains business data) |
|---|---|
| `deploy_status` | Artifact ID, phase, sha256, version. Phases: `up_to_date` (hash match — no-op), `fetched`, `verified`, `loaded`, `failed`, `rolled_back`. Canary metrics summary |
| `metering` | Grant ID / license ID, counters per period — **counts only** (D42) |
| `audit_anchor` | Merkle root of the plane's local audit segment (ties workspace audit into the transparency log without shipping contents) |
| `violation` | USES_VIOLATION incidents: module, action, declared-vs-attempted, auto-suspend state (D46) |
| `health` | Instance heartbeat summary (coarse; fine registry stays in Valkey, Core §18) |

**Rules:**
- The Control Plane MUST treat evidence as **append-only** and anchor every batch in the transparency log (D30) — evidence about the operator is thereby tamper-evident too.
- Evidence MUST be **buffered locally** when Control is unreachable and flushed in order on reconnect; buffering is bounded by disk, never by time.
- Metering evidence is the substrate of verifiable billing (D42): signed by the plane, anchored in the log, checkable by both vendor and Workspace Owner.

---

## 5. Outage Semantics

The Resource Plane MUST keep serving on the **last-known snapshot** indefinitely — availability of tenant workloads never depends on the Control Plane. Degradation is staged, not binary:

| Snapshot age | Effect |
|---|---|
| < 15 min | Normal |
| ≥ 15 min | Warning surfaced in ops channels; evidence buffering active |
| ≥ policy-defined threshold | **High-governance operations locally refused:** new deployments, permission-expanding changes, production REPL write approvals. Runtime traffic unaffected. |

Revocations are the known trade-off of pull-based distribution: a revoked grant/session may live until the next successful pull. Implementations MUST honor the push-hint for revocation-bearing versions and SHOULD apply a shorter pull interval when the last snapshot contained pending revocations.

---

## 6. Versioning & Compatibility

- Protocol version negotiated at session start (`forma-plane/1`); a plane MUST refuse to operate against an unknown major version.
- Snapshot schema evolves additively within a major version; unknown sections are ignored (forward compatibility), unknown **signature or trust constructs are not** (fail closed).
- Clock skew tolerance: signatures carry issued-at; verification allows ±5 min; monotonic `version`, not wall-clock, orders snapshots.

---

## 7. Conformance

1. **Two-channel asymmetry:** no API exists by which a Resource Plane mutates governance state; evidence endpoints are append-only and log-anchored.
2. **Two-stage pipeline:** YAML MUST go through `forma apply` → Control Plane registration → Resource Plane pull. Direct filesystem loading by the Resource Plane is non-conformant.
3. **ETag-based conditional pull** with no persistent stream requirement; 304 Not Modified for unchanged state; monotonic-version rollback protection; delta + complete forms; 5-minute prod / 10-second dev pull cadence.
4. **Artifact envelope verification** in the §3.2 order, including trust-tier gating; hash-based deployment optimization (compare sha256 before fetching); rejected artifacts emit violation evidence.
5. **GitOps convergence** with deploy evidence (`up_to_date` / `loaded` / `failed`) for every phase.
6. **Evidence buffering** (disk-bounded), ordered flush, per-instance monotonic sequence.
7. **Staged outage semantics:** serve-forever on last-known snapshot, high-governance refusal past threshold, revocation fast-path.
8. **Fail-closed** on unknown signature/trust constructs and unknown protocol major versions.

---

## Changelog

### v0.2.0 (2026-07-10)
- Added §0 YAML Registration Pipeline: two-stage workflow (register → deploy) with diagram, dev mode simplification table, and `POST /v1/poll` local trigger
- Updated §3.1: ETag-based conditional pull, removed persistent stream requirement, added design rationale for scalability, added sha256 to deployments section
- Updated §3.2: added envelope schema example, added hash-based deployment optimization with local deployment manifest specification
- Updated §3.3: convergence algorithm includes sha256 comparison step
- Updated §4: added `up_to_date` phase to deploy_status evidence
- Updated §7: added conformance items for two-stage pipeline, ETag pull, and hash-based optimization
- Dev mode: pull interval 10s (prod: 5min), no persistent stream required
