# Notifikasi fire-and-forget (call: async).
# Dipanggil sebagai Service action async — tidak ada hasil yang dinanti.
def execute(resource, params, ctx):
    # Dalam contoh nyata, ini mengirim pesan via ctx.queue/pubsub.
    # Di sini hanya log (best-effort).
    ctx.log.info("notify_async: " + str(params))
    return ok({})
