"""Example lib-formspec-python app: the business-logic side of an
impl: {type: sidecar} action. Run inside a pod next to formspec-sidecar:

    FORMA_APP_SOCKET=/tmp/formspec/app.sock \
    FORMA_SIDECAR_SOCKET=/tmp/formspec/sidecar.sock \
    python examples/app.py
"""

from datetime import datetime, timezone

from lib_formspec import ActionResult, App, Ctx, Invocation

app = App()


@app.action("billing.invoice.approve")
def approve(inv: Invocation, ctx: Ctx) -> ActionResult:
    if not ctx.lock().acquire(f"invoice:{inv.resource_id}", ttl_seconds=30):
        raise RuntimeError("invoice is being processed by someone else")

    try:
        if inv.resource.get("status") != "draft":
            raise RuntimeError("only draft invoices can be approved")

        return ActionResult(
            data={
                "approved_at": datetime.now(timezone.utc).isoformat(),
                "note": inv.params.get("note", ""),
            },
            new_state="approved",
        ).with_event("invoice.approved", {"id": inv.resource_id}, durable=True)
    finally:
        ctx.lock().release(f"invoice:{inv.resource_id}")


if __name__ == "__main__":
    app.run()
