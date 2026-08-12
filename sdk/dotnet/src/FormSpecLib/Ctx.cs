namespace FormSpec;

/// <summary>
/// The ctx.* surface handed to handlers. Every method call is an HTTP call
/// back to formspec-sidecar (§4.3) — the same primitive contract Starlark scripts use.
/// </summary>
public sealed class Ctx
{
    private readonly SidecarClient _client;

    internal Ctx(SidecarClient client) => _client = client;

    public CtxPrimitive Db() => new(_client, "db");
    public CtxPrimitive Cache() => new(_client, "cache");
    public CtxPrimitive Lock() => new(_client, "lock");
    public CtxPrimitive Queue() => new(_client, "queue");
    public CtxPrimitive PubSub() => new(_client, "pubsub");
    public CtxPrimitive Storage() => new(_client, "storage");
    public CtxPrimitive KvStore() => new(_client, "kvstore");

    /// <summary>Entity primitive — access entity records via named("module/entity").</summary>
    public CtxPrimitive Entity() => new(_client, "entity");
}
