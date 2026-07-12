# modules/clinic/scripts/visit_complete.star

def execute(resource, params, ctx):
    total = 0
    for t in resource.field.treatments:
        total += t["quantity"] * t["price"]
    resource.set("total", total)
    resource.set("status", "completed")
    resource.set("completed_at", ctx.now())
    resource.save()
    ctx.log.info("visit.completed", {"visit_id": resource.id, "total": total})
    return ok()
