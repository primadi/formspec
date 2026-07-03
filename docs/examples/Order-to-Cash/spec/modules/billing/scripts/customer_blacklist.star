# modules/billing/scripts/customer_blacklist.star

def execute(resource, params, ctx):
    resource.set("is_blacklisted", True)
    resource.save()
    ctx.log.info("customer.blacklisted", {"customer_id": resource.id})
    return ok()
