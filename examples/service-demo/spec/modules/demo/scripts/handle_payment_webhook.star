# Handler untuk webhook payment gateway (dipanggil setelah verifikasi HMAC).
# Payload sudah terverifikasi oleh framework sebelum script ini berjalan.
# Payload webhook diterima sebagai params.
def execute(resource, params, ctx):
    txn_id = params.get("transaction_id", "unknown")
    amount = params.get("amount", 0)
    status = params.get("status", "unknown")
    ctx.log.info("payment_webhook_received", {
        "transaction_id": txn_id,
        "amount": amount,
        "status": status,
    })
    return ok({
        "received": True,
        "transaction_id": txn_id,
        "status": status,
    })
