# modules/billing/scripts/order_mark_paid.star

def execute(resource, params, ctx):
    # FR3 sudah beres SEBELUM baris ini jalan: framework menolak duplikat
    # event_id dan me-replay response asli (idempotent: true).
    resource.set("gateway_reference", params.gateway_reference)
    resource.set("paid_at", ctx.now())
    resource.save()   # transisi awaiting_payment→paid + tulis outbox event
                      # "paid" — SATU transaksi DB
    return ok()
