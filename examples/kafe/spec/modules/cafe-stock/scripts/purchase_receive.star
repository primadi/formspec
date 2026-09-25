# ─────────────────────────────────────────────────────────────────────────────
# ACTION `receive-goods` — purchase-order → stock-movement + alokasi landed cost
# ─────────────────────────────────────────────────────────────────────────────
#
# Menutup item 10.1 (biaya kirim masuk HPP) dan memberi isi pada item 10.2:
# transisi `submitted → received` kini benar-benar MENCATAT sesuatu, bukan
# hanya memindahkan kolom status.
#
# ─── APA YANG DIKERJAKAN ───
#
#   1. Untuk setiap baris BAHAN yang diisi `received_quantity`:
#      buat satu `stock-movement` (direction: in, source: purchase).
#   2. Alokasikan biaya perolehan tambahan (landed cost) ke `unit_cost` baris
#      itu, sehingga biaya rata-rata bergerak bahan naik — dan HPP menu ikut
#      naik lewat `stock-level`/`menu-cost`, tanpa kolom biaya baru di sana.
#
# ─── DARI MANA BIAYA PEROLEHAN TAMBAHAN BERASAL ───
#
#   a. `shipping_cost` pada PO (satu angka untuk seluruh pengiriman), dan
#   b. baris `line_type: cost` (biaya bebas: "ongkos angkut khusus", "sewa cold
#      storage"), yang bila `allocated_to` diisi hanya membebani bahan itu.
#
#   Keduanya dijumlahkan menjadi `pool_global` + `pool_per_bahan`.
#
# ─── CARA ALOKASI ───
#
#   Strategi dipilih lewat `allocation_strategy` pada Config module ini
#   (`weight` | `value` | `quantity`); default `weight` bila berat lengkap,
#   selain itu `value`.
#
#   `weight` — proporsional BERAT kiriman (qty × ingredient.weight_per_unit).
#       Ongkos kirim mengikuti berat/volume: PO yang mengirim 50 kg beras
#       bersama 100 g vanili membebani beras jauh lebih banyak daripada proporsi
#       nilainya, dan itulah yang benar secara fisik. WAJIB ada berat untuk
#       SEMUA baris — berat yang tidak diketahui DITOLAK dengan menyebut bahan
#       mana yang kosong, bukan dianggap nol (berat nol berarti bahan itu gratis
#       ongkir, klaim yang tidak boleh dibuat oleh data yang tidak ada).
#
#   `value` — proporsional NILAI (qty × unit_cost). Dipakai bila berat belum
#       diisi, atau bila biaya perolehan memang mengikuti nilai (bea masuk,
#       asuransi). Berat yang tidak lengkap TIDAK diam-diam jatuh ke sini:
#       jatuh diam-diam berarti angka salah tanpa memberi tahu siapa pun.
#
#   `quantity` — proporsional JUMLAH UNIT. Hanya masuk akal bila satuan semua
#       baris sebanding; kalau tidak, ia membebani 1 g vanili sama dengan 1 kg
#       beras. Disediakan atas permintaan, bukan default.
#
#   SISA PEMBULATAN dibebankan ke baris terbesar, bukan dibuang. Kalau
#   dibuang, Σ alokasi ≠ shipping_cost dan nilai persediaan tidak akan pernah
#   rekonsiliasi dengan faktur — selisih kecil yang muncul di setiap PO dan
#   tidak bisa dijelaskan saat audit. Baris terbesar dipilih karena ia yang
#   paling tahan terhadap satu unit pembulatan, dan tetap deterministik.
#
# ─── BENTUK SCRIPT ───
#
# Titik masuknya `execute(resource, params, ctx)` — sama seperti
# `stock_level_apply.star` dan `journalize.star`. Starlark TIDAK mengizinkan
# pernyataan `for` di level modul ("for loop not within a function"), jadi semua
# pekerjaan berada di dalam fungsi; di level modul hanya `def` dan konstanta.
# Ini pernah membuat action ini gagal 500 `script compile err` SETELAH
# permission-nya dibetulkan — kelas kegagalan yang hanya tertangkap honesty-scan
# `formspec validate`, bukan oleh review.
#
# ─── MENGAPA TIDAK MEMAKAI `resource.find` UNTUK INGIN DIKONVERSI ───
#
#   `unit` pada baris PO tidak dipakai untuk konversi: `ingredient.unit` adalah
#   satuan dasar, dan `ctx.unit.convert` (S12/item 4.6) memerlukan deklarasi
#   `unit.factors` pada entity+field. Untuk pembelian, supplier menagih dalam
#   satuan bahan itu sendiri (`kg` dibayar sebagai `kg`), jadi konversi hanya
#   diperlukan bila satuan pengiriman berbeda — kasus yang sengaja TIDAK
#   ditebak di sini; validator menolak baris yang satuannya bukan satuan dasar
#   (`unit` default `gram` mengikuti ingredient.unit).
# ─────────────────────────────────────────────────────────────────────────────


