namespace FormSpec;

/// <summary>Transport failure or sidecar-reported error.</summary>
public sealed class FormaException : Exception
{
    public FormaException(string message) : base(message) { }
    public FormaException(string message, Exception inner) : base(message, inner) { }
}
