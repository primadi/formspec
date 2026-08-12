namespace FormSpec;

/// <summary>
/// One action invocation from the sidecar — the wire form of the engine's
/// ExecuteParams (docs/runtimes/04-formspec-sidecar.md §4.2).
/// </summary>
public sealed record Invocation(
    string Module,
    string Entity,
    string Action,
    string ResourceId,
    Dictionary<string, object?> Resource,
    Dictionary<string, object?> Params,
    string UserId)
{
    internal static Invocation FromRequest(
        string module, string entity, string action,
        Dictionary<string, object?> body)
    {
        return new Invocation(
            module,
            entity,
            action,
            (string)body.GetValueOrDefault("resource_id", "")!,
            (Dictionary<string, object?>)body.GetValueOrDefault("resource", new())!,
            (Dictionary<string, object?>)body.GetValueOrDefault("params", new())!,
            (string)body.GetValueOrDefault("user_id", "")!);
    }
}
