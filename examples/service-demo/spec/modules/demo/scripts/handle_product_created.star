# Handler untuk subscription demo.product.on_create (dipanggil oleh outbox
# worker setelah event on_create product di-enqueue secara durable).
# Payload event diterima sebagai params; metadata event ada di params["_event"].
def execute(resource, params, ctx):
    ev = params.get("_event", {})
    product_name = params.get("name", "unknown")
    sku = params.get("sku", "unknown")
    ctx.log.info("product_created_subscription", {
        "event": ev.get("name", ""),
        "resource": ev.get("resource", ""),
        "product": product_name,
        "sku": sku,
    })
    return ok({
        "handled": True,
        "event": ev.get("name", ""),
        "product": product_name,
        "sku": sku,
    })