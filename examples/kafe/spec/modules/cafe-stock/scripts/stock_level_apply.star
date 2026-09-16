# ─────────────────────────────────────────────────────────────────────────────
# GUARD: satu baris stock-level per (cabang, bahan)
# ─────────────────────────────────────────────────────────────────────────────
#
# STATUS: DITULIS TAPI **SENGAJA TIDAK DIPASANG** lewat `hooks:`. Lihat GAP-33
# di bawah — pada entity `summary`, hook memang tidak pernah dipanggil.
#
# Invarian ini kini DITEGAKKAN DATABASE lewat `indexes:` pada entity
# `stock-level` (GAP-22 sudah ditutup):
#     - fields: [branch_id, ingredient_id]
#       unique: true
# Penutup lama berupa `kind: Migration` DDL mentah (`stock-level-unique`) sudah
# dihapus: DDL itu tidak portabel antar-driver, sementara index yang dihasilkan
# engine portabel.
# File ini tetap berguna sebagai RUJUKAN RUMUS biaya rata-rata bergerak (D3)
# untuk script penulis stock-level.
#
# ─── KENAPA TIDAK DIPASANG — dan ini temuan baru (GAP-33) ───
#
# `stock-level` berkarakteristik `summary`. FormSpec menonaktifkan
# create/update/delete permanen untuk summary — entity jenis ini HANYA ditulis
# oleh script, dan penulisan itu TIDAK lewat action pipeline.
#
# Akibatnya: `hooks:` dan `conditions:` pada entity `summary` TIDAK PERNAH
# DIPANGGIL. Jadi guard ini akan terlihat terpasang dan tidak melakukan apa pun
# — persis pola gagal-senyap yang sedang kita kejar di seluruh catatan gap.
#
# Menyadari ini penting: untuk entity `summary`, satu-satunya tempat menegakkan
# invarian adalah DI DALAM script yang menulisnya. Tidak ada titik intersepsi
# deklaratif.
#
# ─── MAKA: INVARIANNYA HARUS DIPEgang OLEH PENULIS ───
#
# Setiap script yang menulis `stock-level` (penerimaan barang, pemakaian saat
# pesanan lunas, bahan terbuang, posting opname) WAJIB memakai pola upsert:
#
#   1. baca baris yang ada untuk (branch_id, ingredient_id)
#   2. kalau ada  -> update  (rumus rata-rata bergerak di bawah)
#   3. kalau tidak -> create
#
# dan WAJIB membungkusnya dengan `ctx.lock` pada kunci
# `stock-level:{branch_id}:{ingredient_id}`, karena pola baca-lalu-tulis tidak
# atomik (GAP-32). Dua pergerakan bersamaan tanpa lock bisa menghasilkan dua
# baris untuk pasangan yang sama.
#
# ─── RUMUS BIAYA RATA-RATA BERGERAK (D3) ───
#
# Hanya pergerakan MASUK yang mengubah biaya rata-rata:
#
#     avg_baru = (qty_lama * avg_lama + qty_masuk * biaya_masuk) / (qty_lama + qty_masuk)
#
# Pergerakan KELUAR memakai avg saat itu sebagai `unit_cost` di stock-movement
# (dibekukan), dan TIDAK mengubah avg. Ini yang membuat HPP historis tidak
# berubah surut saat harga bahan naik.
#
# Pembagian dengan nol harus dijaga: kalau (qty_lama + qty_masuk) == 0,
# pertahankan avg lama (jangan hasilkan NaN — NaN akan menyebar diam-diam ke
# seluruh laporan margin).
#
# ─── GAP TERKAIT ───
#
# GAP-13  metode valuasi tidak ada di engine -> D3 mengerjakannya di Starlark.
# GAP-22  UNIQUE (branch_id, ingredient_id) tidak ditegakkan database.
# GAP-30  ctx.db().query() deadlock di SQLite dalam transaksi aksi.
# GAP-31  tidak ada API find-by-field; terpaksa raw SQL.
# GAP-32  butuh ctx.lock; tanpa itu upsert tidak rapat.
# GAP-33  hooks/conditions tidak berlaku untuk entity `summary`.
# GAP-28  money adalah objek {amount, currency} -> aritmetika avg belum teruji.
# ─────────────────────────────────────────────────────────────────────────────


def moving_average(qty_lama, avg_lama, qty_masuk, biaya_masuk):
    """Biaya rata-rata bergerak setelah satu pergerakan masuk (D3)."""
    total_qty = qty_lama + qty_masuk
    if total_qty == 0:
        # Tidak ada stok sama sekali -> tidak ada dasar untuk rata-rata.
        # Pertahankan nilai lama alih-alih menghasilkan NaN.
        return avg_lama
    return (qty_lama * avg_lama + qty_masuk * biaya_masuk) / total_qty


def execute(resource, params, ctx):
    # `resource` di sini adalah stock-movement yang sedang diproses.
    # Catatan: aritmetika di bawah memakai angka, BUKAN objek money — lihat
    # GAP-28. Idealnya unit_cost diambil sebagai `resource.field.unit_cost.amount`.
    branch_id = resource.field.branch_id
    ingredient_id = resource.field.ingredient_id
    direction = resource.field.direction
    qty = float(resource.field.quantity or 0)

    exclude_id = resource.id or ""

    rows = ctx.db().query(
        "SELECT id, quantity_on_hand, moving_avg_cost FROM cafe_stock_stock_levels "
        + "WHERE branch_id = ? "
        + "  AND ingredient_id = ? "
        + "  AND deleted_at IS NULL "
        + "  AND id != ? "
        + "LIMIT 1",
        [branch_id, ingredient_id, exclude_id],
    )

    if len(rows) == 0:
        # Belum ada baris saldo -> buat.
        # Bungkus dengan ctx.lock di pemanggil (GAP-32).
        return ok({"action": "create", "quantity_on_hand": qty})

    current = rows[0]
    qty_lama = float(current["quantity_on_hand"] or 0)
    avg_lama = float(current["moving_avg_cost"] or 0)

    if direction == "in":
        avg_baru = moving_average(
            qty_lama, avg_lama, qty, float(resource.field.unit_cost or 0)
        )
        return ok(
            {
                "action": "update",
                "stock_level_id": current["id"],
                "quantity_on_hand": qty_lama + qty,
                "moving_avg_cost": avg_baru,
            }
        )

    if direction == "out":
        # Keluar: qty berkurang, avg TIDAK berubah (nilai dibekukan ke
        # stock-movement oleh pemanggil).
        return ok(
            {
                "action": "update",
                "stock_level_id": current["id"],
                "quantity_on_hand": qty_lama - qty,
                "moving_avg_cost": avg_lama,
            }
        )

    # adjust (opname): qty disetel ke hasil hitung fisik, avg dipertahankan.
    return ok(
        {
            "action": "update",
            "stock_level_id": current["id"],
            "quantity_on_hand": qty,
            "moving_avg_cost": avg_lama,
        }
    )
