/**
 * OTC Sell handler — TypeScript demo of a sidecar action.
 *
 * Mirrors the logic of otc_sell.star but runs in a Node.js child process,
 * communicating with the formspec engine over a unix domain socket.
 *
 * Key difference from Starlark:
 *   - Uses ctx.entity().named("module/entity").decrement() instead of
 *     resource.fetch() + modify + resource.save() — the decrement is
 *     ATOMIC (single SQL jsonb_set), eliminating read-modify-write race
 *     conditions on concurrent requests.
 */

import { ActionResult, App } from "@formspec/lib-formspec";

export function registerSellHandler(app: App): void {
  app.handle("pharmacy.otc-sale.sell", async (inv, ctx) => {
    let total = 0;
    const items: any[] = (inv.resource as any)?.items ?? [];

    for (const item of items) {
      // Atomic decrement — single SQL statement, no race condition.
      // Equivalent to Starlark's:
      //   med = resource.fetch("medicine", item["medicine_id"])
      //   med.set("stock", med.field.stock - item["quantity"])
      //   med.save()
      await ctx
        .entity()
        .named("pharmacy/medicine")
        .decrement(item.medicine_id, "stock", item.quantity);

      total += item.quantity * item.unit_price;
    }

    // Return computed total + new state — the engine applies the state
    // transition (pending → completed) and merges the data.
    return new ActionResult({ total }, "completed");
  });
}