def money_of(value, default):
    """Angka dari nilai `money` ({amount, currency}) atau skalar.

    `unit_cost`/`shipping_cost`/`cost_amount` adalah nilai money; `float()`
    menolak objek seperti itu (S7: operand tidak sah = error, bukan 0).
    """
    if value == None:
        return default
    return float(amount(value))


def currency_of(value, default):
    """Mata uang dari nilai money, atau default bila nilainya kosong."""
    if value == None:
        return default
    return currency(value)


def money_like(angka, sumber, default_currency):
    """Bentuk kanonik `money`, DIBULATKAN ke skala mata uangnya.

    Pembulatan ini bukan kosmetik. Alokasi landed cost menghasilkan angka
    pecahan panjang (mis. 41.785714285714285) karena membagi biaya kirim dengan
    berat. Tanpa pembulatan, `unit_cost` tersimpan dengan 15 desimal — dan
    meskipun skema menerimanya sebagai string, itu bukan nilai uang yang sah:
    IDR tidak punya pecahan di bawah 1 (skala 0), USD 2. Akibatnya laporan
    keuangan menampilkan angka yang tidak pernah bisa dibayar siapa pun, dan
    setiap pembulatan di lapisan bawah menjadi selisih yang tidak bisa
    dijelaskan saat audit.

    Pembulatan half-up, bukan `round()` bawaan: `round()` Starlark memakai
    half-even (0.5 kadang naik, kadang turun). Itu benar untuk statistik, tetapi
    untuk uang ia menghasilkan angka yang tidak bisa diprediksi. Pada jalur ini
    nilai selalu non-negatif, jadi floor(x + 0.5) = half-up.
    """
    mata_uang = currency_of(sumber, default_currency)
    # Skala mata uang: IDR tidak punya pecahan (0), mata uang lain 2.
    # Starlark TIDAK punya operator `**` — pangkat ditulis eksplisit.
    if mata_uang == "IDR":
        skala = 0
        faktor = 1.0
    else:
        skala = 2
        faktor = 100.0
    dibulatkan = float(int(angka * faktor + 0.5)) / faktor
    if skala == 0:
        return {"amount": str(int(dibulatkan)), "currency": mata_uang}
    if dibulatkan == int(dibulatkan):
        return {"amount": str(int(dibulatkan)), "currency": mata_uang}
    return {"amount": ("%." + str(skala) + "f") % dibulatkan, "currency": mata_uang}


def field_of(row, name, default):
    """Baca field dari baris child.

    Baris child datang sebagai dict ATAU objek FieldMap tergantung jalur
    hidrasinya (`gl_balance_update.star` mencatat perbedaan yang sama). Membaca
    satu bentuk saja membuat script benar di satu jalur dan `None` di jalur
    lain — nilai yang terlihat seperti "field kosong", bukan error.
    """
    if type(row) == "dict":
        value = row.get(name)
    else:
        value = getattr(row, name, None)
    if value == None:
        return default
    return value


def berat_lengkap(baris_bahan):
    """True bila SEMUA baris punya berat per satuan yang diketahui.

    Diperiksa sekali di depan supaya pilihan default (`weight` vs `value`)
    deterministik, bukan bergantung urutan baris.
    """
    for b in baris_bahan:
        if b["berat"] == None or b["berat"] <= 0:
            return False
    return True


def berat_of(resource, ingredient_id):
    """Berat per satuan dasar dari master bahan, atau None bila belum diisi.

    Dibaca lewat `resource.fetch` (lapisan entity, bukan SQL). `None` sengaja
    dibedakan dari 0: berat 0 berarti "tidak menyerap ongkos kirim", sedangkan
    `None` berarti "tidak diketahui" — dan keduanya harus diperlakukan berbeda.
    """
    if ingredient_id == "" or ingredient_id == None:
        return None
    bahan = resource.fetch("cafe-stock.ingredient", ingredient_id)
    if bahan == None:
        return None
    w = bahan.field.weight_per_unit
    if w == None:
        return None
    return float(w)


def collect_pools(po):
    """Pisahkan kolam biaya: umum (proporsional) dan per-bahan tertentu."""
    pool_umum = money_of(po.field.shipping_cost, 0)
    per_bahan = {}
    for baris in po.field.lines or []:
        if field_of(baris, "line_type", "ingredient") != "cost":
            continue
        nilai = money_of(field_of(baris, "cost_amount", None), 0)
        if nilai == 0:
            continue
        kunci = field_of(baris, "allocated_to", "")
        if kunci == "":
            pool_umum = pool_umum + nilai
        else:
            per_bahan[kunci] = per_bahan.get(kunci, 0) + nilai
    return pool_umum, per_bahan


