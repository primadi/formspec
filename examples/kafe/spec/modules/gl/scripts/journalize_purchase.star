# ─────────────────────────────────────────────────────────────────────────────
# PEMBELIAN → JURNAL — mendengarkan event durable dari module `cafe-stock`
# ─────────────────────────────────────────────────────────────────────────────
#
# Item 10.7. `cafe-stock/purchase-order` memancarkan `on_po_received` (dan
# `on_po_cancelled`) sebagai event durable; **tidak ada satu baris pun di module
# pemesanan** yang tahu bahwa modul ini ada. Itulah bentuk integrasi yang diminta
# pemilik: purchase memancarkan, gl mendengarkan, dan pengetahuan akuntansi
# (kode akun, arah debit/kredit) tinggal di sini.
#
# ─── BENTUK JURNAL ───
#
#   Persediaan Bahan   (debit)   = Σ baris bahan (qty × unit_cost)
#   Utang Dagang       (kredit)  = jumlah yang sama
#
# Mengapa persediaan, bukan beban: bahan yang dibeli BELUM menjadi beban — ia
# menjadi aset sampai dipakai. Beban muncul saat bahan itu dikonsumsi (HPP pada
# penjualan). Mencatat pembelian langsung sebagai beban akan membuat laba bulan
# ini bergantung pada kapan bahan dibeli, bukan kapan terjual — dan itu kesalahan
# akuntansi, bukan pilihan gaya.
#
# Mengapa utang dagang, bukan kas: PO menyatakan barang diterima, bukan dibayar.
# Pelunasan ke supplier adalah peristiwa tersendiri (payment voucher) yang belum
# dimodelkan — dan mengasumsikan "diterima = dibayar tunai" akan menghasilkan
# saldo kas yang tidak pernah cocok dengan rekening bank.
#
# ─── BIAYA KIRIM (landed cost, item 10.1) ───
#
# `shipping_cost` dan baris `line_type: cost` SUDAH dialokasikan ke
# `stock-movement.unit_cost` oleh action `receive-goods` di sisi
# `cafe-stock` — jadi menambahkannya lagi di sini akan menghitung dua kali.
# Jurnal ini mencatat persediaan senilai yang dibukukan oleh pergerakan stok,
# dan itulah jumlah yang benar untuk dipertanggungjawabkan.
#
# Akibatnya: `Σ baris bahan` DI SINI adalah nilai barang saja, sedangkan
# `unit_cost` yang tersimpan sudah termasuk landed cost. Kedua angka itu berbeda
# dengan sengaja, dan yang perlu direkonsiliasi adalah nilai persediaan total
# (dari `stock-level.stock_value`), bukan baris per baris PO ini.
#
# ─── IDEMPOTEN ───
#
# Retry outbox boleh mengantarkan event yang sama lebih dari sekali. Kunci
# idempotensinya `source_id` (= id PO), sama seperti jurnal penjualan.
#
# ─── PEMBALIKAN ───
#
# Aturan simetri cancel (7.7.2): pemicu yang menulis jurnal WAJIB menyatakan
# pembalikannya sejak awal. `on_po_cancelled` membalik jurnalnya.
# ─────────────────────────────────────────────────────────────────────────────


def money_of(value, default):
    """Angka dari nilai `money` ({amount, currency}) atau skalar."""
    if value == None:
        return default
    return float(amount(value))


def currency_of(value, default):
    """Mata uang dari nilai money, atau default bila nilainya kosong."""
    if value == None:
        return default
    return currency(value)


def decimal_like(x):
    """Bentuk decimal untuk baris jurnal: bilangan bulat tanpa `.0`."""
    if x == int(x):
        return int(x)
    return x


def field_of(row, name, default):
    """Baca field dari baris child — dict ATAU FieldMap, tergantung hidrasinya."""
    if type(row) == "dict":
        value = row.get(name)
    else:
        value = getattr(row, name, None)
    if value == None:
        return default
    return value


def account_id_for(resource, ctx, role, code_setting, code_default):
    """ID record akun untuk satu peran (persediaan/utang/...).

    Kunci setting kosong → fail dengan pesan yang menunjuk kuncinya. Menebak
    kode akun akan menghasilkan jurnal yang seimbang secara aritmetika tetapi
    salah secara akuntansi — kegagalan yang jauh lebih mahal daripada error.
    """
    code = ctx.config.get(code_setting)
    if code == None or code == "":
        code = code_default
    if code == None or code == "":
        fail("FORMSPEC.GL.ACCOUNT_NOT_SET: kunci " + code_setting + " belum diatur - jurnal pembelian tidak bisa dibangun")
    acc = resource.find("gl.account", {"code": code})
    if acc == None:
        fail("FORMSPEC.GL.ACCOUNT_NOT_FOUND: setting akun '" + role + "' menunjuk kode '" + code + "' yang tidak ada di bagan akun - lengkapi chart of accounts atau perbaiki kunci " + code_setting + " di Config gl")
    # `acc.id` = identitas record (kolom tabel, bukan field data).
    return acc.id


