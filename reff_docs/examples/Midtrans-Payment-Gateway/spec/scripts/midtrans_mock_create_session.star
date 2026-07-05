# modules/billing/scripts/midtrans_mock_create_session.star

def execute(resource, params, ctx):
    # HANYA dipanggil saat mock_enabled: true (framework routing)
    import uuid

    fake_transaction_id = "mock-" + str(uuid.uuid4())[:8]
    fake_payment_url = "https://mock.local/pay/" + fake_transaction_id

    ctx.log.info("midtrans.mock.session_created", {
        "order_id": params.order_id,
        "transaction_id": fake_transaction_id,
    })

    return ok({
        "payment_url": fake_payment_url,
        "transaction_id": fake_transaction_id,
    })
