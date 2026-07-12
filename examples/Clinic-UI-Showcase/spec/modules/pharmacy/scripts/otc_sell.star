# modules/pharmacy/scripts/otc_sell.star
# Selesaikan penjualan OTC — kurangi stok tiap item (mirip rx_dispense.star)
# + hitung total di script, bukan saat create (mirip visit_complete.star).

def execute(resource, params, ctx):
    total = 0
    for item in resource.field.items:
        med = resource.fetch("medicine", item["medicine_id"])
        med.set("stock", med.field.stock - item["quantity"])
        med.save()
        total += item["quantity"] * item["unit_price"]

    resource.set("total", total)
    resource.set("status", "completed")
    resource.save()
    ctx.log.info("otc_sale.completed", {"otc_sale_id": resource.id, "total": total})
    return ok()
