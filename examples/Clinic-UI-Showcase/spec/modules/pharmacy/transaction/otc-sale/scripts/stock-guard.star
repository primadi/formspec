# modules/pharmacy/scripts/otc_sale_stock_guard.star
# hooks: before/create on otc-sale — reject the sale before the row ever
# exists if any line item's quantity exceeds current stock.

def execute(resource, params, ctx):
    items = resource.field.items
    if items == None:
        return ok()
    for item in items:
        med = resource.fetch("medicine", item["medicine_id"])
        if med.field.stock < item["quantity"]:
            return fail("stok obat tidak mencukupi untuk salah satu item")
    return ok()
