# ─────────────────────────────────────────────────────────────────────────────
# GUARD: satu shift terbuka per (cabang, kasir)
# ─────────────────────────────────────────────────────────────────────────────
#
# Aturan bisnis #10. Sama kelasnya dengan GAP-22: constraint-nya TIDAK bisa
# dinyatakan di YAML dan tidak ditegakkan database.
#
# Kenapa tidak bisa di YAML: butuh PARTIAL unique index —
#     UNIQUE (branch_id, cashier_id) WHERE status = 'open'
# `IndexDecl` hanya mendukung {fields, unique}, tanpa predikat parsial. Dan
# bahkan kalau didukung pun, GAP-22 membuat `indexes:` diabaikan seluruhnya.
#
# PENUTUP SUDAH DITULIS DAN TERVERIFIKASI:
#   spec/modules/cafe-order/migrations/shift-open-unique.yaml
# (`CREATE UNIQUE INDEX ... WHERE status = 'open'`) — ter-apply lewat
# `formspec migrate apply` ("3 custom migration(s)").
# Guard ini sekarang LAPIS KEDUA. Hapus setelah migration terbukti berjalan di
# produksi — dan itu belum pasti, karena GAP-35 (ddl tidak portabel ke PostgreSQL).
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
