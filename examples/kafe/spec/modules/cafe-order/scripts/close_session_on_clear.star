# ─────────────────────────────────────────────────────────────────────────────
# SUBSCRIPTION HANDLER: meja dikosongkan → sesi kunjungan ditutup
# ─────────────────────────────────────────────────────────────────────────────
#
# Dipanggil oleh `cafe-order/subscriptions/table-cleared.yaml` untuk event
# `cafe-master.dining-table.on_cleared` (transisi `release`, dipicu kasir saat
# menutup meja).
#
# KENAPA INI BAGIAN DARI KEBENARAN, BUKAN KERAPIAN:
#   `table-session` punya partial unique index "satu sesi TERBUKA per meja"
#   (10.34c), sementara halaman masuk QR membuat sesi baru setiap kali tamu
#   memindai kartu meja. Bila sesi lama tidak pernah ditutup, tamu kedua di meja
#   itu mendapat 500 `UNIQUE constraint failed` dari halaman masuk. Menutup sesi
#   saat meja dikosongkan adalah yang membuat index itu aman.
#
# Kenapa bukan hook pada `dining-table`: hook di sana akan menulis entity milik
# module lain dari script cafe-master → akses lintas-module yang harus
# dideklarasikan `uses.resources`, dan `SubscriptionSpec` belum punya blok itu.
# Pola subscription-per-pemilik-entity menghindari persoalan itu seluruhnya.
#
# Idempoten: outbox adalah at-least-once. Bila tidak ada sesi terbuka (tamu
# sudah pergi, atau meja dikosongkan dua kali) script ini tidak melakukan apa
# pun — bukan error, karena tidak ada yang salah.
# ─────────────────────────────────────────────────────────────────────────────


def close_open_session(resource, params, ctx):
    """Tutup sesi terbuka di meja yang baru dikosongkan."""
    # Meja mengirim id-nya SENDIRI sebagai `id`, bukan `dining_table_id`.
    #
    # Kesalahan pertama saya di sini membacanya sebagai `dining_table_id` —
    # nama field yang sama di entity `order` — sehingga `params.get` selalu
    # None, script mengambil jalur "tidak ada meja", dan sesi tidak pernah
    # ditutup TANPA satu pun error tercatat. Pelajaran yang layak diingat:
    # payload sebuah event adalah record PEMANCAR-nya, jadi nama fieldnya
    # adalah nama field entity itu (`id`), bukan nama kolom relasi yang
    # menunjuk ke sana dari entity lain.
    table_id = params.get("id")
    if table_id == None or table_id == "":
        return ok({"status": "no_table"})

    # `resource.find` cocok pada SEMUA pasangan (AND). Kombinasi meja + status
    # terbuka sudah cukup: index unik parsial menjamin paling banyak satu baris,
    # jadi tidak ada agregat/list yang dibutuhkan di sini.
    session = resource.find("cafe-order.table-session", {
        "dining_table_id": table_id,
        "status": "open",
    })
    if session == None:
        # Tidak ada sesi terbuka. Meja memang bisa dikosongkan tanpa tamu
        # (mis. kasir merapikan), jadi ini jalur normal — bukan kegagalan.
        return ok({"status": "no_open_session", "table": table_id})

    session.set("status", "closed")
    # `closed_at` diisi supaya sesi tertutup bisa dibedakan dari sesi yang
    # ditinggalkan, dan supaya durasi kunjungan bisa dihitung.
    session.set("closed_at", ctx.now())
    session.save()
    return ok({"status": "closed", "session": session.id, "table": table_id})


def execute(resource, params, ctx):
    """Pintu masuk handler (satu event, satu arah)."""
    return close_open_session(resource, params, ctx)
