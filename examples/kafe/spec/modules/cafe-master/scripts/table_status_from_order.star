# ─────────────────────────────────────────────────────────────────────────────
# SUBSCRIPTION HANDLER: pesanan lunas/disajikan/batal → status meja
# ─────────────────────────────────────────────────────────────────────────────
#
# Dipanggil oleh `cafe-master/subscriptions/table-occupancy.yaml` untuk tiga
# event dari module cafe-order (pemancarnya adalah transisi: `confirm-payment`
# → `emit: on_paid`, `mark-served` → `emit: on_served`, dan
# `cancel-order`/`void-order` → `emit: on_cancel`):
#
#   cafe-order.order.on_paid    → meja jadi `occupied` (tamu sedang dilayani)
#   cafe-order.order.on_served  → meja jadi `served` (pesanan diantar)
#   cafe-order.order.on_cancel  → meja kembali `available` (tamu batal)
#
# KENAPA HANDLER INI MILIK cafe-master: ia menulis `dining-table`, dan entity
# itu milik cafe-master. Akses lintas-module dari script ditegakkan saat runtime
# ("undeclared cross-module access to cafe-master.dining-table from module
# cafe-order"), dan `SubscriptionSpec` belum punya blok `uses` untuk
# mendeklarasikannya. Pola ini sama dengan `gl/journalize.star` (pemilik jurnal
# mendengarkan event pesanan) — modul yang tahu aturan sebuah entity adalah
# modul yang memilikinya.
#
# Arah ketergantungannya tetap benar: cafe-master mendengarkan NAMA event yang
# sepenuhnya terkualifikasi, bukan sebaliknya. cafe-order tidak tahu apa pun
# tentang meja, dan tidak perlu tahu.
#
# KENAPA `find` → `set` → `save`, BUKAN `upsert`:
#   `resource.upsert` adalah jalur tulis SATU-SATUNYA untuk entity
#   `characteristic: summary` — ia MENOLAK entity non-summary
#   ("UpsertProjection is only valid for summary entities"). `dining-table`
#   adalah `master`, jadi jalurnya `find` (cari meja) → `set` (ubah field) →
#   `save` (tulis balik).
#
# KENAPA `fetch` (by id), BUKAN `find`:
#   `dining_table_id` DI event adalah ID record meja (relasi disimpan sebagai
#   id), jadi `fetch` adalah pencarian yang tepat — dan itu bukan pilihan
#   gaya: `resource.find(entity, {"id": ...})` TIDAK BISA dipakai di sini.
#   `find` menerjemahkan setiap kunci match ke kolom turunan `_<field>`
#   (`id` → `_id`), sedangkan `id` adalah kolom PRIMARY KEY fisik, bukan field
#   data — hasilnya `no such column: _id`. Terukur 2026-09-27.
#   `fetch` juga menghormati tenant isolation, sama seperti `find`.
#
# CATATAN PENTING TENTANG PERMISSION (10.46):
#   Subscription berjalan sebagai SYSTEM, di luar jalur HTTP. `save()` menulis
#   lewat store, dan store memvalidasi **legalitas** transisi (`from`/`to`
#   terdaftar) tetapi TIDAK memeriksa `require_permission` — gate itu hidup di
#   lapisan HTTP. Untuk meja, transisi yang dipakai (`available|reserved|served
#   → occupied`, `occupied → served`, `occupied → available`) adalah transisi
#   yang MEMANG tidak digerbangi, jadi tidak ada gate yang dilewati di sini.
#   Bila kelak sebuah transisi bergerbang ditulis dari script, batasnya
#   menyempit — itulah isi item 10.46 yang tetap terbuka.
# ─────────────────────────────────────────────────────────────────────────────
#   digerbangi, jadi tidak ada gate yang dilewati di sini. Bila kelak sebuah
#   transisi bergerbang ditulis dari script, batasnya menyempit — itulah isi
#   item 10.46 yang tetap terbuka.
# ─────────────────────────────────────────────────────────────────────────────


# Status meja yang MASIH berarti "bisa ditempati tamu baru". `occupied` sengaja
# TIDAK ada di sini: pesanan KEDUA dari sesi yang sama (tamu menambah es teh)
# membayar saat meja sudah `occupied`, dan itu tidak boleh dianggap perubahan.
MENEMPATI = ["available", "reserved", "served"]


def status_of(table, default):
    """Status meja sebagai string.

    Field yang belum ada pada baris lama mengembalikan None (bukan error) —
    pembacaan field yang tidak ada memang mengembalikan None. Diperlakukan
    sebagai `default` supaya script tidak menabrak baris warisan.
    """
    value = table.field.table_status
    if value == None:
        return default
    return str(value)


