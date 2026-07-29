# modules/clinic/scripts/visit_cancel.star

def execute(resource, params, ctx):
    resource.set("status", "cancelled")
    resource.save()
    ctx.log.info("visit.cancelled", {"visit_id": resource.id})
    return ok()
