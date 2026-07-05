# modules/general-ledger/scripts/journal_post.star

def execute(resource, params, ctx):
    # Guard state machine sudah memvalidasi debit == credit sebelum jalan.
    # Transition dari draft → posted + tulis outbox journal-posted
    # dalam satu transaksi DB.
    resource.set("status", "posted")
    resource.save()
    ctx.log.info("journal.posted", {
        "journal_id": resource.id,
        "number": resource.field.number,
    })
    return ok()
