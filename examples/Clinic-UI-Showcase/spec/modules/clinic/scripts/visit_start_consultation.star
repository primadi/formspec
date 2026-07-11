# modules/clinic/scripts/visit_start_consultation.star

def execute(resource, params, ctx):
    resource.set("status", "in_consultation")
    resource.set("started_at", ctx.now())
    resource.save()
    ctx.log.info("visit.started", {"visit_id": resource.id})
    return ok()
