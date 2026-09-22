# ─────────────────────────────────────────────────────────────────────────────
# PEMELIHARA: proyeksi `stock-level` (saldo & biaya rata-rata bergerak)
# ─────────────────────────────────────────────────────────────────────────────
#
# Script ini adalah `maintained_by` dari entity `stock-level` (summary). Ia
# dipanggil lewat hook `after create` pada `stock-movement`, dan menulis
# proyeksi lewat `resource.upsert` — SATU-SATUNYA jalur tulis summary yang
# didukung (item 4.1, Opsi A).
#
# Invarian "satu baris per (cabang, bahan)" DITEGAKKAN DATABASE lewat
# `indexes:` pada entity `stock-level`:
#     - fields: [branch_id, ingredient_id]
#       unique: true
# `resource.upsert` mencari baris dengan `match` yang sama lalu update, atau
# insert bila belum ada — dan unique index adalah jaring pengaman terakhir.
#
# ─── KENAPA `resource.upsert`, BUKAN `resource.create`/`save` ───
#
# Entity `summary` menolak create/update/delete lewat API maupun lewat
# `resource.create`/`save` (guard store yang sama). Satu-satunya jalur tulis
# adalah `resource.upsert`, dan ia hanya mengizinkan script yang disebut
# `maintained_by` entity itu. Script lain → error.
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
# ─── GAP TERKAIT (kini tertutup) ───
#
# ✅ GAP-13  valuasi inventory — dijalankan lewat jalur tulis summary (4.1).
# ✅ GAP-22  UNIQUE (branch_id, ingredient_id) ditegakkan database (1.6).
# ✅ GAP-30  ctx.db() deadlock di SQLite — tidak lagi dipakai (4.5).
# ✅ GAP-31  find-by-field — `resource.find` (4.3).
# ✅ GAP-32  upsert atomik — `resource.upsert` + unique index (4.4).
# ✅ GAP-33  hooks pada summary — pemelihara dipanggil dari hook `stock-movement`.
# ─────────────────────────────────────────────────────────────────────────────


def moving_average(qty_lama, avg_lama, qty_masuk, biaya_masuk):
    """Biaya rata-rata bergerak setelah satu pergerakan masuk (D3)."""
    total_qty = qty_lama + qty_masuk
    if total_qty == 0:
        # Tidak ada stok sama sekali -> tidak ada dasar untuk rata-rata.
        # Pertahankan nilai lama alih-alih menghasilkan NaN.
        return avg_lama
    return (qty_lama * avg_lama + qty_masuk * biaya_masuk) / total_qty


def money_amount(value, default):
    """Angka dari nilai `money` (objek {amount, currency}) atau skalar.

    `unit_cost`/`moving_avg_cost` adalah nilai `money`; `float()` menolak
    objek seperti itu (S7/1.3 — operand tidak sah = error, bukan 0).
    Kegagalannya SENYAP bagi pemanggil: hook `after` tidak membatalkan
    respons, jadi error hanya tercatat di log engine.
    """
    if value == None:
        return default
    return float(amount(value))


def money_like(angka, sumber, default_currency):
    """Bentuk kanonik `money` untuk ditulis ke proyeksi.

    Mata uang ikut sumber bila ada, supaya proyeksi tidak menebak mata uang
    (spec §10: ragu = tolak/pertahankan, bukan tebak). Nilai bulat ditulis
    tanpa `.0` supaya bentuk tersimpannya sama dengan nilai money lain di app
    (mis. `{"amount": "60", "currency": "IDR"}`).
    """
    if angka == int(angka):
        amount_str = str(int(angka))
    else:
        amount_str = str(angka)
    if sumber == None:
        return {"amount": amount_str, "currency": default_currency}
    return {"amount": amount_str, "currency": currency(sumber)}


def min_stock_of(resource, ingredient_id):
    """Batas minimum stok dari entity `ingredient`, 0 bila tidak dideklarasikan."""
    ing = resource.fetch("cafe-stock.ingredient", ingredient_id)
    if ing == None:
        return 0
    return float(ing.field.min_stock or 0)


def below_min(resource, ingredient_id, qty_on_hand):
    """is_below_min: dipicu dari ingredient.min_stock (bukan di-update manual)."""
    return qty_on_hand < min_stock_of(resource, ingredient_id)


