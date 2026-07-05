# modules/billing/scripts/order_checkout.star

def execute(resource, params, ctx):
    # Blacklist check TIDAK di sini — dia precondition, rumahnya di
    # blok `conditions` action sehingga terbaca di kontrak.
    # Yang tersisa di script murni PROSEDUR: langkah berurutan.

    # FR1 — rule ada di field (natural_key_rule); ctx.next_key membaca rule itu,
    # menangani lock + reset period + format. Tidak pernah MAX()+1 manual.
    number = ctx.next_key("number")
    resource.set("number", number).save()

    # FR2 — gateway dipanggil HANYA lewat Service wrapper yang dideklarasikan
    # di uses.resources. Saat dev: otomatis ke mockup; saat prod: konektor asli.
    session = payment_gateway.call("create-session", {
        "order_id": resource.id,
        "amount": resource.field.total,
    })

    ctx.log.info("order.checkout", {"order_id": resource.id, "number": number})
    return ok({"payment_url": session.payment_url})
