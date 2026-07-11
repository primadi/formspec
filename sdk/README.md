# Forma Client SDKs

This directory hosts **two unrelated protocol families** — don't confuse
them, they talk to different servers for different purposes:

| Family | Talks to | Audience | Docs |
|---|---|---|---|
| `lib-forma-*` ([php/](php/), [python/](python/), [typescript/](typescript/)) | `forma-sidecar`, over a local unix socket | Non-Go app processes implementing `impl: {type: sidecar}` action handlers | This file, §"lib-forma-* — Sidecar Protocol" below; `docs/runtimes/04-forma-sidecar.md` |
| `@forma/client` ([browser/](browser/)) | `forma-resource`'s generated REST API, over HTTPS | Frontend developers building pages against Forma entities | [`browser/README.md`](browser/README.md); `docs/cli-tools/03-forma-generate.md` |

---

## `lib-forma-*` — Sidecar Protocol

Thin client SDKs that bridge a non-Go app process to `forma-sidecar`
(docs/runtimes/04-forma-sidecar.md §4.4). Each SDK does exactly three
things — and deliberately nothing more:

1. Runs a small HTTP listener on a local socket, receiving
   `POST /invoke/{module}/{entity}/{action}` from the sidecar and calling
   the handler function the developer registered.
2. Exposes a `ctx` object whose methods (`ctx.db().query(...)`,
   `ctx.lock().acquire(...)`, ...) are HTTP calls back to the sidecar's
   `/ctx/{primitive}/{operation}` endpoints.
3. Serializes/deserializes the wire types.

**No Forma business logic lives here** — no state machine, no permission
checks, no entity storage. All of that stays in `forma-sidecar`.

| SDK | Directory | Runtime | Dependencies |
|---|---|---|---|
| `lib-forma-php` | [php/](php/) | PHP ≥ 8.1 | ext-curl, ext-json (stdlib only) |
| `lib-forma-python` | [python/](python/) | Python ≥ 3.9 | none (stdlib only) |
| `lib-forma` (TypeScript) | [typescript/](typescript/) | Node ≥ 18 | none at runtime |

## Wire contract

Both directions are HTTP/1.1 over a unix domain socket (default) or
localhost TCP. Default socket paths (override via env vars):

| Env var | Default | Direction |
|---|---|---|
| `FORMA_APP_SOCKET` | `/var/run/forma/app.sock` | sidecar → app (`/invoke/...`, `/health`) |
| `FORMA_SIDECAR_SOCKET` | `/var/run/forma/sidecar.sock` | app → sidecar (`/ctx/...`) |

Invoke request body: `{resource_id, resource, params, tenant_id, user_id}`.
Invoke response: `{data, new_state?, events?: [{name, durable?, payload?}]}`,
or non-200 with `{error}`. Ctx request: `{named?, sql?, args?, key?, value?,
ttl_seconds?}`; ctx response: `{data?, ok?, error?}`.

The authoritative Go counterparts are `internal/action/sidecar.go`
(invoke) and `internal/sidecar/ctx.go` (ctx proxy) — change those and these
SDKs together.

### Status

The `ctx.*` primitive backends in the engine are still stubs
(docs/runtimes/04-forma-sidecar.md §8): the sidecar answers `501` for
operations whose datastore backend is not implemented yet. The SDKs
surface that as a per-call error; the invoke path is fully functional.

---

## `@forma/client` — REST API Protocol

`browser/` is a completely different client: it calls `forma-resource`'s
generated REST API directly (`docs/spec/02-core-basic.md` §16) — no unix
socket, no sidecar involved. Paired with `forma generate --lang typescript`
(`cmd/forma/generate.go`), it's the typed client for hand-building frontend
pages (`docs/spec/05-frontend.md` §7's `forma.api`, before any manifest-driven
renderer exists). See [`browser/README.md`](browser/README.md) for the
runtime API and `docs/cli-tools/03-forma-generate.md` for the full guide —
including a step-by-step React + shadcn walkthrough.
