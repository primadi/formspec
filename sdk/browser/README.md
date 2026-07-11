# @forma/client

Typed browser/Node runtime for Forma's generated REST API
(docs/spec/02-core-basic.md §16). This is the hand-written half of
`forma.api` (docs/spec/05-frontend.md §7) — `forma generate --lang
typescript` (`cmd/forma/generate.go`) emits the typed half on top of it.
Zero runtime dependencies (native `fetch`), Node ≥ 18 or any modern browser.

**Full guide (installation, codegen walkthrough, a React + shadcn
example):** `docs/cli-tools/03-forma-generate.md`.

```bash
npm install @forma/client   # or, until published: npm install <path-to-sdk/browser>
```

```ts
import { FormaClient, FormaApiError } from "@forma/client";
import { createApi } from "./generated/forma-client"; // forma generate output

const client = new FormaClient({
  baseUrl: "https://api.example.com",
  workspace: "acme",
  getToken: () => localStorage.getItem("forma_token") ?? undefined,
});
const api = createApi(client);

const { data: invoices } = await api.billing.invoices.list({ perPage: 20 });

try {
  await api.billing.invoices.approve(invoices[0].id, { note: "ok" });
} catch (e) {
  if (e instanceof FormaApiError && e.isConflict) {
    // 409 — someone else edited it first, refetch and retry
  }
}
```

Without generated types, the same generic methods work directly (useful
before running codegen, or for an entity it hasn't been run for yet):

```ts
await client.list("billing", "invoices", { search: "acme" });
await client.action("billing", "invoices", id, "approve", { note: "" });
```

See [examples/manual-page.ts](examples/manual-page.ts) for a fuller,
runnable-shape example.
