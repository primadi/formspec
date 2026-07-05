# modules/inventory/scripts/movement_apply.star

def execute(resource, params, ctx):
    # FR1 — lock mencegah race condition: dua kasir kurangi
    # stok bersamaan. Lock diambil untuk setiap produk di movement.
    locked_keys = []
    for line in resource.field.lines:
        key = "stock:" + line.product_id + ":" + resource.field.warehouse_id
        ctx.lock.acquire(key, ttl=10)
        locked_keys.append(key)

    # Semua lock didapat → cek apakah stok cukup
    for line in resource.field.lines:
        level = stock_level.query() \
            .where("product_id", line.product_id) \
            .where("warehouse_id", resource.field.warehouse_id) \
            .first()

        if resource.field.type == "out":
            available = level.field.quantity_available if level else 0
            if available < line.quantity:
                for k in locked_keys:
                    ctx.lock.release(k)
                return fail("Stok tidak cukup untuk produk " + line.product_id)

    # Stok cukup → transisi status + tulis outbox stock-applied
    resource.set("status", "applied")
    resource.save()

    ctx.log.info("movement.applied", {
        "movement_id": resource.id,
        "number": resource.field.number,
        "type": resource.field.type,
    })
    return ok()
