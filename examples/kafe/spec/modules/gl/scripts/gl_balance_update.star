# ─────────────────────────────────────────────────────────────────────────────
# PEMELIHARA: proyeksi `gl-balance` (saldo per akun per periode)
# ─────────────────────────────────────────────────────────────────────────────
#
# Dipanggil oleh outbox worker via reliable_event (`deliver: {target:
# gl.gl-balance.update}`) untuk event journal-posted / journal-reversed —
# params-nya payload event: {id, number, entry_date, source, source_id,
# currency} (lihat events: pada journal-entry).
#
# Entity ini `characteristic: summary` → SATU-SATUNYA jalur tulis adalah
# `resource.upsert`, dan hanya script yang disebut `maintained_by` yang
# diizinkan (item 4.1). `resource.fetch`/`resource.find` membaca lewat lapisan
# entity (bukan SQL); API `gl_balance.query()...` dulu ditulis aspirasional di
# vertical dan tidak pernah bisa dikompilasi — honesty check kafe yang
# menangkapnya (verifikasi PG/SQLite 2026-09-21).
# ─────────────────────────────────────────────────────────────────────────────


def execute(resource, params, ctx):
    journal = resource.fetch("gl.journal-entry", params.get("id"))
    if journal == None:
        fail("FORMSPEC.GL.JOURNAL_NOT_FOUND: event jurnal menyebut id " + str(params.get("id")) + " yang tidak ada")

    period = str(journal.field.entry_date)[:7]
    currency = journal.field.currency or "IDR"

    # Arah penerapan ditentukan status jurnal, bukan nama event: event
    # `journal-posted` dan `journal-reversed` sama-sama menargetkan action
    # `update` (satu-satunya script pemelihara proyeksi ini — `maintained_by`
    # hanya menyebut satu ref, jadi script kedua tidak akan diizinkan
    # `resource.upsert`). Jurnal `reversed` MENGURANGI pergerakannya.
    sign = 1.0
    if journal.field.status == "reversed":
        sign = -1.0

    lines = journal.field.lines or []
    for line in lines:
        is_dict = type(line) == "dict"
        account_id = line["account_id"] if is_dict else line.account_id
        debit = float((line["debit"] if is_dict else line.debit) or 0) * sign
        credit = float((line["credit"] if is_dict else line.credit) or 0) * sign

        # Akun dibaca DI LUAR cabang: tanpa ini, iterasi pertama (baris belum
        # ada → cabang insert) memakai `debit - credit` mentah yang mengabaikan
        # arah saldo normal, sehingga akun kredit-normal (omzet, pajak,
        # liabilitas) tercatat negatif di pembuatan baris pertamanya.
        acc = resource.fetch("gl.account", account_id)
        if acc == None:
            fail("FORMSPEC.GL.ACCOUNT_NOT_FOUND: baris jurnal menunjuk akun " + str(account_id) + " yang tidak ada di bagan akun")

        match = {"account_id": account_id, "period": period, "currency": currency}
        existing = resource.find("gl.gl-balance", match)

        opening = 0.0
        d = 0.0
        c = 0.0
        if existing != None:
            opening = float(existing.field.opening_balance or 0)
            d = float(existing.field.debit_movement or 0)
            c = float(existing.field.credit_movement or 0)

        d = d + debit
        c = c + credit

        # Saldo berjalan selalu diukur pada arah saldo normal akun — itu yang
        # membuat omzet (+125.000) dan kas (+143.750) sama-sama positif, bukan
        # cerminan satu sama lain. Arahnya diambil dari bagan akun: modul gl
        # yang tahu, bukan pemanggil event.
        if acc.field.normal_balance == "debit":
            closing = opening + d - c
        else:
            closing = opening + c - d

        resource.upsert("gl.gl-balance", match, {
            "opening_balance": opening,
            "debit_movement": d,
            "credit_movement": c,
            "closing_balance": closing,
        })

    ctx.log.info("gl_balance.updated", {"journal_id": journal.id, "period": period, "status": journal.field.status})
    return ok()
