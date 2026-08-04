# modules/arisan-field/scripts/period_run_lottery.star
#
# Aksi `run-lottery` pada entity arisan-period (arisan-field).
# Membuat record penarikan (draw) untuk pemenang yang memiliki iuran VALID
# pada periode ini, lalu menutup periode (open → closed).
#
# params.member_id (wajib)      : id anggota pemenang undian
# params.contribution_id (wajib): id iuran valid milik pemenang di periode ini
# params.amount (opsional)      : total pot yang diterima; default = nominal iuran pemenang
#
# Satu periode hanya boleh menghasilkan SATU penarikan: begitu periode
# ditutup, aksi run-lottery menolak dijalankan lagi.

def execute(resource, params, ctx):
    if resource.field.status != "open":
        return fail("periode sudah ditutup — undian hanya untuk periode terbuka")

    if "member_id" not in params:
        return fail("params.member_id wajib diisi: pemenang undian")
    if "contribution_id" not in params:
        return fail("params.contribution_id wajib diisi: iuran valid pemenang di periode ini")

    contrib = resource.fetch("contribution", params["contribution_id"])

    if contrib.field.period_id != resource.id:
        return fail("iuran yang dipilih bukan untuk periode ini")
    if contrib.field.member_id != params["member_id"]:
        return fail("iuran yang dipilih bukan milik pemenang")
    if contrib.field.status != "validated":
        return fail("anggota belum memiliki iuran valid untuk periode ini — undian dibatalkan")

    amount = params["amount"] if "amount" in params else contrib.field.amount

    draw = resource.create("draw", {
        "transaction_date": ctx.today(),
        "group_id": resource.field.group_id,
        "period_id": resource.id,
        "member_id": params["member_id"],
        "amount": amount,
        "status": "drawn",
    })

    resource.set("status", "closed")
    resource.save()

    ctx.log.info("period.lottery_drawn", {
        "period_id": resource.id,
        "member_id": params["member_id"],
        "draw_id": draw.id,
    })

    return ok({"draw_id": draw.id})
