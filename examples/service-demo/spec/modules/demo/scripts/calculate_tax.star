# Kalkulator pajak — komputasi stateless murni.
# Dipanggil sebagai Service action (kind: Service, impl: script_ref).
# Tidak ada resource record — hanya params {amount, rate}.
def execute(resource, params, ctx):
    amount = params.get("amount", 0)
    rate = params.get("rate", 0)
    tax = amount * rate
    return ok({"amount": amount, "rate": rate, "tax": tax, "total": amount + tax})
