# modules/gl/scripts/journal_post.star

def execute(resource, params, ctx):
    resource.set("status", "posted")
    resource.save()
    ctx.log.info("journal.posted", {
        "journal_id": resource.id,
        "number": resource.field.number,
    })
    return ok()
