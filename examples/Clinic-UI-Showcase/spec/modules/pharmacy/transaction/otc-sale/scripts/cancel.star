# modules/pharmacy/scripts/otc_cancel.star

def execute(resource, params, ctx):
    resource.set("status", "cancelled")
    resource.save()
    ctx.log.info("otc_sale.cancelled", {"otc_sale_id": resource.id})
    return ok()
