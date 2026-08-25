# Handler untuk tracked async job (todo 7.13) — call: async + track: true.
# Melaporkan progres via ctx.job.progress; hasil dikembalikan lewat ok() dan
# muncul di event `completed` kanal jobs.
def execute(resource, params, ctx):
    total = params.get("rows", 100)
    for i in range(1, 6):
        ctx.job.progress(i * 20, "processing batch %d/5" % i)
    return ok({
        "rows": total,
        "format": "pdf",
        "generated_by": ctx.user.id,
    })