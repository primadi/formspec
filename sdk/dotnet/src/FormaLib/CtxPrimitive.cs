namespace Forma;

/// <summary>
/// One ctx primitive handle (db/cache/lock/…), optionally bound to a named
/// datastore via <see cref="Named"/>. Operations map 1:1 to POST /ctx/{prim}/{op}.
/// </summary>
public sealed class CtxPrimitive
{
    private readonly SidecarClient _client;
    private readonly string _type;
    private readonly string _boundName;

    internal CtxPrimitive(SidecarClient client, string type) : this(client, type, "") { }
    internal CtxPrimitive(SidecarClient client, string type, string boundName)
    {
        _client = client;
        _type = type;
        _boundName = boundName;
    }

    /// <summary>Bind to a named datastore instead of the default one.</summary>
    public CtxPrimitive Named(string name) => new(_client, _type, name);

    // ---- db ----

    /// <summary>Execute a query on the datastore.</summary>
    public List<Dictionary<string, object?>> Query(string sql, params object?[] args)
    {
        var body = new Dictionary<string, object?> { ["sql"] = sql };
        if (args.Length > 0) body["args"] = args.ToList();
        var resp = Call("query", body);
        return resp.GetValueOrDefault("data") as List<Dictionary<string, object?>> ?? new();
    }

    // ---- cache / kvstore ----

    /// <summary>Get a value by key.</summary>
    public object? Get(string key) => Call("get", new() { ["key"] = key }).GetValueOrDefault("data");

    /// <summary>Set a value by key with an optional TTL.</summary>
    public void Set(string key, object? value, int ttlSeconds = 0)
    {
        var body = new Dictionary<string, object?> { ["key"] = key, ["value"] = value };
        if (ttlSeconds > 0) body["ttl_seconds"] = ttlSeconds;
        Call("set", body);
    }

    /// <summary>Delete a key.</summary>
    public void Delete(string key) => Call("delete", new() { ["key"] = key });

    // ---- lock ----

    /// <summary>Acquire a distributed lock.</summary>
    public bool Acquire(string key, int ttlSeconds = 30)
    {
        var resp = Call("acquire", new() { ["key"] = key, ["ttl_seconds"] = ttlSeconds });
        return resp.GetValueOrDefault("ok") is true;
    }

    /// <summary>Release a distributed lock.</summary>
    public void Release(string key) => Call("release", new() { ["key"] = key });

    // ---- entity atomic operations ----

    /// <summary>Atomically merge fields into an entity record (entity/update).
    /// Uses jsonb_merge / json_patch — single SQL statement, no race condition.</summary>
    public void Update(string id, Dictionary<string, object?> fields)
    {
        var body = new Dictionary<string, object?> { ["key"] = id, ["fields"] = fields };
        Call("update", body);
    }

    /// <summary>Atomically increment a numeric field on an entity record.
    /// Single SQL statement — no read-modify-write race condition.</summary>
    public void Increment(string id, string field, double amount)
    {
        var body = new Dictionary<string, object?> { ["key"] = id, ["field"] = field, ["amount"] = amount };
        Call("increment", body);
    }

    /// <summary>Atomically decrement a numeric field on an entity record.
    /// Includes a guard against negative values. Returns the new field value.</summary>
    public double? Decrement(string id, string field, double amount)
    {
        var body = new Dictionary<string, object?> { ["key"] = id, ["field"] = field, ["amount"] = amount };
        var resp = Call("decrement", body);
        return resp.GetValueOrDefault("data") as double?;
    }

    // ---- internal ----

    private Dictionary<string, object?> Call(string op, Dictionary<string, object?> body)
    {
        if (!string.IsNullOrEmpty(_boundName))
        {
            body = new(body) { ["named"] = _boundName };
        }
        return _client.Post($"/ctx/{_type}/{op}", body);
    }
}
