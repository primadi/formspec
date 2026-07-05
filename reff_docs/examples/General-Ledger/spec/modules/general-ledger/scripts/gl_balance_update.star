# modules/general-ledger/scripts/gl_balance_update.star

def execute(resource, params, ctx):
    # Dipanggil oleh outbox worker via reliable_event.
    # Idempotent: kalau sudah ada balance untuk (account, period, currency),
    # update; kalau belum, create.

    journal = journal_entry.load(params.id)
    period = str(journal.field.entry_date)[:7]  # "2026-07"

    for line in journal.field.lines:
        account = line.account_id
        existing = gl_balance.query() \
            .where("account_id", account) \
            .where("period", period) \
            .where("currency", journal.field.currency) \
            .first()

        if existing:
            existing.set("debit_movement",
                existing.field.debit_movement + line.debit)
            existing.set("credit_movement",
                existing.field.credit_movement + line.credit)
            existing.save()
        else:
            gl_balance.new() \
                .set("account_id", account) \
                .set("period", period) \
                .set("currency", journal.field.currency) \
                .set("debit_movement", line.debit) \
                .set("credit_movement", line.credit) \
                .save()

    ctx.log.info("gl_balance.updated", {
        "journal_id": journal.id,
        "period": period,
    })
    return ok()
