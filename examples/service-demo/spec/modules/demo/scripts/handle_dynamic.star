# Handler untuk dynamic subscription (todo 7.3.4) — dibuat runtime lewat admin
# panel, bukan manifest. Sandbox Starlark tanpa filesystem/network — bukti
# eksekusi lewat ctx.log (tercatat di ScriptResult) + return ok.
def execute(resource, params, ctx):
    ctx.log.info("dynamic_subscription_fired", {
        "product": params.get("name", "unknown"),
    })
    return ok({"handled": True, "product": params.get("name", "")})