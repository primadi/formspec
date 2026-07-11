# modules/pharmacy/scripts/rx_dispense.star
# Serahkan obat — kurangi stok tiap item (uses: medicine.find, medicine.update).

def execute(resource, params, ctx):
    for item in resource.field.items:
        med = medicine.load(item.medicine_id)
        med.set("stock", med.field.stock - item.quantity)
        med.save()

    resource.set("status", "dispensed")
    resource.save()
    ctx.log.info("prescription.dispensed", {"prescription_id": resource.id})
    return ok()