def execute(resource, params, ctx):
    po = resource

    # ── Kumpulkan baris bahan dan nilainya (dasar alokasi) ──
    baris_bahan = []
    total_nilai = 0.0
    for baris in po.field.lines or []:
        if field_of(baris, "line_type", "ingredient") == "cost":
            continue
        qty = float(field_of(baris, "received_quantity", 0) or field_of(baris, "quantity", 0) or 0)
        harga = money_of(field_of(baris, "unit_cost", None), 0)
        nilai = qty * harga
        ingredient_id = field_of(baris, "ingredient_id", "")
        baris_bahan.append({
            "row": baris,
            "ingredient_id": ingredient_id,
            "qty": qty,
            "harga": harga,
            "nilai": nilai,
            "berat": berat_of(resource, ingredient_id),
            "unit_cost": field_of(baris, "unit_cost", None),
            "alokasi": 0.0,
        })
        total_nilai = total_nilai + nilai

    if len(baris_bahan) == 0:
        fail("PO tidak punya baris bahan - tidak ada yang bisa diterima")
    if total_nilai <= 0:
        fail("PO belum punya harga satuan - isi unit_cost sebelum menerima barang")

    pool_umum, per_bahan = collect_pools(po)

    # ── Pilih dasar alokasi (item 10.6) ──
    strategi = ctx.config.get("allocation_strategy") or ""
    if strategi == "":
        strategi = "weight" if berat_lengkap(baris_bahan) else "value"
    if strategi == "weight" and not berat_lengkap(baris_bahan):
        # Diminta eksplisit tetapi beratnya tidak lengkap: MENOLAK, bukan
        # diam-diam pindah ke `value`. Berpindah diam-diam menghasilkan angka
        # yang salah tanpa satu pun tanda — kelas kegagalan yang paling mahal
        # karena tidak ada yang tahu harus memeriksa apa.
        kosong = []
        for b in baris_bahan:
            if b["berat"] == None or b["berat"] <= 0:
                kosong.append(str(b["ingredient_id"]))
        fail("FORMSPEC.PURCHASE.WEIGHT_MISSING: allocation_strategy=weight tapi berat per satuan belum diisi untuk bahan: " + ", ".join(kosong) + " - isi ingredient.weight_per_unit, atau pilih allocation_strategy: value secara eksplisit")

    for b in baris_bahan:
        if strategi == "weight":
            b["_bobot"] = b["qty"] * b["berat"]
        elif strategi == "quantity":
            b["_bobot"] = b["qty"]
        else:
            b["_bobot"] = b["nilai"]

    total_bobot = 0.0
    for b in baris_bahan:
        total_bobot = total_bobot + b["_bobot"]
    if total_bobot <= 0:
        fail("gaya alokasi '" + strategi + "' menghasilkan bobot nol untuk PO " + str(po.field.number) + " - tidak ada dasar untuk membagi biaya kirim")

    # ── Fase alokasi: hitung SEMUA porsi sebelum menulis apa pun ──
    #
    # Dua fase, bukan satu: sisa pembulatan hanya bisa dihitung setelah seluruh
    # porsi diketahui, dan porsi harus menjumlah persis ke pool_umum. Menulis
    # pergerakan di dalam loop alokasi membuat sisa tidak bisa lagi dibebankan.
    porsi_total = 0.0
    idx_terbesar = 0
    for i in range(len(baris_bahan)):
        b = baris_bahan[i]
        porsi = pool_umum * b["_bobot"] / total_bobot
        b["alokasi"] = porsi
        porsi_total = porsi_total + porsi
        if b["_bobot"] > baris_bahan[idx_terbesar]["_bobot"]:
            idx_terbesar = i

    sisa = pool_umum - porsi_total
    if sisa != 0:
        baris_bahan[idx_terbesar]["alokasi"] = baris_bahan[idx_terbesar]["alokasi"] + sisa

    for b in baris_bahan:
        if b["ingredient_id"] in per_bahan:
            b["alokasi"] = b["alokasi"] + per_bahan[b["ingredient_id"]]

    # ── Fase tulis: satu pergerakan stok per baris bahan ──
    dibuat = 0
    for b in baris_bahan:
        if b["qty"] <= 0:
            continue
        biaya_per_satuan = b["harga"] + (b["alokasi"] / b["qty"])
        resource.create("cafe-stock.stock-movement", {
            "transaction_date": po.field.transaction_date,
            "branch_id": po.field.branch_id,
            "ingredient_id": b["ingredient_id"],
            "direction": "in",
            "quantity": b["qty"],
            "unit_cost": money_like(biaya_per_satuan, b["unit_cost"], "IDR"),
            "source": "purchase",
            "source_ref": po.field.number,
            "note": "Penerimaan PO " + str(po.field.number),
        })
        dibuat = dibuat + 1

    if dibuat == 0:
        fail("tidak ada baris dengan jumlah diterima > 0 - pergerakan stok tidak dibuat")

    return {
        "number": po.field.number,
        "movements": dibuat,
        "landed_cost": money_like(pool_umum, po.field.shipping_cost, "IDR"),
    }
