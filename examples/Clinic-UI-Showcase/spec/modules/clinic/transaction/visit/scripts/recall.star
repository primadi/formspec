# recall.star — Tarik pasien kembali ke antrian
#
# Dipanggil saat petugas salah memanggil pasien.
# Guard state machine (data.get("diagnosis", None) == None) memastikan
# diagnosis belum terisi — jadi script ini bisa langsung eksekusi.

def execute(resource, params, ctx):
    resource.set("status", "waiting")
    resource.set("started_at", None)
    resource.save()
    ctx.log.info("visit.recalled", {"visit_id": resource.id})
    return ok()
