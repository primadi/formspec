// Illustrative example of the generic FormaClient API — the shape
// docs/cli-tools/03-formspec-generate.md §5 builds a full React page around.
// This file exercises the client directly (no generated types) so it's
// useful even before running `formspec generate`.

import { FormaApiError, FormaClient, FormaRecord } from "../src/index.js";

// A hand-written stand-in for what `formspec generate` would emit for this
// entity — this example intentionally shows the client used *without*
// codegen, but still needs a type to get typed ids/fields back.
interface Invoice {
  customer_id: string;
  status: string;
  total: string;
}

const client = new FormaClient({
  baseUrl: process.env.FORMA_API_URL ?? "http://localhost:8080",
  workspace: process.env.FORMA_WORKSPACE ?? "demo",
  getToken: () => process.env.FORMA_TOKEN,
});

async function main() {
  // List, with pagination + search — the only query options wired
  // server-side today (docs/cli-tools/03-formspec-generate.md §4).
  const page = await client.list("billing", "invoices", { perPage: 10, search: "acme" });
  console.log(`${page.total} invoice(s), page ${page.page}/${page.totalPages}`);

  // Create — server assigns id/version/timestamps. FormaRecord<Invoice>
  // gives typed access to both the reserved columns (id, version, ...)
  // and this entity's own fields — see §4 of the guide.
  const invoice = await client.create<FormaRecord<Invoice>>("billing", "invoices", {
    customer_id: "cust-1",
    status: "draft",
    total: "150000.00", // decimal fields are strings — never risk float precision
  });
  console.log("created", invoice.id);

  // Update — PATCH body is only the changed fields; no version/CAS token
  // to send, the server tracks that internally.
  await client.update<FormaRecord<Invoice>>("billing", "invoices", invoice.id, { total: "175000.00" });

  // Custom action.
  try {
    await client.action("billing", "invoices", invoice.id, "approve", { note: "looks good" });
  } catch (e) {
    if (e instanceof FormaApiError) {
      if (e.isValidation) console.error("validation failed:", e.details);
      else if (e.isForbidden) console.error("not permitted to approve");
      else if (e.isConflict) console.error("invoice changed concurrently — refetch and retry");
      else throw e;
    } else {
      throw e;
    }
  }

  await client.delete("billing", "invoices", invoice.id);
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
