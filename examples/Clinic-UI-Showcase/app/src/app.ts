/**
 * Clinic UI Showcase — TypeScript sidecar app entrypoint.
 *
 * Registers business-logic handlers for sidecar actions and starts the
 * App listener on FORMA_APP_SOCKET (unix:///tmp/forma/app.sock by default).
 *
 * forma-sidecar (or forma dev --runtime node) will:
 *   1. Spawn this app as a child process
 *   2. Call POST /invoke/pharmacy/otc-sale/sell when the sell action triggers
 *   3. Forward ctx.* primitive calls back to the engine over FORMA_SIDECAR_SOCKET
 */

import { App } from "@forma/lib-forma";
import { registerSellHandler } from "./handlers/otc_sell";

const app = new App();
registerSellHandler(app);

app.run().then((server) => {
  const addr = server.address();
  process.stderr.write(`[clinic-showcase] ready on ${JSON.stringify(addr)}\n`);
});
