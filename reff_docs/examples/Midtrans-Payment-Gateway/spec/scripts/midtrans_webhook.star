# modules/billing/scripts/midtrans_webhook.star
#
# PERHATIAN: Script ini adalah STUB.
# Lihat impl/billing/midtrans.go → PaymentGateway.Webhook
# Framework SUDAH verifikasi signature (kind: Webhook) + idempotency SEBELUM handler jalan.

def execute(resource, params, ctx):
    # Framework SUDAH verifikasi signature (kind: Webhook).
    # Framework SUDAH menolak duplikat transaction_id (idempotent: true).
    # Handler hanya meneruskan ke order.mark-paid.

    order.call("mark-paid", {
        "gateway_reference": params.transaction_id,
        "event_id": params.transaction_id,
    })

    ctx.log.info("midtrans.webhook_processed", {
        "transaction_id": params.transaction_id,
        "status": params.transaction_status,
    })

    return ok({"status": "processed"})