def occupy(resource, params, ctx):
    """Pesanan lunas → meja `occupied` (kalau memang belum terisi)."""
    table_id = params.get("dining_table_id")
    if table_id == None or table_id == "":
        # Pesanan takeaway / kasir tanpa meja. Tidak ada yang perlu ditandai.
        return ok({"status": "no_table"})

    table = resource.fetch("cafe-master.dining-table", table_id)
    if table == None:
        return ok({"status": "table_not_found", "table_id": table_id})

    current = status_of(table, "available")
    if current == "occupied":
        # Idempoten: outbox adalah at-least-once, dan pesanan berikutnya dari
        # sesi yang sama juga memancarkan on_paid. Meja yang sudah terisi
        # TIDAK boleh "berubah" hanya karena ada pembayaran lagi.
        return ok({"status": "already_occupied", "table": table.id})

    if current not in MENEMPATI:
        # `not_available` (rusak/dibersihkan oleh admin) tidak boleh diubah
        # diam-diam oleh pembayaran. Biarkan kasir yang memutuskan.
        return ok({"status": "not_placeable", "table_status": current, "table": table.id})

    table.set("table_status", "occupied")
    table.save()
    return ok({"status": "occupied", "table": table.id, "from": current})


def serve(resource, params, ctx):
    """Pesanan disajikan → meja `served`.

    ---
    KETERBATASAN YANG DIAKUI (ledger 10.41 — revisi rancangan awal).

    Rancangan awal 10.41 menginginkan `served` HANYA bila **semua** pesanan
    dalam sesi sudah disajikan. Itu agregat lintas-record, dan Starlark tidak
    punya list/count (`resource.find` mengembalikan SATU record). Jalur yang
    tersisa adalah `ctx.db().query`, dan untuk itu Subscription harus punya blok
    `uses` — yang belum ada di kind ini.

    Yang dipakai sekarang adalah aturan yang **lebih longgar tetapi tidak pernah
    salah arah**: pesanan yang disajikan mengubah meja menjadi `served`. Bila
    satu sesi punya dua pesanan dan baru satu yang diantar, meja lebih cepat
    `served` — tamu masih duduk, dan pembayaran berikutnya membawanya kembali
    `occupied`, jadi tidak ada tamu yang kehilangan mejanya karena ini.

    Yang TIDAK dikorbankan: `occupied` hanya lahir dari pembayaran, dan hanya
    `release` (ber-permission) yang bisa mengosongkan meja yang terisi.
    """
    table_id = params.get("dining_table_id")
    if table_id == None or table_id == "":
        return ok({"status": "no_table"})

    table = resource.fetch("cafe-master.dining-table", table_id)
    if table == None:
        return ok({"status": "table_not_found", "table_id": table_id})

    current = status_of(table, "available")
    if current == "occupied":
        table.set("table_status", "served")
        table.save()
        return ok({"status": "served", "table": table.id})

    if current == "served":
        # Idempoten: outbox at-least-once, dan tiap pesanan sesi memancarkan
        # on_served sendiri.
        return ok({"status": "already_served", "table": table.id})

    return ok({"status": "not_served", "table_status": current, "table": table.id})


def release(resource, params, ctx):
    """Pesanan batal → meja kembali `available` (hanya bila sedang terisi)."""
    table_id = params.get("dining_table_id")
    if table_id == None or table_id == "":
        return ok({"status": "no_table"})

    table = resource.fetch("cafe-master.dining-table", table_id)
    if table == None:
        return ok({"status": "table_not_found", "table_id": table_id})

    current = status_of(table, "available")
    if current != "occupied":
        # Meja sudah `available` (kasir sudah mengosongkan) atau `served`
        # (tamu masih duduk) — pembatalan tidak boleh menariknya dari tamu.
        #
        # CATATAN (sisa yang diakui, ledger 10.52): bila SATU meja punya
        # beberapa pesanan dan hanya satu yang dibatalkan, script ini tidak
        # bisa tahu apakah masih ada pesanan lain yang hidup — Starlark hanya
        # punya `find` (satu record), tanpa list/count. Untuk sekarang itu
        # diterima: pembatalan pada meja berisi beberapa pesanan akan
        # mengosongkan meja lebih awal, dan kasir menandainya kembali.
        return ok({"status": "not_released", "table_status": current, "table": table.id})

    table.set("table_status", "available")
    table.save()
    return ok({"status": "released", "table": table.id})


def execute(resource, params, ctx):
    """Pintu masuk handler.

    Nama event menentukan arahnya — `_event` disuntikkan runtime ke params
    (pola yang sama dengan `gl/journalize.star`).
    """
    event = params["_event"]["name"]
    if event.endswith("on_paid"):
        return occupy(resource, params, ctx)
    if event.endswith("on_served"):
        return serve(resource, params, ctx)
    return release(resource, params, ctx)
