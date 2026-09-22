# ─────────────────────────────────────────────────────────────────────────────
# SUBSCRIPTION HANDLER: penjualan kafe → jurnal double-entry
# ─────────────────────────────────────────────────────────────────────────────
#
# Dipanggil oleh `subscriptions/sales-to-journal.yaml` (kind: Subscription milik
# module gl) untuk dua event dari module cafe-order:
#
#   cafe-order.order.on_paid    → buat + posting jurnal penjualan
#   cafe-order.order.on_cancel  → balikkan jurnal yang sudah diposting
#
# PEMISAHAN TANGGUNG JAWAB (keputusan pemilik, 2026-09-21):
#   * module cafe-order hanya MEMANCARKAN event durable (`on_paid`/`on_cancel`,
#     publish.durable → outbox + retry + dead-letter). Ia tidak tahu kode akun.
#   * module gl memiliki jurnal, bagan akun, dan SETTING pemetaannya
#     (config/gl.yaml: gl_journal_account_*). Jurnal tidak seimbang berarti
#     setting akun gl belum lengkap — errornya FORMSPEC.GL.*, bukan error
#     module pemesanan.
#
# Invarian yang menopang:
#   * journal-entry.lines child punya `exists: account` → akun yang ditulis
#     WAJIB ada di bagan akun (ditegakkan database).
#   * action `post` punya guard `sum_line('debit') == sum_line('credit')` —
#     DB-lah yang menegakkan keseimbangan, script ini hanya lapis pertama yang
#     memberi pesan yang menunjuk setting yang kurang.
#
# `resource.find` (4.3) dipakai untuk membaca bagan akun lewat lapisan entity
# (tenant isolation & row_scope ikut berlaku) — bukan raw SQL.
# ─────────────────────────────────────────────────────────────────────────────


def currency_of(value, default):
    """Mata uang dari nilai money, atau default bila nilainya kosong."""
    if value == None:
        return default
    return currency(value)


def money_of(value):
    """Angka dari nilai money ({amount, currency}) atau skalar. None → 0."""
    if value == None:
        return 0.0
    return float(amount(value))


def decimal_like(x):
    """Bentuk decimal untuk baris jurnal: bilangan bulat tanpa `.0`."""
    if x == int(x):
        return int(x)
    return x


def account_id_for(resource, ctx, role, code_setting, code_default):
    """ID record akun untuk satu peran (kas/omzet/pajak/...).

    Kunci setting kosong = komponen itu BELUM DIATUR → return None (pemanggil
    memutuskan: jika komponennya nol boleh dilewati, jika tidak nol → fail).
    """
    code = ctx.config.get(code_setting)
    if code == None or code == "":
        return None, code_setting
    acc = resource.find("gl.account", {"code": code})
    if acc == None:
        msg = "FORMSPEC.GL.ACCOUNT_NOT_FOUND: setting akun '" + role + "' menunjuk kode '" + code + "' yang tidak ada di bagan akun - lengkapi chart of accounts atau perbaiki kunci gl_journal_account_" + role + " di Config gl"
        fail(msg)
    # `acc.id` = identitas record (kolom tabel, bukan field data). `acc.field.id`
    # SELALU None: membaca field yang tidak ada mengembalikan None, bukan error —
    # jadi `hasattr(acc.field, "id")` tidak pernah bisa jadi pembeda.
    return acc.id, None


