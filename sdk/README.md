# FormSpec Client SDKs

This directory hosts **two unrelated protocol families** — don't confuse
them, they talk to different servers for different purposes:

| Family | Talks to | Audience | Docs |
|---|---|---|---|
| `lib-formspec-*` ([go/](go/), [php/](php/), [python/](python/), [typescript/](typescript/), [java/](java/), [dotnet/](dotnet/), [ruby/](ruby/), [rust/](rust/)) | `formspec dev` / `formspec serve`, over a local unix socket | App processes in any language implementing `impl: {type: sidecar}` action handlers | This file, §"lib-formspec-* — Sidecar Protocol" below; `docs/runtimes/04-formspec-sidecar.md` |
| `@formspec/client` ([browser/](browser/)) | `formspec-resource`'s generated REST API, over HTTPS | Frontend developers building pages against FormSpec entities | [`browser/README.md`](browser/README.md); `docs/cli-tools/03-formspec-generate.md` |

---

## `lib-formspec-*` — Sidecar Protocol

Thin client SDKs that bridge an app process to `formspec dev` / `formspec serve`
(docs/runtimes/04-formspec-sidecar.md §4.4). Each SDK does exactly three
things — and deliberately nothing more:

1. Runs a small HTTP listener on a local socket, receiving
   `POST /invoke/{module}/{entity}/{action}` from the sidecar and calling
   the handler function the developer registered.
2. Exposes a `ctx` object whose methods (`ctx.db().query(...)`,
   `ctx.lock().acquire(...)`, ...) are HTTP calls back to the sidecar's
   `/ctx/{primitive}/{operation}` endpoints.
3. Serializes/deserializes the wire types.

**No FormSpec business logic lives here** — no state machine, no permission
checks, no entity storage. All of that stays in the `formspec` binary.

| SDK | Directory | Runtime | Dependencies |
|---|---|---|---|
| `lib-formspec-go` | [go/](go/) | Go ≥ 1.22 | none (stdlib only) |
| `lib-formspec-php` | [php/](php/) | PHP ≥ 8.1 | ext-curl, ext-json (stdlib only) |
| `lib-formspec-python` | [python/](python/) | Python ≥ 3.9 | none (stdlib only) |
| `lib-formspec-ts` | [typescript/](typescript/) | Node ≥ 18 | none at runtime |
| `lib-formspec-java` | [java/](java/) | Java ≥ 17 | none (stdlib only) |
| `lib-formspec-dotnet` | [dotnet/](dotnet/) | .NET ≥ 8.0 | none (stdlib only) |
| `lib-formspec-ruby` | [ruby/](ruby/) | Ruby ≥ 3.0 | none (stdlib only) |
| `lib-formspec-rust` | [rust/](rust/) | Rust ≥ 1.75 | `ureq`, `serde`/`serde_json` |

## Wire contract

Both directions are HTTP/1.1 over a unix domain socket (default) or
localhost TCP. Default socket paths (override via env vars):

| Env var | Default | Direction |
|---|---|---|
| `FORMA_APP_SOCKET` | `/tmp/formspec/app.sock` | sidecar → app (`/invoke/...`, `/health`) |
| `FORMA_SIDECAR_SOCKET` | `/tmp/formspec/sidecar.sock` | app → sidecar (`/ctx/...`) |

### Invoke (sidecar → app)

Invoke request body: `{resource_id, resource, params, user_id}`.
Invoke response: `{data, new_state?, events?: [{name, durable?, payload?}]}`,
or non-200 with `{error}`.

**Note:** `tenant_id` is NOT in the wire — it is derived by the sidecar from the
`X-FormSpec-Workspace` header that the SDK auto-injects on every request.
The Invocation struct exposed to handlers has no `tenantId`/`workspaceId`
field. If the handler needs workspace info, it calls `ctx.workspace.id`.

### Ctx (app → sidecar)

Ctx request: `{named?, sql?, args?, key?, value?, ttl_seconds?}`.
Ctx response: `{data?, ok?, error?}`.

**Note:** Entity operations (`update`, `increment`, `decrement`) do NOT accept
`tenant_id` as a parameter. Workspace isolation is enforced by the sidecar
from the `X-FormSpec-Workspace` header — cannot be overridden per-request.

The authoritative Go counterparts are `internal/action/sidecar.go`
(invoke) and `internal/sidecar/ctx.go` (ctx proxy) — change those and these
SDKs together.

### Status

The `ctx.*` primitive backends in the engine are still stubs
(docs/runtimes/04-formspec-sidecar.md §8): the sidecar answers `501` for
operations whose datastore backend is not implemented yet. The SDKs
surface that as a per-call error; the invoke path is fully functional.

---

## `@formspec/client` — REST API Protocol

`browser/` is a completely different client: it calls `formspec-resource`'s
generated REST API directly (`docs_old/spec/02-core-basic.md` §16) — no unix
socket, no sidecar involved. Paired with `formspec generate --lang typescript`
(`cmd/formspec/generate.go`), it's the typed client for hand-building frontend
pages (`docs_old/spec/05-frontend.md` §7's `formspec.api`, before any manifest-driven
renderer exists). See [`browser/README.md`](browser/README.md) for the
runtime API and `docs/cli-tools/03-formspec-generate.md` for the full guide —
including a step-by-step React + shadcn walkthrough.