def execute(resource, params, ctx):
    # `resource` di sini adalah stock-movement yang sedang diproses.
    # Script ini adalah `maintained_by` dari entity `stock-level`, jadi ia
    # boleh memanggil `resource.upsert` (satu-satunya jalur tulis summary).
    branch_id = resource.field.branch_id
    ingredient_id = resource.field.ingredient_id
    direction = resource.field.direction
    qty = float(resource.field.quantity or 0)
    # `last_movement_at` diambil dari tanggal transaksi pergerakan (field yang
    # tersedia di record) — `created_at` engine tidak diekspos ke script.
    now = resource.field.transaction_date

    # Baca saldo yang ada lewat lapisan entity (bukan SQL). `resource.find`
    # mengembalikan resource atau None.
    current = resource.find(
        "cafe-stock.stock-level",
        {"branch_id": branch_id, "ingredient_id": ingredient_id},
    )

    if current == None:
        # Belum ada baris saldo -> buat dengan qty awal.
        # Pergerakan masuk pertama: avg = biaya masuk (tidak ada dasar lain).
        if direction == "in":
            avg_awal = money_amount(resource.field.unit_cost, 0)
        else:
            avg_awal = 0
        resource.upsert(
            "cafe-stock.stock-level",
            {"branch_id": branch_id, "ingredient_id": ingredient_id},
            {
                "quantity_on_hand": qty,
                "moving_avg_cost": money_like(avg_awal, resource.field.unit_cost, "IDR"),
                "stock_value": money_like(qty * avg_awal, resource.field.unit_cost, "IDR"),
                "last_movement_at": now,
                "is_below_min": below_min(resource, ingredient_id, qty),
            },
        )
        return ok({"action": "create", "quantity_on_hand": qty})

    qty_lama = float(current.field.quantity_on_hand or 0)
    avg_lama = money_amount(current.field.moving_avg_cost, 0)

    if direction == "in":
        avg_baru = moving_average(
            qty_lama, avg_lama, qty, money_amount(resource.field.unit_cost, 0)
        )
        qty_baru = qty_lama + qty
        resource.upsert(
            "cafe-stock.stock-level",
            {"branch_id": branch_id, "ingredient_id": ingredient_id},
            {
                "quantity_on_hand": qty_baru,
                "moving_avg_cost": money_like(avg_baru, resource.field.unit_cost, "IDR"),
                "stock_value": money_like(qty_baru * avg_baru, resource.field.unit_cost, "IDR"),
                "last_movement_at": now,
                "is_below_min": below_min(resource, ingredient_id, qty_baru),
            },
        )
        return ok({"action": "update", "quantity_on_hand": qty_baru})

    if direction == "out":
        # Keluar: qty berkurang, avg TIDAK berubah (nilai dibekukan ke
        # stock-movement oleh pemanggil).
        qty_baru = qty_lama - qty
        resource.upsert(
            "cafe-stock.stock-level",
            {"branch_id": branch_id, "ingredient_id": ingredient_id},
            {
                "quantity_on_hand": qty_baru,
                "moving_avg_cost": money_like(avg_lama, current.field.moving_avg_cost, "IDR"),
                "stock_value": money_like(qty_baru * avg_lama, current.field.moving_avg_cost, "IDR"),
                "last_movement_at": now,
                "is_below_min": below_min(resource, ingredient_id, qty_baru),
            },
        )
        return ok({"action": "update", "quantity_on_hand": qty_baru})

    # adjust (opname): qty disetel ke hasil hitung fisik, avg dipertahankan.
    resource.upsert(
        "cafe-stock.stock-level",
        {"branch_id": branch_id, "ingredient_id": ingredient_id},
        {
            "quantity_on_hand": qty,
            "moving_avg_cost": money_like(avg_lama, current.field.moving_avg_cost, "IDR"),
            "stock_value": money_like(qty * avg_lama, current.field.moving_avg_cost, "IDR"),
            "last_movement_at": now,
            "is_below_min": below_min(resource, ingredient_id, qty),
        },
    )
    return ok({"action": "update", "quantity_on_hand": qty})
