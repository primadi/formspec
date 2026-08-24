# fill_order_unit_price.star
# Hook before create/update (entity.yaml → spec.hooks) — auto-fill unit_price
# dari menu_item_id yang dipilih. line_total dan total_amount dihitung
# server-side via computed field (entity.yaml), jadi script ini hanya mengisi
# harga satuan bila belum diisi klien.
#
# Contoh pemakaian:
#   hooks:
#     - on: before
#       action: create
#       impl: { type: script_ref, ref: fill_order_unit_price }

def execute(resource, params, ctx):
    items = resource.field.items or []
    new_items = []
    for item in items:
        menu_id = item.get("menu_item_id")
        price = item.get("unit_price")
        # Auto-fill harga dari menu item bila belum diisi klien
        if menu_id and not price:
            menu = resource.fetch("cafe-master.menu-item", menu_id)
            price = menu.field.price
        # Salin semua field item, lalu isi unit_price
        new_item = {}
        for key in item:
            new_item[key] = item[key]
        new_item["unit_price"] = price
        new_items.append(new_item)
    resource.set("items", new_items)
    return ok()