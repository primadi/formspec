# modules/pharmacy/scripts/rx_start_compounding.star

def execute(resource, params, ctx):
    resource.set("status", "compounding")
    resource.save()
    ctx.log.info("prescription.compounding", {"prescription_id": resource.id})
    return ok()
