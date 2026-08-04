# modules/arisan-field/scripts/contribution_validate.star
#
# Aksi `validate` pada entity contribution (arisan-field).
# Mencocokkan iuran (contribution) dengan mutasi bank (bank-mutation) yang
# diberikan lewat params.mutation_id:
#   - hanya iuran berstatus pending yang boleh divalidasi
#   - mutasi harus milik rekening grup yang sama (group_id sama)
#   - nominal mutasi harus sama dengan nominal iuran
# Lalu:
#   - iuran: pending -> validated, matched_mutation_id diisi
#   - mutasi: unmatched -> matched, matched_contribution_id diisi
#
# params.mutation_id (wajib): id mutasi bank yang mencocoki iuran ini.
#
# CATATAN ENGINE: resource.fetch() pada entity yang punya relasi berjalan
# lewat resolveRelations yang memakai koneksi base (bukan koneksi transaksi
# aksi). Di SQLite (dev) ini DEADLOCK karena koneksi tunggal sedang dipegang
# transaksi aksi. Di PostgreSQL (produksi) tidak deadlock. Fix upstream:
# resolveRelations harus memakai txReadDB(ctx, s.db), bukan s.db.

def execute(resource, params, ctx):
    if resource.field.status != "pending":
        return fail("hanya iuran berstatus pending yang bisa divalidasi")

    if "mutation_id" not in params:
        return fail("params.mutation_id wajib diisi: pilih mutasi bank yang sesuai")

    mutation = resource.fetch("bank-mutation", params["mutation_id"])

    if mutation.field.group_id != resource.field.group_id:
        return fail("mutasi bank tidak cocok: rekening grup berbeda")
    if mutation.field.amount != resource.field.amount:
        return fail("mutasi bank tidak cocok: nominal berbeda (" + str(mutation.field.amount) + " vs " + str(resource.field.amount) + ")")

    resource.set("status", "validated")
    resource.set("matched_mutation_id", mutation.id)
    resource.save()

    mutation.set("status", "matched")
    mutation.set("matched_contribution_id", resource.id)
    mutation.save()

    ctx.log.info("contribution.validated", {
        "contribution_id": resource.id,
        "mutation_id": mutation.id,
        "amount": resource.field.amount,
    })

    return ok({"status": "validated", "mutation_id": mutation.id})
