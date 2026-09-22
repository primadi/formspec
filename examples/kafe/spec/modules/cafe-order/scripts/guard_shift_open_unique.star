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
# ✅ GAP-30 (item 4.5) — `ctx.db().query()` dulu deadlock di SQLite dalam
#   transaksi aksi. `resource.find` memakai jalur baca store yang scope-aware.
# ✅ GAP-31 (item 4.3) — dulu tidak ada API find-by-field; kini
#   `resource.find(entity, {field: value, ...})` mencari lewat lapisan entity.
# ⚠️ GAP-32 (item 4.4) — SELECT-then-act tidak atomik; yang menutupnya adalah
#   partial unique index di atas (database), bukan guard ini.
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

    existing = resource.find(
        "cafe-order.shift",
        {"branch_id": branch_id, "cashier_id": cashier_id, "status": "open"},
    )

    if existing != None and existing.id != resource.id:
        return fail(
            "Kasir ini masih memiliki shift terbuka di cabang tersebut. "
            + "Tutup shift yang sedang berjalan sebelum membuka shift baru."
        )

    return ok()
