# lib-formspec-dotnet

Thin .NET client for `formspec-sidecar` (docs/runtimes/04-formspec-sidecar.md).
.NET 8.0+, stdlib only — no NuGet dependencies beyond the BCL.

```bash
dotnet add package FormSpec.LibForma
```

```csharp
using FormSpec;

var app = new App();  // sockets from FORMA_APP_SOCKET / FORMA_SIDECAR_SOCKET

app.Handle("billing.invoice.approve", async (inv, ctx) =>
{
    var rows = await ctx.Db().QueryAsync("SELECT ...");          // proxied to the sidecar engine
    await ctx.Cache().Named("session-cache").GetAsync("key");    // named datastore

    return new ActionResult(
        new { approved_at = DateTime.UtcNow.ToString("O") },
        "approved")
        .WithEvent("invoice.approved", new { id = inv.ResourceId });
});

await app.RunAsync();  // sidecar calls POST /invoke/billing/invoice/approve
```

Handlers may return an `ActionResult`, plain data (becomes `data`), or
throw — exceptions surface to the sidecar as HTTP 500 with the message.

See [examples/Program.cs](examples/Program.cs) for a runnable app, and
[../README.md](../README.md) for the wire contract shared by all SDKs.
