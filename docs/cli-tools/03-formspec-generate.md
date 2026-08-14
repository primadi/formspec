# `formspec generate` & the Browser Client SDK

**Status:** Partially implemented — see §7
**License:** Creative Commons CC0

> `formspec generate` derives a typed TypeScript client from your entity manifests. This document is both its CLI reference and the practical guide for **programmers building a page by hand** — the escape hatch that exists independently of the manifest-driven renderer (not built yet; its contracts are being written incrementally at `docs/spec/frontend/`). If you're hand-writing a React (or any) frontend against FormSpec's generated REST API today, this is the document to read.

The normative API delivery contract this client generates against (exposure model, query conventions, response envelope, error codes) lives at [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §6, §8 — read that first if you're implementing against the REST API directly rather than through this generated client. `formspec.api` — the generated, typed client this document covers — is one of two ways to consume that contract; the other is the (not yet built) manifest-driven renderer that interprets `kind: Page`/`Form`/`Table` at runtime. Both speak the same REST contract; `formspec.api` just hands you typed methods instead of you writing `fetch` calls by hand.

---

## 1. Two Layers

FormSpec's browser client is split in two:

| Layer                            | What it is                                                                                | Where it lives                                                                           | Hand-edited?                                                           |
| -------------------------------- | ----------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| **Runtime** (`@formspec/client`) | Auth, HTTP transport, pagination, error mapping — the plumbing every entity's calls share | `sdk/browser` (npm package)                                                              | No — a stable, hand-written library you install                        |
| **Generated types**              | One TypeScript interface + one typed method group per exposed entity                      | Wherever `--out` points (e.g. `renderers/react-shadcn/src/generated/formspec-client.ts`) | **Never** — regenerate after every manifest change; the header says so |

You always need both: the generated file `import`s `@formspec/client` and calls its generic methods with your entity's real types filled in.

---

## 2. Installing

```bash
# from this repo, until @formspec/client is published to a registry:
npm install /workspaces/formspec/sdk/browser
# or, in a workspace/monorepo setup, a "file:" / "workspace:" dependency
```

---

## 3. `formspec generate` — CLI Reference

```bash
formspec generate --spec ./spec --out ./src/generated/formspec-client.ts
```

| Flag     | Default                          | Meaning                                                           |
| -------- | -------------------------------- | ----------------------------------------------------------------- |
| `--spec` | `./spec`                         | Manifest directory to read (same layout `formspec apply` accepts) |
| `--out`  | `./formspec-client.generated.ts` | Output file path (directories created as needed)                  |
| `--lang` | `typescript`                     | Target language(s); only `typescript` is implemented (see §7)     |

What it does, deterministically:

1. Loads entities from `--spec` (same loader `formspec-resource` uses — `internal/entity.Registry`).
2. Computes routes via `internal/api.GenerateRoutes` / `GenerateCustomActionRoutes` — the **same functions the server itself uses** to decide what's exposed, so the generated client can never drift from what the server actually serves (no re-derivation of `expose`/`disabled`/pluralization rules).
3. For every exposed entity, emits:
   - `interface {Module}{Entity}` — the entity's own fields (not the framework columns — those come from `FormaRecord<T>` in the runtime, see §4).
   - `interface {Module}{Entity}CreateInput` — same fields, excluding computed ones.
   - `type {Module}{Entity}UpdateInput = Partial<...CreateInput>` — PATCH sends only what changed.
   - `interface {Module}{Entity}{Action}Params` — one per custom action, if it declares `params.validate`.
4. Emits `export function createApi(client: FormaClient) { ... }` — a typed, nested `{module: {plural: {list, find, create, update, delete, ...customActions}}}` object built on the runtime's generic methods.

Nothing is silently guessed: if an entity has no `expose:` block, it produces zero routes and zero generated code for it (deny-by-default, D49) — `formspec generate` errors out if _no_ entity is exposed at all, since there'd be nothing to generate.

Field type mapping:

| Manifest `type`                      | TypeScript                       | Why                                                                                                                                                                                                                                                                             |
| ------------------------------------ | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `string`, `uuid`, `date`, `datetime` | `string`                         | —                                                                                                                                                                                                                                                                               |
| `integer`                            | `number`                         | 64-bit in Go, safe enough for typical counts in JS                                                                                                                                                                                                                              |
| `decimal`, `number`                  | `string`                         | **Never `number`** — money needs arbitrary precision, which `number` (float64) cannot guarantee. _Caveat:_ `internal/db` does not yet coerce decimal values to strings on the wire — this type is the intended contract, not a runtime guarantee, until that lands server-side. |
| `boolean`                            | `boolean`                        | —                                                                                                                                                                                                                                                                               |
| `enum`                               | `"a" \| "b" \| "c"` union        | From `enum_values`; falls back to `string` if none declared                                                                                                                                                                                                                     |
| `json`                               | `unknown`                        | Arbitrary shape                                                                                                                                                                                                                                                                 |
| `relation`                           | `string`                         | The referenced ID (`belongs_to`)                                                                                                                                                                                                                                                |
| `child`                              | `Array<{ ...inline fields... }>` | Recursively typed from `child.fields`                                                                                                                                                                                                                                           |

A field counts as required if either the manifest's top-level `required: true` **or** a `rules: [required, ...]` entry is present — real manifests in this repo exclusively use the latter, so both are checked.

**Field keys are never renamed.** `customer_id` stays `"customer_id"` in the generated interface, not `customerId` — the wire JSON is whatever `EntityRecord.MarshalJSON` produces (`internal/db/crud.go`), which spreads your field names verbatim. A generator that camelCased keys would produce code that compiles but is silently wrong at runtime (`record.customerId` would always be `undefined`).

---

## 4. The Runtime (`@formspec/client`)

```ts
import { FormaClient, FormaApiError } from "@formspec/client"

const client = new FormaClient({
  baseUrl: "https://api.example.com", // your formspec-resource origin
  workspace: "acme", // every route is workspace-prefixed (§16)
  getToken: () => localStorage.getItem("formspec_token") ?? undefined,
})
```

Generic methods any generated (or hand-written) call goes through:

```ts
client.list<T>(module, plural, { page?, perPage?, search? }): Promise<ListResult<T>>
client.find<T>(module, plural, id): Promise<T>
client.create<T>(module, plural, input): Promise<T>
client.update<T>(module, plural, id, patch): Promise<T>   // PATCH — send only changed fields
client.delete(module, plural, id): Promise<void>
client.action<T>(module, plural, id, actionName, params?): Promise<T>
```

`ListOptions` only has `page`/`perPage`/`search` — `sort` and `filter[field][op]` are part of the normative query convention above but **not wired server-side yet** (`internal/api/handler.go` never parses them), so they are intentionally absent from the runtime rather than silently ignored.

Every record you get back is typed `FormaRecord<T>` — your entity's own fields (`T`) plus the reserved columns:

```ts
interface RecordMeta {
  id: string
  tenant_id: string
  version: number
  created_at: string
  updated_at: string
  created_by: string
  updated_by: string
}
type FormaRecord<T> = RecordMeta & T
```

Errors are always `FormaApiError` (never a raw fetch rejection for a non-2xx response):

```ts
try {
  await api.billing.invoices.approve(id, { note })
} catch (e) {
  if (e instanceof FormaApiError) {
    if (e.isConflict) {
      /* 409 — someone else edited it, refetch */
    }
    if (e.isValidation) {
      /* 422 — e.details has per-field messages */
    }
    if (e.isForbidden) {
      /* 403 — hide the action, don't retry */
    }
  }
}
```

---

## 5. Building a Page by Hand — Walkthrough (React + shadcn)

This is the pattern for `renderers/react-shadcn/` (React + Vite + shadcn/Tailwind, see `renderers/react-shadcn/package.json`) today — there is no renderer yet, so _every_ page is "manual" in the sense the eventual renderer's `asset` escape-hatch component contract means: an ES module with a `mount(el, props, formspec)` entry point, framework-agnostic. This will keep working once/if a renderer exists, since it's the same underlying client.

**1. Generate, once per manifest change:**

```bash
formspec generate --spec ./spec --out renderers/react-shadcn/src/generated/formspec-client.ts
```

**2. Wire the client once** (e.g. `renderers/react-shadcn/src/lib/formspec.ts`):

```ts
import { FormaClient } from "@formspec/client"
import { createApi } from "@/generated/formspec-client"
import { useAuthStore } from "@/lib/auth-store" // wherever your token lives

export const formaClient = new FormaClient({
  baseUrl: import.meta.env.VITE_FORMA_API_URL,
  workspace: import.meta.env.VITE_FORMA_WORKSPACE,
  getToken: () => useAuthStore.getState().token,
})

export const api = createApi(formaClient)
```

**3. A list page** — no data-fetching library required (though `@tanstack/react-query` — not currently a `renderers/react-shadcn/` dependency — is a natural fit if you want caching/revalidation; the client returns plain Promises either way):

```tsx
import { useEffect, useState } from "react"
import { api } from "@/lib/formspec"
import type { FormaRecord } from "@formspec/client"
import type { BillingInvoice } from "@/generated/formspec-client"
import { Button } from "@/components/ui/button"

export function InvoiceListPage() {
  const [invoices, setInvoices] = useState<FormaRecord<BillingInvoice>[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.billing.invoices
      .list({ perPage: 20 })
      .then((result) => setInvoices(result.data))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div>Loading…</div>

  return (
    <table>
      <tbody>
        {invoices.map((inv) => (
          <tr key={inv.id}>
            <td>{inv.customer_id}</td>
            <td>{inv.status}</td>
            <td>{inv.total}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
```

**4. A create form** (with `react-hook-form` + `zod`, both already in `renderers/react-shadcn/package.json`):

```tsx
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { api } from "@/lib/formspec"
import { FormaApiError } from "@formspec/client"
import { Button } from "@/components/ui/button"

const schema = z.object({
  customer_id: z.string().min(1),
  total: z.string().min(1), // decimal fields are strings — see §3
})

export function CreateInvoiceForm({
  onCreated,
}: {
  onCreated: (id: string) => void
}) {
  const form = useForm({ resolver: zodResolver(schema) })

  async function onSubmit(values: z.infer<typeof schema>) {
    try {
      const invoice = await api.billing.invoices.create({
        ...values,
        status: "draft",
      })
      onCreated(invoice.id)
    } catch (e) {
      if (e instanceof FormaApiError && e.isValidation) {
        e.details?.forEach((d) =>
          form.setError(d.field as any, { message: d.message }),
        )
      }
    }
  }

  return (
    <form onSubmit={form.handleSubmit(onSubmit)}>
      {/* fields bound via form.register(...) */}
      <Button type="submit">Create</Button>
    </form>
  )
}
```

**5. A custom action button:**

```tsx
async function handleApprove(id: string) {
  try {
    await api.billing.invoices.approve(id, { note: "" })
    // refetch or optimistically update
  } catch (e) {
    if (e instanceof FormaApiError && e.isForbidden) {
      // hide/disable the button instead of showing this error next time —
      // permission-driven UI is still the caller's responsibility until a
      // renderer exists (see contract below)
    }
  }
}
```

**Permissions today:** nothing here hides a button for a caller who lacks permission. The server enforces it (a `403 FORBIDDEN` `FormaApiError`), but the _UI_ decision to hide/disable is on you until a renderer exists. Don't rely on hiding controls as a security boundary — it never was one; enforcement always lives at the resource (`required_permission`), never at the UI layer. The eventual renderer's permission-filtering contract — why it derives visibility from the permission catalog rather than a UI-declared role list, and why page-based authorization is rejected as an enforcement mechanism — is specified at [`docs/spec/frontend/04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md) §4.

---

## 6. What This Does _Not_ Give You

Explicitly out of scope for `@formspec/client` + `formspec generate` (all part of the not-yet-built manifest-driven renderer):

- No `kind: Page`/`Form`/`Table` interpretation — you lay out the page yourself.
- No `formspec.ui` (toast/dialog/confirm/drawer), `formspec.subscribe` (realtime), `formspec.navigate`, `formspec.form()` headless engine, or `formspec.files` transfer manager.
- No permission-driven visibility — you decide what to hide based on `FormaApiError.isForbidden` or by fetching the caller's permission list yourself (no generated helper for that today).
- No `sort=`/`filter[field][op]=` query support — not wired server-side (§4).

---

## 7. Status Implementasi Hari Ini

**Implemented:**

- `@formspec/client` runtime (`sdk/browser`) — `FormaClient` with `list/find/create/update/delete/action`, `FormaApiError` with `.isConflict/.isValidation/.isForbidden`, zero runtime dependencies (native `fetch`).
- `formspec generate --lang typescript` (`cmd/formspec/generate.go`) — reads manifests via the same registry/route-generation code the server uses, emits per-entity interfaces + a typed `createApi(client)` wrapper.
- Fixed as a prerequisite: `internal/db.EntityRecord` had no JSON tags at all, so the real wire format was PascalCase keys with entity fields nested under a `"Data"` key — not what §16 specifies and not what any client (generated or hand-written) could reasonably expect. `EntityRecord.MarshalJSON` (`internal/db/crud.go`) now flattens fields and uses snake_case, matching the spec. No test previously exercised the full HTTP response path, so this had gone uncaught.

**Not implemented:**

- `--lang go` and `--lang openapi` (`docs/cli-tools/02-formspec-cli.md` §5 mentions both; only TypeScript exists).
- The manifest-driven renderer itself — `kind: Page`/`Form`/`Table`/`Dashboard`/etc. interpretation, derived admin panel, permission-driven visibility (contracts for this are being written incrementally at `docs/spec/frontend/`).
- `sort`/`filter[field][op]` query parameters (declared in §16, not parsed by `internal/api/handler.go`).
- Decimal-as-string is the generated _type_, not yet an enforced _wire_ guarantee — `internal/db` doesn't coerce decimal values before JSON-encoding them.
- Dart codegen for unmanaged clients (Flutter, native). Unmanaged clients are first-class API consumers today via this same REST contract — HTTP, realtime WebSocket, server-enforced permissions — Dart is simply not an official codegen target yet (TypeScript is the only one built).

---

## 8. References

| Document                                                                     | Content                                                                                                         |
| ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| [`02-formspec-cli.md`](02-formspec-cli.md) §5                                | `formspec generate` in the context of the full CLI verb set                                                     |
| [`docs/runtimes/05-engine-api-layer.md`](../runtimes/05-engine-api-layer.md) | `internal/api` package internals, response envelope                                                             |
| [`docs/spec/frontend/`](../spec/frontend/README.md)                          | Normative contracts for the manifest-driven renderer this document's escape hatch stands apart from             |
| `sdk/README.md`                                                              | The other client family — `lib-formspec-*` for `formspec-sidecar` (a different protocol; don't confuse the two) |