def receive_amount(resource, params):
    """Nilai barang yang diterima = Σ (received_quantity | quantity) × unit_cost.

    Baris `line_type: cost` DILEWATI: biaya itu sudah masuk `unit_cost` pergerakan
    stok lewat alokasi landed cost di sisi cafe-stock. Menjumlahkannya lagi di
    sini akan menghitung biaya kirim dua kali.
    """
    total = 0.0
    lines = params.get("lines")
    if lines == None:
        lines = []
    for baris in lines:
        if field_of(baris, "line_type", "ingredient") == "cost":
            continue
        qty = field_of(baris, "received_quantity", 0) or field_of(baris, "quantity", 0) or 0
        harga = money_of(field_of(baris, "unit_cost", None), 0)
        total = total + (float(qty) * harga)
    return total


def journalize_purchase(resource, params, ctx):
    po_id = params.get("id")
    po_number = params.get("number")

    # Idempoten per source_id (retry outbox tidak boleh menggandakan jurnal).
    existing = resource.find("gl.journal-entry", {"source_id": po_id})
    if existing != None:
        return {"status": "already_journaled", "journal": existing.id}

    nilai = receive_amount(resource, params)
    if nilai <= 0:
        fail("FORMSPEC.GL.NO_AMOUNT: pembelian " + str(po_number) + " diterima tanpa nilai persediaan - isi unit_cost pada baris PO sebelum menerima barang")

    persediaan_id = account_id_for(resource, ctx, "persediaan", "gl_journal_account_persediaan", "1-2000")
    utang_id = account_id_for(resource, ctx, "utang_dagang", "gl_journal_account_utang_dagang", "2-3000")

    lines = [
        {
            "description": "Persediaan bahan",
            "account_id": persediaan_id,
            "debit": decimal_like(nilai),
            "credit": 0,
        },
        {
            "description": "Utang dagang",
            "account_id": utang_id,
            "debit": 0,
            "credit": decimal_like(nilai),
        },
    ]

    jid = resource.create("gl.journal-entry", {
        "transaction_date": params.get("transaction_date"),
        "entry_date": params.get("transaction_date"),
        "reference": po_number,
        "source": "cafe-stock.purchase-order",
        "source_id": po_id,
        "description": "Pembelian " + str(po_number),
        "currency": currency_of(field_of((params.get("lines") or [{}])[0], "unit_cost", None), "IDR"),
        "lines": lines,
    })

    # Posting langsung: guard `post` menegakkan keseimbangan di lapisan action,
    # jadi jurnal yang tidak seimbang tidak akan bisa diposting.
    resource.call("gl.journal-entry." + str(jid.id), "post", {})
    return {"journal": jid.id, "status": "posted", "amount": nilai}


def reverse_purchase(resource, params, ctx):
    """Pembatalan PO membalik jurnalnya (aturan simetri 7.7.2)."""
    existing = resource.find("gl.journal-entry", {"source_id": params.get("id")})
    if existing == None:
        # PO dibatalkan sebelum barang diterima → tidak ada jurnal. Idempoten.
        return {"status": "no_journal"}
    if existing.field.status == "reversed":
        return {"status": "already_reversed"}
    resource.call("gl.journal-entry." + str(existing.id), "reverse", {})
    return {"status": "reversed", "journal": existing.id}


def execute(resource, params, ctx):
    # Event membawa SELURUH record (tidak ada `payload.fields`), jadi `lines`
    # ikut — termasuk baris `line_type: cost` yang harus dilewati saat menghitung
    # nilai barang. Itulah alasan script ini membaca child-nya, bukan sekadar
    # `total_amount`.
    #
    # CATATAN PENTING tentang `script_ref`: setiap file .star adalah unit
    # kompilasi SENDIRI. Fungsi yang didefinisikan di `journalize.star` TIDAK
    # terlihat di sini — versi pertama script ini memanggil `journalize_sale` dari
    # sana dan gagal `undefined: journalize_sale` saat event pertama dikirim.
    # Kegagalannya terlihat (outbox worker mencatat + retry), tetapi baru muncul
    # di RUNTIME karena `formspec validate` hanya mengompilasi tiap file secara
    # terpisah — ia tidak bisa tahu bahwa satu script merujuk fungsi milik script
    # lain. Karena itu script ini mandiri: hanya menangani event pembelian.
    event = params["_event"]["name"]
    if event == "cafe-stock.purchase-order.on_po_received":
        return journalize_purchase(resource, params, ctx)
    return reverse_purchase(resource, params, ctx)
