# Handler untuk subscription durable demo.product.on_create (Tier 2 streaming,
# todo 7.3.2). Dipanggil oleh StreamingWorker setelah event di-append ke stream
# dan melewati filter/transform Starlark. Payload yang diterima sudah hasil
# transform subscription (field name/sku/min_stock + source).
def execute(resource, params, ctx):
    ctx.log.info("product_stream_subscription", {
        "product": params.get("name", "unknown"),
        "sku": params.get("sku", "unknown"),
        "min_stock": params.get("min_stock", 0),
        "source": params.get("source", ""),
    })
    return ok({
        "handled": True,
        "product": params.get("name", ""),
        "sku": params.get("sku", ""),
        "source": params.get("source", ""),
    })