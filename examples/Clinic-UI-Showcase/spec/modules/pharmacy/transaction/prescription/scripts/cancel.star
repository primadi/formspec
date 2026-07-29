# modules/pharmacy/scripts/rx_cancel.star

def execute(resource, params, ctx):
    resource.set("status", "cancelled")
    resource.save()
    ctx.log.info("prescription.cancelled", {"prescription_id": resource.id})
    return ok()
