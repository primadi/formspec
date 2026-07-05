# modules/billing/scripts/midtrans_mock_webhook.star

def execute(resource, params, ctx):
    order.call("mark-paid", {
        "gateway_reference": params.transaction_id,
        "event_id": params.transaction_id,
    })

    ctx.log.info("midtrans.mock.webhook_processed", {
        "transaction_id": params.transaction_id,
    })

    return ok({"status": "processed"})
