# ─────────────────────────────────────────────────────────────────────────────
# GUARD: satu harga per menu per cabang (keputusan desain D1)
# ─────────────────────────────────────────────────────────────────────────────
#
# KENAPA SCRIPT INI ADA
#
# Aturan ini kini DITEGAKKAN DATABASE. GAP-22 sudah ditutup: `indexes:` dihormati,
# dan menyebut field `relation` di dalamnya menghasilkan kolom turunan
# (`_branch_id`, `_menu_item_id`). Terbukti lewat `formspec migrate plan`:
#     CREATE UNIQUE INDEX idx_cafe_master_menu_item_prices_branch_id_menu_item_id
#       ON cafe_master_menu_item_prices (_branch_id, _menu_item_id);
# Penutup lama berupa `kind: Migration` DDL mentah (`menu-item-price-unique`)
# sudah dihapus karena tidak lagi diperlukan — dan karena DDL itu tidak portabel
# (GAP-35).
#
# STATUS: LAPIS KEDUA — bukan pengaman utama.
#
# Guard ini tetap dipertahankan untuk satu alasan yang masih nyata:
#   - GAP-36: constraint hanya menolak baris BARU; ia tidak bisa merapikan
#     duplikat yang sudah ada (DML ditolak di `kind: Migration`), sementara guard
#     memberi pesan yang jelas ke pengguna.
#
# Constraint database selalu lebih kuat: ia berlaku untuk semua jalur tulis
# (API, script, seed, operator), guard hanya pada jalur yang melewatinya.
#
# ─── GAP YANG MEMPENGARUHI SCRIPT INI ───
#
# GAP-30 — `ctx.db().query()` di dalam transaksi aksi DEADLOCK di SQLite
#   (koneksi tunggal sementara transaksi aksi masih dipegang). Ini terdokumentasi
#   langsung sebagai komentar di examples/arisan/.../validate.star:
#     "Di SQLite (dev) ini DEADLOCK karena koneksi tunggal sedang dipegang
#      transaksi aksi. Di PostgreSQL (produksi) tidak deadlock."
#   Konsekuensi untuk kafe: guard ini aman di produksi (PostgreSQL) tapi
#   MEMBUAT DEV SQLITE GAGAL — dan yang gagal justru jalur yang ingin dijaga.
#   Ini memperburuk GAP-24: dev server mati, dan kalau hidup pun guard ini
#   akan deadlock pada driver dev.
#
# GAP-31 — tidak ada API Starlark "find by field value". `resource.fetch(entity,
#   id)` butuh ID, bukan filter; itu tidak menolong untuk cek keunikan. Karena
#   itu script ini terpaksa memakai `ctx.db().query` — padahal konvensi proyek
#   (AGENTS.md) menyatakan "never raw SQL". Jadi kita harus memilih antara
#   melanggar konvensi atau membiarkan aturan bisnis tidak dijaga.
#   API yang dibutuhkan: sesuatu seperti
#     resource.find("cafe-master/menu-item-price", {"branch_id": b, "menu_item_id": m})
#   sehingga `uses.resources: [menu-item-price.find]` cukup, tanpa SQL.
#
# GAP-32 — guard ini memakai SELECT-then-act (bukan transaksi atomik). Dua
#   permintaan bersamaan bisa sama-sama lolos SELECT lalu sama-sama INSERT.
#   Menutupnya butuh `ctx.lock` mengelilingi cek+simpan. Artinya: untuk
#   menegakkan satu UNIQUE constraint, kita menulis lock + query + insert/update
#   manual — reimplementasi yang lebih rapuh daripada constraint yang tidak
#   dihormati engine.
#
# Dipasang lewat `hooks:` pada entity `menu-item-price` (on: before, action:
# create dan update).
# ─────────────────────────────────────────────────────────────────────────────


def execute(resource, params, ctx):
    branch_id = resource.field.branch_id
    menu_item_id = resource.field.menu_item_id

    if not branch_id or not menu_item_id:
        # Validasi field wajib sudah ditangani entity; ini hanya sabuk pengaman
        # supaya query di bawah tidak pernah dijalankan dengan filter kosong
        # (filter kosong = "cocok dengan apa pun", bahaya untuk cek keunikan).
        return fail("Cabang dan Menu wajib diisi sebelum menyimpan harga.")

    # Saat UPDATE, baris yang sedang diedit tidak boleh dianggap sebagai
    # duplikat dirinya sendiri. `resource.id` kosong pada create → "" tidak
    # cocok dengan id mana pun, jadi perilakunya benar untuk kedua kasus.
    exclude_id = resource.id or ""

    rows = ctx.db().query(
        "SELECT id FROM cafe_master_menu_item_prices "
        + "WHERE branch_id = ? "
        + "  AND menu_item_id = ? "
        + "  AND deleted_at IS NULL "
        + "  AND id != ? "
        + "LIMIT 1",
        [branch_id, menu_item_id, exclude_id],
    )

    if len(rows) > 0:
        # Pesan menyebut CABANG dan MENU, bukan "duplikat" saja — supaya
        # kasir/admin tahu baris mana yang harus diubah, bukan harus menebak.
        return fail(
            "Harga untuk menu ini sudah ada di cabang tersebut. "
            + "Ubah baris harga yang sudah ada, jangan menambah baris baru."
        )

    return ok()
