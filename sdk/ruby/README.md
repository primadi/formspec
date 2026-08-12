# lib-formspec-ruby

Thin Ruby client for `formspec-sidecar` (docs/runtimes/04-formspec-sidecar.md).
Ruby 3.0+, stdlib only — no gems beyond the standard library.

```bash
gem install lib-formspec
```

```ruby
require "lib-formspec"

app = FormSpec::App.new  # sockets from FORMA_APP_SOCKET / FORMA_SIDECAR_SOCKET

app.handle("billing.invoice.approve") do |inv, ctx|
  rows = ctx.db.query("SELECT ...")                   # proxied to the sidecar engine
  ctx.cache.named("session-cache").get("key")         # named datastore

  FormSpec::ActionResult.new(
    { approved_at: Time.now.iso8601 },
    new_state: "approved"
  ).with_event("invoice.approved", { id: inv.resource_id })
end

app.run  # blocks; sidecar calls POST /invoke/billing/invoice/approve
```

Handlers may return an `ActionResult`, plain data (becomes `data`), or
raise — exceptions surface to the sidecar as HTTP 500 with the message.

See [examples/app.rb](examples/app.rb) for a runnable app, and
[../README.md](../README.md) for the wire contract shared by all SDKs.
