// Example lib-formspec (TypeScript) app: the business-logic side of an
// impl: {type: sidecar} action. Run inside a pod next to formspec-sidecar:
//
//   FORMA_APP_SOCKET=/tmp/formspec/app.sock \
//   FORMA_SIDECAR_SOCKET=/tmp/formspec/sidecar.sock \
//   npx ts-node examples/app.ts

import { ActionResult, App } from "@formspec/lib-formspec";

const app = new App();

app.handle("billing.invoice.approve", async (inv, ctx) => {
  const lockKey = `invoice:${inv.resourceId}`;
  if (!(await ctx.lock().acquire(lockKey, 30))) {
    throw new Error("invoice is being processed by someone else");
  }

  try {
    if (inv.resource.status !== "draft") {
      throw new Error("only draft invoices can be approved");
    }

    return new ActionResult(
      { approved_at: new Date().toISOString(), note: inv.params.note ?? "" },
      "approved",
    ).withEvent("invoice.approved", { id: inv.resourceId }, true);
  } finally {
    await ctx.lock().release(lockKey);
  }
});

app.run();