def build_lines(resource, params, ctx):
    """Susun baris jurnal dari payload pesanan + setting akun gl.

    Kas (debit)   = total_amount — uang yang diterima usaha.
    Omzet (kredit)= subtotal — pendapatan.
    Pajak (kredit)= tax_amount.
    Service charge (kredit) & diskon (debit) hanya bila nilainya != 0.

    Komponen bernilai ≠ 0 tanpa setting akun = jurnal tidak seimbang = fail
    dengan FORMSPEC.GL.ACCOUNT_NOT_SET (tanggung jawab gl).
    """
    total = money_of(params.get("total_amount"))
    if total == 0.0:
        return None, "FORMSPEC.GL.NO_AMOUNT: pesanan %s lunas tanpa nilai" % params.get("number")

    kas_id, kas_setting = account_id_for(resource, ctx, "kas", "gl_journal_account_kas", None)
    if kas_id == None:
        fail("FORMSPEC.GL.ACCOUNT_NOT_SET: kunci gl_journal_account_kas belum diatur - jurnal tidak bisa dibangun (setting akun gl belum lengkap)")
    omzet_id, _ = account_id_for(resource, ctx, "omzet", "gl_journal_account_omzet", None)
    if omzet_id == None:
        fail("FORMSPEC.GL.ACCOUNT_NOT_SET: kunci gl_journal_account_omzet kosong (setting akun gl belum lengkap)")
    pajak_id, _ = account_id_for(resource, ctx, "pajak", "gl_journal_account_pajak", None)

    debit = 0.0
    credit = 0.0
    lines = []

    lines.append({
        "description": "Kas",
        "account_id": kas_id,
        "debit": decimal_like(total),
        "credit": 0,
    })
    debit += total

    subtotal = money_of(params.get("subtotal"))
    if subtotal > 0.0:
        lines.append({
            "description": "Omzet penjualan",
            "account_id": omzet_id,
            "debit": 0,
            "credit": decimal_like(subtotal),
        })
        credit += subtotal

    tax = money_of(params.get("tax_amount"))
    if tax != 0.0:
        if pajak_id == None:
            fail("FORMSPEC.GL.ACCOUNT_NOT_SET: pesanan " + str(params.get("number")) + " punya pajak " + str(tax) + " tapi kunci gl_journal_account_pajak kosong - jurnal tidak seimbang; setting akun gl belum lengkap")
        lines.append({
            "description": "Pajak penjualan",
            "account_id": pajak_id,
            "debit": 0,
            "credit": decimal_like(tax),
        })
        credit += tax

    service = money_of(params.get("service_charge_amount"))
    if service != 0.0:
        sc_id, sc_setting = account_id_for(resource, ctx, "service_charge", "gl_journal_account_service_charge", None)
        if sc_id == None:
            fail("FORMSPEC.GL.ACCOUNT_NOT_SET: pesanan " + str(params.get("number")) + " punya service charge " + str(service) + " tapi kunci gl_journal_account_service_charge kosong - setting akun gl belum lengkap")
        lines.append({
            "description": "Service charge",
            "account_id": sc_id,
            "debit": 0,
            "credit": decimal_like(service),
        })
        credit += service

    diskon = money_of(params.get("discount_amount")) + money_of(params.get("manual_discount_amount")) + money_of(params.get("points_value"))
    if diskon != 0.0:
        d_id, d_setting = account_id_for(resource, ctx, "diskon", "gl_journal_account_diskon", None)
        if d_id == None:
            fail("FORMSPEC.GL.ACCOUNT_NOT_SET: pesanan " + str(params.get("number")) + " punya diskon " + str(diskon) + " tapi kunci gl_journal_account_diskon kosong - setting akun gl belum lengkap")
        lines.append({
            "description": "Diskon penjualan",
            "account_id": d_id,
            "debit": decimal_like(diskon),
            "credit": 0,
        })
        debit += diskon

    # Belt and suspenders: komponen sudah lengkap, tapi kalau dua sisi tetap
    # tidak sama (mis. promo yang tidak terpetakan), tolak — jurnal tidak
    # seimbang tidak boleh sampai ke bagan.
    if debit != credit:
        fail("FORMSPEC.GL.UNBALANCED: jurnal tidak seimbang (debit " + str(decimal_like(debit)) + " != kredit " + str(decimal_like(credit)) + ") - setting akun gl belum lengkap untuk komponen pesanan ini")
    return lines, None


def execute(resource, params, ctx):
    event = params["_event"]["name"]

    if event == "cafe-order.order.on_paid":
        return journalize_sale(resource, params, ctx)
    return reverse_journal(resource, params, ctx)


def journalize_sale(resource, params, ctx):
    # Idempoten per source_id: pesanan yang sama tidak boleh jurnal dua kali
    # (retry outbox / pengiriman ganda tidak boleh menggandakan jurnal).
    existing = resource.find("gl.journal-entry", {"source_id": params.get("id")})
    if existing != None:
        return ok({"status": "already_journaled", "journal": existing.id})

    lines, err = build_lines(resource, params, ctx)
    if err != None:
        fail(err)

    if lines == None or len(lines) < 2:
        fail("FORMSPEC.GL.TOO_FEW_LINES: jurnal penjualan butuh minimal 2 baris")

    jid = resource.create("gl.journal-entry", {
        "transaction_date": params.get("transaction_date"),
        "entry_date": params.get("transaction_date"),
        "reference": params.get("number"),
        "source": "cafe-order.order",
        "source_id": params.get("id"),
        "description": "Penjualan " + str(params.get("number")),
        "currency": currency_of(params.get("total_amount"), "IDR"),
        "lines": lines,
    })

    # Posting di sini (bukan menunggu manusia): guard `post` menegakkan
    # keseimbangan di lapisan action juga — jurnal yang lolos pembuatan
    # tapi tetap tidak seimbang tidak akan bisa diposting.
    #
    # `resource.create` mengembalikan HANDLE record (bukan string id) — id-nya
    # lewat `jid.id`. Target `gl.journal-entry.<id>` = aksi pada SATU record
    # (bentuk yang sama dengan `resource.call("module.entity", ...)`, ditambah
    # segmen id terakhir).
    resource.call("gl.journal-entry." + str(jid.id), "post", {})
    return ok({"journal": jid.id, "status": "posted"})


def reverse_journal(resource, params, ctx):
    # `on_cancel` juga dipancarkan untuk pesanan yang belum lunas — jurnal
    # memang tidak ada → tidak ada yang perlu dibalikkan (idempoten).
    existing = resource.find("gl.journal-entry", {"source_id": params.get("id")})
    if existing == None:
        return ok({"status": "no_journal"})

    # Jurnal yang sudah dibalikkan tidak dibalikkan lagi (idempoten).
    if existing.field.status == "reversed":
        return ok({"status": "already_reversed"})

    resource.call("gl.journal-entry." + str(existing.id), "reverse", {})
    return ok({"status": "reversed", "journal": existing.id})
