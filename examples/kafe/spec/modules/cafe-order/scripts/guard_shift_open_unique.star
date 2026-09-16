# ─────────────────────────────────────────────────────────────────────────────
# GUARD: satu shift terbuka per (cabang, kasir)
# ─────────────────────────────────────────────────────────────────────────────
#
# Aturan bisnis #10 — kini DITEGAKKAN DATABASE sebagai partial unique index (S8):
#     UNIQUE (branch_id, cashier_id) WHERE status = 'open'
# dinyatakan langsung di `shift/entity.yaml`:
#     - fields: [branch_id, cashier_id]
#       unique: true
#       where: "status = 'open'"
# Predikat itu portabel (SQLite & PostgreSQL sama-sama mendukung partial index),
# jadi penutup lama berupa `kind: Migration` DDL mentah sudah dihapus — bersama
# GAP-35 yang tidak lagi berlaku untuk kasus ini.
#
# Guard ini LAPIS KEDUA yang memberi pesan error ramah di action pipeline.
# Constraint database-lah yang berlaku untuk semua jalur tulis.
#
# GAP-30 — ctx.db().query() deadlock di SQLite dalam transaksi aksi (lihat
#   catatan lengkap di cafe-master/scripts/guard_menu_item_price_unique.star).
# GAP-31 — tidak ada API find-by-field; terpaksa raw SQL.
# GAP-32 — SELECT-then-act tidak atomik; butuh ctx.lock untuk benar-benar rapat.
#
# Dipasang lewat `hooks:` pada entity `shift` (on: before, action: create).
# ─────────────────────────────────────────────────────────────────────────────


def execute(resource, params, ctx):
    # Hanya relevan saat shift baru berstatus terbuka. Menutup shift
    # (status -> closed) tidak boleh diblokir.
    if resource.field.status != "open":
        return ok()

    branch_id = resource.field.branch_id
    cashier_id = resource.field.cashier_id

    if not branch_id or not cashier_id:
        return fail("Cabang dan Kasir wajib diisi sebelum membuka shift.")

    exclude_id = resource.id or ""

    rows = ctx.db().query(
        "SELECT id FROM cafe_order_shifts "
        + "WHERE branch_id = ? "
        + "  AND cashier_id = ? "
        + "  AND status = 'open' "
        + "  AND deleted_at IS NULL "
        + "  AND id != ? "
        + "LIMIT 1",
        [branch_id, cashier_id, exclude_id],
    )

    if len(rows) > 0:
        return fail(
            "Kasir ini masih memiliki shift terbuka di cabang tersebut. "
            + "Tutup shift yang sedang berjalan sebelum membuka shift baru."
        )

    return ok()
