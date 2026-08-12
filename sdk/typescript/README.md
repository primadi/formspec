# @formspec/lib-formspec (TypeScript)

Thin Node.js/TypeScript client for `formspec-sidecar`
(docs/runtimes/04-formspec-sidecar.md). Node ≥ 18, no runtime dependencies.

```bash
npm install @formspec/lib-formspec
```

```ts
import { ActionResult, App } from "@formspec/lib-formspec";

const app = new App(); // sockets from FORMA_APP_SOCKET / FORMA_SIDECAR_SOCKET

app.handle("billing.invoice.approve", async (inv, ctx) => {
  const rows = await ctx.db().query("SELECT ...");            // proxied to the sidecar engine
  await ctx.cache().named("session-cache").get("key");        // named datastore

  return new ActionResult(
    { approved_at: new Date().toISOString() },
    "approved",
  ).withEvent("invoice.approved", { id: inv.resourceId });
});

app.run(); // sidecar calls POST /invoke/billing/invoice/approve
```

Handlers may return an `ActionResult`, plain data (becomes `data`), or
throw — errors surface to the sidecar as HTTP 500 with the message.

See [examples/app.ts](examples/app.ts) for a runnable app, and
[../README.md](../README.md) for the wire contract shared by all SDKs.
