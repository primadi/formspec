using Forma;

// Example lib-forma-dotnet app: the business-logic side of an
// impl: {type: sidecar} action. Run inside a pod next to forma-sidecar:
//
//   FORMA_APP_SOCKET=http://localhost:9802 \
//   FORMA_SIDECAR_SOCKET=unix:///var/run/forma/sidecar.sock \
//   dotnet run --project examples

var app = new App();

app.Handle("billing.invoice.approve", async (Invocation inv, Ctx ctx) =>
{
    var lockKey = $"invoice:{inv.ResourceId}";
    if (!await Task.Run(() => ctx.Lock().Acquire(lockKey, 30)))
    {
        throw new InvalidOperationException("invoice is being processed by someone else");
    }

    try
    {
        var status = inv.Resource.GetValueOrDefault("status") as string;
        if (status != "draft")
        {
            throw new InvalidOperationException("only draft invoices can be approved");
        }

        return new ActionResult(
            new
            {
                approved_at = DateTime.UtcNow.ToString("O"),
                note = inv.Params.GetValueOrDefault("note", "")
            },
            "approved")
            .WithEvent("invoice.approved", new { id = inv.ResourceId }, durable: true);
    }
    finally
    {
        await Task.Run(() => ctx.Lock().Release(lockKey));
    }
});

await app.RunAsync();
