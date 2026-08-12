# lib-formspec-python

Thin Python client for `formspec-sidecar` (docs/runtimes/04-formspec-sidecar.md).
Python ≥ 3.9, stdlib only — no dependencies.

```bash
pip install lib-formspec
```

```python
from lib_formspec import ActionResult, App, Ctx, Invocation

app = App()  # sockets from FORMA_APP_SOCKET / FORMA_SIDECAR_SOCKET

@app.action("billing.invoice.approve")
def approve(inv: Invocation, ctx: Ctx):
    rows = ctx.db().query("SELECT ...")                 # proxied to the sidecar engine
    ctx.cache().named("session-cache").get("key")       # named datastore

    return ActionResult(
        data={"approved_at": "..."}, new_state="approved",
    ).with_event("invoice.approved", {"id": inv.resource_id})

app.run()  # blocks; sidecar calls POST /invoke/billing/invoice/approve
```

Handlers may return an `ActionResult`, a plain dict/list (becomes `data`),
or raise — exceptions surface to the sidecar as HTTP 500 with the message.

See [examples/app.py](examples/app.py) for a runnable app, and
[../README.md](../README.md) for the wire contract shared by all SDKs.
