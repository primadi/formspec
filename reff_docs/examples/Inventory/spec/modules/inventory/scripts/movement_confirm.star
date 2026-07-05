# modules/inventory/scripts/movement_confirm.star

def execute(resource, params, ctx):
    # Konfirmasi movement — transition draft → confirmed.
    # Belum mempengaruhi stok, hanya status change.
    resource.set("status", "confirmed")
    resource.save()
    ctx.log.info("movement.confirmed", {
        "movement_id": resource.id,
        "number": resource.field.number,
        "type": resource.field.type,
    })
    return ok()
