# modules/billing/scripts/customer_update_tier.star

def execute(resource, params, ctx):
    old_tier = resource.field.member_tier
    resource.set("member_tier", params.member_tier)
    resource.save()
    ctx.log.info("customer.tier_updated", {
        "customer_id": resource.id,
        "old_tier": old_tier,
        "new_tier": params.member_tier,
    })
    return ok()
