# `forma generate` & the Browser Client SDK

**Status:** Partially implemented — see §7
**License:** Creative Commons CC0
**Governed by:** `docs/spec/02-core-basic.md` §16 (API Delivery), §23 (`forma generate`), `docs/spec/05-frontend.md` §7 (`forma.api` contract)

> `forma generate` derives a typed TypeScript client from your entity manifests. This document is both its CLI reference and the practical guide for **programmers building a page by hand** — the escape hatch that exists independently of the manifest-driven renderer described in `docs/spec/05-frontend.md` (which is not built yet — see that spec's own status notes). If you're hand-writing a React (or any) frontend against Forma's generated REST API today, this is the document to read.

---

## 1. Two Layers

Forma's browser client is split in two, matching the split in `docs/spec/05-frontend.md` §7 (`forma.api` = "generated, typed"):

| Layer | What it is | Where it lives | Hand-edited? |
|---|---|---|---|
| **Runtime** (`@forma/client`) | Auth, HTTP transport, pagination, error mapping — the plumbing every entity's calls share | `sdk/browser` (npm package) | No — a stable, hand-written library you install |
| **Generated types** | One TypeScript interface + one typed method group per exposed entity | Wherever `--out` points (e.g. `web/src/generated/forma-client.ts`) | **Never** — regenerate after every manifest change; the header says so |

You always need both: the generated file `import`s `@forma/client` and calls its generic methods with your entity's real types filled in.

---

## 2. Installing

```bash
# from this repo, until @forma/client is published to a registry:
npm install /workspaces/forma/sdk/browser
# or, in a workspace/monorepo setup, a "file:" / "workspace:" dependency
```

---

## 3. `forma generate` — CLI Reference

```bash
forma generate --spec ./spec --out ./src/generated/forma-client.ts
```

| Flag | Default | Meaning |
|---|---|---|
| `--spec` | `./spec` | Manifest directory to read (same layout `forma apply` accepts) |
| `--out` | `./forma-client.generated.ts` | Output file path (directories created as needed) |
| `--lang` | `typescript` | Target language(s); only `typescript` is implemented (see §7) |

What it does, deterministically:

1. Loads entities from `--spec` (same loader `forma-resource` uses — `internal/entity.Registry`).
2. Computes routes via `internal/api.GenerateRoutes` / `GenerateCustomActionRoutes` — the **same functions the server itself uses** to decide what's exposed, so the generated client can never drift from what the server actually serves (no re-derivation of `expose`/`disabled`/pluralization rules).
3. For every exposed entity, emits:
   - `interface {Module}{Entity}` — the entity's own fields (not the framework columns — those come from `FormaRecord<T>` in the runtime, see §4).
   - `interface {Module}{Entity}CreateInput` — same fields, excluding computed ones.
   - `type {Module}{Entity}UpdateInput = Partial<...CreateInput>` — PATCH sends only what changed.
   - `interface {Module}{Entity}{Action}Params` — one per custom action, if it declares `params.validate`.
4. Emits `export function createApi(client: FormaClient) { ... }` — a typed, nested `{module: {plural: {list, find, create, update, delete, ...customActions}}}` object built on the runtime's generic methods.

Nothing is silently guessed: if an entity has no `expose:` block, it produces zero routes and zero generated code for it (deny-by-default, D49) — `forma generate` errors out if *no* entity is exposed at all, since there'd be nothing to generate.

Field type mapping:

| Manifest `type` | TypeScript | Why |
|---|---|---|
| `string`, `uuid`, `date`, `datetime` | `string` | — |
| `integer` | `number` | 64-bit in Go, safe enough for typical counts in JS |
| `decimal`, `number` | `string` | **Never `number`** — money needs arbitrary precision, which `number` (float64) cannot guarantee. *Caveat:* `internal/db` does not yet coerce decimal values to strings on the wire — this type is the intended contract, not a runtime guarantee, until that lands server-side. |
| `boolean` | `boolean` | — |
| `enum` | `"a" \| "b" \| "c"` union | From `enum_values`; falls back to `string` if none declared |
| `json` | `unknown` | Arbitrary shape |
| `relation` | `string` | The referenced ID (`belongs_to`) |
| `child` | `Array<{ ...inline fields... }>` | Recursively typed from `child.fields` |

A field counts as required if either the manifest's top-level `required: true` **or** a `rules: [required, ...]` entry is present — real manifests in this repo exclusively use the latter, so both are checked.

**Field keys are never renamed.** `customer_id` stays `"customer_id"` in the generated interface, not `customerId` — the wire JSON is whatever `EntityRecord.MarshalJSON` produces (`internal/db/crud.go`), which spreads your field names verbatim. A generator that camelCased keys would produce code that compiles but is silently wrong at runtime (`record.customerId` would always be `undefined`).

---

## 4. The Runtime (`@forma/client`)

```ts
import { FormaClient, FormaApiError } from "@forma/client";

const client = new FormaClient({
  baseUrl: "https://api.example.com",     // your forma-resource origin
  workspace: "acme",                       // every route is workspace-prefixed (§16)
  getToken: () => localStorage.getItem("forma_token") ?? undefined,
});
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

`ListOptions` only has `page`/`perPage`/`search` — `sort` and `filter[field][op]` are declared in `docs/spec/02-core-basic.md` §16 but **not wired server-side yet** (`internal/api/handler.go` never parses them), so they are intentionally absent rather than silently ignored.

Every record you get back is typed `FormaRecord<T>` — your entity's own fields (`T`) plus the reserved columns:

```ts
interface RecordMeta {
  id: string; tenant_id: string; version: number;
  created_at: string; updated_at: string; created_by: string; updated_by: string;
}
type FormaRecord<T> = RecordMeta & T;
```

Errors are always `FormaApiError` (never a raw fetch rejection for a non-2xx response):

```ts
try {
  await api.billing.invoices.approve(id, { note });
} catch (e) {
  if (e instanceof FormaApiError) {
    if (e.isConflict) { /* 409 — someone else edited it, refetch */ }
    if (e.isValidation) { /* 422 — e.details has per-field messages */ }
    if (e.isForbidden) { /* 403 — hide the action, don't retry */ }
  }
}
```

---

## 5. Building a Page by Hand — Walkthrough (React + shadcn)

This is the pattern for `web/` (React + Vite + shadcn/Tailwind, see `web/package.json`) today — there is no renderer yet, so *every* page is "manual" in the sense `docs/spec/05-frontend.md` §7 means for `asset` components. This will keep working once/if a renderer exists, since it's the same underlying client.

**1. Generate, once per manifest change:**

```bash
forma generate --spec ./spec --out web/src/generated/forma-client.ts
```

**2. Wire the client once** (e.g. `web/src/lib/forma.ts`):

```ts
import { FormaClient } from "@forma/client";
import { createApi } from "@/generated/forma-client";
import { useAuthStore } from "@/lib/auth-store"; // wherever your token lives

export const formaClient = new FormaClient({
  baseUrl: import.meta.env.VITE_FORMA_API_URL,
  workspace: import.meta.env.VITE_FORMA_WORKSPACE,
  getToken: () => useAuthStore.getState().token,
});

export const api = createApi(formaClient);
```

**3. A list page** — no data-fetching library required (though `@tanstack/react-query` — not currently a `web/` dependency — is a natural fit if you want caching/revalidation; the client returns plain Promises either way):

```tsx
import { useEffect, useState } from "react";
import { api } from "@/lib/forma";
import type { FormaRecord } from "@forma/client";
import type { BillingInvoice } from "@/generated/forma-client";
import { Button } from "@/components/ui/button";

export function InvoiceListPage() {
  const [invoices, setInvoices] = useState<FormaRecord<BillingInvoice>[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.billing.invoices.list({ perPage: 20 })
      .then((result) => setInvoices(result.data))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div>Loading…</div>;

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
  );
}
```

**4. A create form** (with `react-hook-form` + `zod`, both already in `web/package.json`):

```tsx
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { api } from "@/lib/forma";
import { FormaApiError } from "@forma/client";
import { Button } from "@/components/ui/button";

const schema = z.object({
  customer_id: z.string().min(1),
  total: z.string().min(1), // decimal fields are strings — see §3
});

export function CreateInvoiceForm({ onCreated }: { onCreated: (id: string) => void }) {
  const form = useForm({ resolver: zodResolver(schema) });

  async function onSubmit(values: z.infer<typeof schema>) {
    try {
      const invoice = await api.billing.invoices.create({ ...values, status: "draft" });
      onCreated(invoice.id);
    } catch (e) {
      if (e instanceof FormaApiError && e.isValidation) {
        e.details?.forEach((d) => form.setError(d.field as any, { message: d.message }));
      }
    }
  }

  return (
    <form onSubmit={form.handleSubmit(onSubmit)}>
      {/* fields bound via form.register(...) */}
      <Button type="submit">Create</Button>
    </form>
  );
}
```

**5. A custom action button:**

```tsx
async function handleApprove(id: string) {
  try {
    await api.billing.invoices.approve(id, { note: "" });
    // refetch or optimistically update
  } catch (e) {
    if (e instanceof FormaApiError && e.isForbidden) {
      // hide/disable the button instead of showing this error next time —
      // permission-driven UI is still the caller's responsibility (§ D38,
      // docs/spec/05-frontend.md §1.4) until a renderer derives it for you
    }
  }
}
```

**Permissions today:** unlike the eventual renderer (`docs/spec/05-frontend.md` §1.4, D38), nothing here hides a button for a caller who lacks permission — the server enforces it (a `403 FORBIDDEN` `FormaApiError`), but the *UI* decision to hide/disable is on you until that piece exists. Don't rely on hiding controls as a security boundary — it never was one (D38 confused-deputy rationale applies here too).

---

## 6. What This Does *Not* Give You

Explicitly out of scope for `@forma/client` + `forma generate` (all part of the not-yet-built renderer, `docs/spec/05-frontend.md`):

- No `kind: Page`/`Form`/`Table` interpretation — you lay out the page yourself.
- No `forma.ui` (toast/dialog/confirm/drawer), `forma.subscribe` (realtime), `forma.navigate`, `forma.form()` headless engine, or `forma.files` transfer manager.
- No permission-driven visibility — you decide what to hide based on `FormaApiError.isForbidden` or by fetching the caller's permission list yourself (no generated helper for that today).
- No `sort=`/`filter[field][op]=` query support — not wired server-side (§4).

---

## 7. Status Implementasi Hari Ini

**Implemented:**
- `@forma/client` runtime (`sdk/browser`) — `FormaClient` with `list/find/create/update/delete/action`, `FormaApiError` with `.isConflict/.isValidation/.isForbidden`, zero runtime dependencies (native `fetch`).
- `forma generate --lang typescript` (`cmd/forma/generate.go`) — reads manifests via the same registry/route-generation code the server uses, emits per-entity interfaces + a typed `createApi(client)` wrapper.
- Fixed as a prerequisite: `internal/db.EntityRecord` had no JSON tags at all, so the real wire format was PascalCase keys with entity fields nested under a `"Data"` key — not what §16 specifies and not what any client (generated or hand-written) could reasonably expect. `EntityRecord.MarshalJSON` (`internal/db/crud.go`) now flattens fields and uses snake_case, matching the spec. No test previously exercised the full HTTP response path, so this had gone uncaught.

**Not implemented:**
- `--lang go` and `--lang openapi` (`docs/cli-tools/01-forma-cli.md` §5 mentions both; only TypeScript exists).
- Everything in `docs/spec/05-frontend.md` beyond §7's client contract (the manifest-driven renderer itself, kinds, admin panel derivation).
- `sort`/`filter[field][op]` query parameters (declared in §16, not parsed by `internal/api/handler.go`).
- Decimal-as-string is the generated *type*, not yet an enforced *wire* guarantee — `internal/db` doesn't coerce decimal values before JSON-encoding them.
- Dart codegen (mentioned alongside TypeScript in `docs/spec/05-frontend.md` §7.5 for mobile/unmanaged clients).

---

## 8. References

| Document | Content |
|---|---|
| `docs/spec/02-core-basic.md` §16, §23 | Normative REST contract; `forma generate` verb |
| `docs/spec/05-frontend.md` §7 | `forma.api`/`forma.ui`/`forma.form()` full contract (aspirational renderer context) |
| `docs/implementation/api-layer.md` | `internal/api` package internals, response envelope |
| `sdk/README.md` | The other client family — `lib-forma-*` for `forma-sidecar` (a different protocol; don't confuse the two) |
