# modules/pharmacy/scripts/rx_mark_ready.star

def execute(resource, params, ctx):
    resource.set("status", "ready")
    resource.save()
    ctx.log.info("prescription.ready", {"prescription_id": resource.id})
    return ok()
