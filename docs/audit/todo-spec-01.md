Audit: docs/plan/todo.md vs docs/spec/
Spec baseline: 11-reference.md §2 Decisions Log contains only D1–D50 (lines 46–95; D50 is the last row at line 95). No decision D51+ exists anywhere in docs/spec/ (grep confirmed zero hits for D51–D58).

Findings (ranked by severity)
1. [HIGH] D51–D58 are not recorded in any spec document — they are plan-invented decisions presented with the same "D##" authority as the canonical log
todo.md lines 293–304 define D51–D58 under "Key Design Decisions (from 1.4 implementation discussions)." None appear in docs/spec/ (11-reference.md §2 stops at D50, line 95). The numbering collides with the reserved canonical namespace and will read as if they were ratified spec decisions. They are at best implementation notes. This is the root problem; sub-items below are the specific contradictions.

2. [HIGH] Line 251 — D50 miscited for Plane Protocol; the correct decision is D47
todo.md line 251: 5.6 | Plane Protocol (gRPC + mTLS) per D50.

D50 (11-reference.md:95) is "Multi-protocol router with workspace slug prefix and smart internal dispatch" — nothing to do with the inter-plane wire protocol.
Plane Protocol is D47 (11-reference.md:92, "Two-channel inter-plane model (Plane Protocol v0.1.0)"), and mTLS/no write-back is specified in 02-core-basic.md:181 and the dedicated 06-plane-protocol.md.
Fix: cite D47 (and 06-plane-protocol.md), not D50. (The task referred to "line 351"; the actual line is 251. The D50 summary in the bottom table at line 347 is a separate, correct statement — see finding 8.)
3. [HIGH] D52 internal namespace /_/api/v1 contradicts the spec's routing + exposure model
todo.md line 298 (D52) and line 288 describe an internal HTTP namespace /_/api/v1/... "auto, all entities." This conflicts with two normative statements:

D49 / 02-core-basic.md:79, 550 — deny-by-default: "No external endpoint is created unless the entity declares spec.expose." A /_/api/v1 route auto-created for all entities is exactly the surface D49 forbids.
D50 / 02-core-basic.md:553 — "Internal dispatch: same-process callers MUST bypass the network — the router detects registry locality and dispatches via direct function call." The spec models "internal" as no HTTP at all, not as a second HTTP namespace. There is no /_/ route family anywhere in §16 (the only defined REST path is /{workspace_slug}/api/{version}/{module}/{plural}/:id/:action, line 555).
4. [HIGH] D53 page-scoped URL /{ws}/{app}/{page}/api/v1/... contradicts the normative REST path and the spirit of D38
todo.md lines 194, 208–213, 288, 299 (Fase 4.7 + D53) define /{ws}/{app}/{page}/api/v1/{module}/{plural}.

The only normative REST structure is /{workspace_slug}/api/{version}/{module}/{plural}/:id/:action (02-core-basic.md:555; D50 line 95). There is no {app}/{page} segment in the spec, and {workspace_slug} is the sole required prefix (02-core-basic.md:552).
D38 (11-reference.md:83) explicitly rejects page-based authority as enforcement ("UI provenance cannot be verified server-side (confused deputy)… unmanaged clients (Flutter/API) never pass through a page at all"). todo.md line 234 ("Browser never hits backend directly; all through SPA → backend proxy") reintroduces a page-gate as an auth boundary, which is precisely the confused-deputy pattern D38 warns against — even though line 238 does preserve resource-level enforcement. At minimum the URL scheme diverges from §16.
5. [HIGH] D58 "three impl tiers" contradicts the "Five Implementation Types" and the D46 trust-tier model
todo.md line 304 (D58): "Three impl tiers. Local script (Tier 1) → Local binary/WASM (Tier 2) → Cloud-hosted handler (Tier 3)."

01-overview.md §10 (line 303, "Five Implementation Types") and 02-core-basic.md §6.1 (line 183) define five types: native, compiled, script, script_ref, sidecar. D58's three-way grouping is a different, unreconciled taxonomy.
"Cloud-hosted handler (Tier 3)" has no spec basis — the only "another process/ecosystem" type is sidecar, which the spec defines as local (Unix socket, 01-overview.md:313) and which D45 (11-reference.md:90) requires to be stateless local. There is no cloud-hosted execution tier.
A genuine three-tier model does exist but is different: D46 trust tiers (11-reference.md:91) = unverified→sandbox (script/script_ref/WASM) / Verified→+sidecar / scanned+approved→+native. D58 conflates/replaces this with a "local vs cloud" axis.
6. [MEDIUM] D55 (and line 287) invert the spec's cross-process protocol priority
todo.md line 301 (D55) and line 287: "same-process = direct Go call; cross-process = gRPC (binary, faster); REST as fallback."

D50 (11-reference.md:95) says cross-process callers "use the configured protocol adapter (REST/gRPC/WS)" — caller/config choice, no gRPC preference.
02-core-basic.md:551 states "REST and WebSocket are required transports; gRPC is recommended." Framing REST as a "fallback" behind gRPC inverts the spec's required/recommended ordering. The same-process=direct-dispatch half is correct (02-core-basic.md:553).
7. [MEDIUM] Internal test-count inconsistency in the header
Line 4: "Total Tests: ~152 (all passing)."
Line 321 (Test Summary): Total ~166.
Additionally, the Test Summary rows (lines 312–320: 89+9+23+7+18+9+2+0+1) actually sum to 158, matching neither 152 nor 166. Three different numbers.
Line 3 "Last Updated: 2026-07-06" is one day behind the current date (2026-07-07) — cosmetic, not an inconsistency.
8. [LOW/OK] Bottom-table decision summaries (lines 339–347) — all verified correct
Each summary matches 11-reference.md:

D3 (line 341) ↔ ref:48 ✓; D17 (342) ↔ ref:62 ✓; D20 (343) ↔ ref:65 ✓; D29 (344) ↔ ref:74 ✓; D32 (345) ↔ ref:77 ✓; D49 (346) ↔ ref:94 ✓; D50 (347, "Multi-protocol router, workspace slug, internal dispatch") ↔ ref:95 ✓.
Fase decision citations also verified correct: 5.4 artifact signing→D11 (ed25519 trust chain, ref:56; reinforced ref:69) ✓; 5.5 transparency log/Merkle→D30 (ref:75) ✓; 6.6 backup→D41 (ref:86) ✓; 6.7 LSP/JSON Schema→D34 (ref:79) ✓; 3.3 Subscription→D35 (ref:80) ✓; 4.7 page footprint→D38 (ref:83) ✓. Only the 5.6→D50 citation (finding 2) is wrong.

9. [NOTE] D54, D56, D57 are consistent with the spec (but still unrecorded)
D54 (line 300) correctly restates D38 (11-reference.md:83) page→permission materialization. ✓ consistent.
D56 URL transparency (line 302) aligns with D24 "manifests never encrypted" (11-reference.md:69). ✓
D57 Starlark-not-encrypted (line 303) aligns with D24 point (1) "real IP may be binary via impl native/compiled" (11-reference.md:69). ✓ (Note: D24 speaks of manifests; scripts are the .star file type — the principle transfers but is not literally stated for scripts.)