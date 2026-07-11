namespace Forma;

/// <summary>
/// Structured handler result — the wire form of the engine's ExecuteResult.
/// Handlers may also return plain data, which becomes <c>data</c>.
/// </summary>
public sealed class ActionResult
{
    private readonly List<Dictionary<string, object?>> _events = new();

    public ActionResult(object? data = null, string? newState = null)
    {
        Data = data;
        NewState = newState;
    }

    public object? Data { get; }
    public string? NewState { get; }

    /// <summary>Add an event emission to the result.</summary>
    public ActionResult WithEvent(string name, object? payload = null, bool durable = false)
    {
        var evt = new Dictionary<string, object?> { ["name"] = name };
        if (payload is not null)
        {
            // Convert anonymous objects to dictionaries for JSON serialization
            evt["payload"] = ToDictionary(payload);
        }
        if (durable) evt["durable"] = true;

        var result = new ActionResult(Data, NewState);
        result._events.AddRange(_events);
        result._events.Add(evt);
        return result;
    }

    /// <summary>Serialize to the wire format expected by forma-sidecar.</summary>
    internal Dictionary<string, object?> ToWire()
    {
        var wire = new Dictionary<string, object?> { ["data"] = Data };
        if (NewState is not null) wire["new_state"] = NewState;
        if (_events.Count > 0) wire["events"] = _events;
        return wire;
    }

    private static Dictionary<string, object?> ToDictionary(object obj)
    {
        var dict = new Dictionary<string, object?>();
        foreach (var prop in obj.GetType().GetProperties())
        {
            dict[prop.Name] = prop.GetValue(obj);
        }
        return dict;
    }
}
