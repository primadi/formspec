# ─────────────────────────────────────────────────────────────────────────────
# GUARD: satu harga per menu per cabang (keputusan desain D1)
# ─────────────────────────────────────────────────────────────────────────────
#
# KENAPA SCRIPT INI ADA
#
# Aturan ini kini DITEGAKKAN DATABASE: `indexes:` dihormati,
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
# ─── GAP YANG DULU MEMPENGARUHI SCRIPT INI (kini tertutup) ───
#
# ✅ GAP-31 (item 4.3) — dulu tidak ada API "find by field value", sehingga
#   script ini terpaksa memakai `ctx.db().query` (melanggar konvensi "never raw
#   SQL"). Sekarang ada `resource.find(entity, {field: value, ...})` yang
#   mencari lewat lapisan entity — jadi tenant isolation & row scope berlaku,
#   dan nama tabel/kolom fisik tidak perlu diketahui script.
#
# ✅ GAP-30 (item 4.5) — `ctx.db().query()` di dalam transaksi aksi dulu
#   DEADLOCK di SQLite (koneksi tunggal). `resource.find` memakai jalur baca
#   store yang sudah scope-aware (`txReadDB`), jadi tidak ada koneksi kedua.
#
# ⚠️ GAP-32 (item 4.4) — guard ini masih SELECT-then-act (bukan transaksi
#   atomik). Dua permintaan bersamaan bisa sama-sama lolos SELECT lalu
#   sama-sama INSERT. Yang menutupnya adalah UNIQUE index di atas (database),
#   bukan guard ini — guard hanya memberi pesan yang lebih baik.
#
# Dipasang lewat `hooks:` pada entity `menu-item-price` (on: before, action:
# create dan update).
# ─────────────────────────────────────────────────────────────────────────────


def execute(resource, params, ctx):
    branch_id = resource.field.branch_id
    menu_item_id = resource.field.menu_item_id

    if not branch_id or not menu_item_id:
        # Validasi field wajib sudah ditangani entity; ini hanya sabuk pengaman
        # supaya pencarian di bawah tidak pernah dijalankan dengan filter kosong
        # (filter kosong = "cocok dengan apa pun", bahaya untuk cek keunikan).
        return fail("Cabang dan Menu wajib diisi sebelum menyimpan harga.")

    # Cari harga yang sudah ada untuk (cabang, menu) ini. `resource.find`
    # mengembalikan resource atau None — tanpa SQL, tanpa nama tabel fisik.
    existing = resource.find(
        "cafe-master.menu-item-price",
        {"branch_id": branch_id, "menu_item_id": menu_item_id},
    )

    # Saat UPDATE, baris yang sedang diedit tidak boleh dianggap sebagai
    # duplikat dirinya sendiri. `resource.id` kosong pada create → tidak cocok
    # dengan id mana pun, jadi perilakunya benar untuk kedua kasus.
    if existing != None and existing.id != resource.id:
        # Pesan menyebut CABANG dan MENU, bukan "duplikat" saja — supaya
        # kasir/admin tahu baris mana yang harus diubah, bukan harus menebak.
        return fail(
            "Harga untuk menu ini sudah ada di cabang tersebut. "
            + "Ubah baris harga yang sudah ada, jangan menambah baris baru."
        )

    return ok()
