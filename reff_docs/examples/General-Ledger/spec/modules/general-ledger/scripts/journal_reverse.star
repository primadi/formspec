# modules/general-ledger/scripts/journal_reverse.star

def execute(resource, params, ctx):
    # Reverse journal entry yang sudah posted.
    # Transition dari posted → reversed + tulis outbox journal-reversed.
    resource.set("status", "reversed")
    resource.save()
    ctx.log.info("journal.reversed", {
        "journal_id": resource.id,
        "number": resource.field.number,
    })
    return ok()
