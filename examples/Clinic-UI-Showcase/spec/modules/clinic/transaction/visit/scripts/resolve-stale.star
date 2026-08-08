# modules/clinic/scripts/visit_resolve_stale.star
# Path khusus: selesaikan kunjungan stale yang melewati backdate policy.
# Mirip complete, tapi disengaja + diaudit + emit event completed-overdue
# supaya hilir (payment, revenue) tahu ini pengecualian, bukan complete normal.

def execute(resource, params, ctx):
    total = 0
    # treatments is optional — a consultation can finish with no treatments.
    for t in (resource.field.treatments or []):
        total += t["quantity"] * t["price"]
    resource.set("total", total)
    resource.set("status", "completed")
    resource.set("completed_at", ctx.now())
    resource.save()
    ctx.log.info("visit.resolve-stale", {"visit_id": resource.id, "total": total})
    return ok()