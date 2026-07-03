# modules/inventory/scripts/stock_level_update.star

def execute(resource, params, ctx):
    # Dipanggil oleh outbox worker via reliable_event.
    movement = stock_movement.load(params.id)

    for line in movement.field.lines:
        key = "stock:" + line.product_id + ":" + movement.field.warehouse_id
        ctx.lock.acquire(key, ttl=5)

        level = stock_level.query() \
            .where("product_id", line.product_id) \
            .where("warehouse_id", movement.field.warehouse_id) \
            .first()

        if not level:
            level = stock_level.new() \
                .set("product_id", line.product_id) \
                .set("warehouse_id", movement.field.warehouse_id) \
                .set("quantity_on_hand", 0) \
                .set("quantity_available", 0)

        if movement.field.type == "in":
            level.set("quantity_on_hand",
                level.field.quantity_on_hand + line.quantity)
        elif movement.field.type == "out":
            level.set("quantity_on_hand",
                level.field.quantity_on_hand - line.quantity)

        level.set("quantity_available",
            level.field.quantity_on_hand - level.field.quantity_reserved)
        level.set("last_movement_at", ctx.now())
        level.save()

        ctx.lock.release(key)

    ctx.log.info("stock_level.updated", {
        "movement_id": movement.id,
        "lines": len(movement.field.lines),
    })
    return ok()
