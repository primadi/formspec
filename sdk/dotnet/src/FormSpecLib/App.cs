using System.Collections.Concurrent;
using System.Net;
using System.Net.Sockets;
using System.Text;

namespace FormSpec;

/// <summary>
/// The lib-formspec-dotnet listener: accepts POST /invoke/{module}/{entity}/{action}
/// from formspec-sidecar and dispatches to registered handlers. Also answers
/// GET /health for the sidecar's app monitor.
///
/// <example>
/// <code>
/// var app = new App();
/// app.Handle("billing.invoice.approve", async (inv, ctx) =>
/// {
///     await ctx.Lock().AcquireAsync("invoice:" + inv.ResourceId, 30);
///     return new ActionResult(new { approved_at = DateTime.UtcNow }, "approved");
/// });
/// await app.RunAsync();
/// </code>
/// </example>
/// </summary>
public sealed class App : IAsyncDisposable
{
    private readonly ConcurrentDictionary<string, Func<Invocation, Ctx, Task<object?>>> _handlers = new();
    private readonly int _port;
    private readonly Ctx _ctx;
    private TcpListener? _listener;
    private CancellationTokenSource? _cts;

    /// <summary>
    /// Creates an App that listens on localhost.
    /// Socket paths from FORMA_APP_SOCKET / FORMA_SIDECAR_SOCKET env vars.
    /// </summary>
    public App() : this(DetectPort(), DetectSidecarEndpoint()) { }

    /// <param name="listenPort">localhost TCP port to listen on</param>
    /// <param name="sidecarEndpoint">"unix://..." or "http://..."</param>
    public App(int listenPort, string sidecarEndpoint)
    {
        _port = listenPort;
        _ctx = new Ctx(new SidecarClient(sidecarEndpoint));
    }

    /// <summary>Register a handler for "module.entity.action".</summary>
    public void Handle(string action, Func<Invocation, Ctx, Task<object?>> handler)
    {
        if (!_handlers.TryAdd(action, handler))
            throw new FormaException($"handler for {action} already registered");
    }

    /// <summary>Register a synchronous handler for "module.entity.action".</summary>
    public void Handle(string action, Func<Invocation, Ctx, object?> handler)
    {
        Handle(action, (inv, ctx) => Task.FromResult(handler(inv, ctx)));
    }

    /// <summary>Starts the listener; blocks until the server is stopped.</summary>
    public async Task RunAsync()
    {
        _cts = new CancellationTokenSource();
        _listener = new TcpListener(IPAddress.Loopback, _port);
        _listener.Start();

        Console.Error.WriteLine($"[lib-formspec-dotnet] listening on http://localhost:{_port}/");

        try
        {
            while (!_cts.Token.IsCancellationRequested)
            {
                var client = await _listener.AcceptTcpClientAsync(_cts.Token);
                // Fire-and-forget each connection (sequential processing happens
                // naturally as the sidecar serializes invocations)
                _ = ServeOneAsync(client, _cts.Token);
            }
        }
        catch (OperationCanceledException) { }
        finally
        {
            _listener.Stop();
        }
    }

    public ValueTask DisposeAsync()
    {
        _cts?.Cancel();
        _cts?.Dispose();
        return ValueTask.CompletedTask;
    }

    private async Task ServeOneAsync(TcpClient tcpClient, CancellationToken ct)
    {
        using (tcpClient)
        using (var stream = tcpClient.GetStream())
        using (var reader = new StreamReader(stream, Encoding.UTF8, leaveOpen: true))
        {
            try
            {
                // Read request line
                var requestLine = await reader.ReadLineAsync(ct);
                if (string.IsNullOrEmpty(requestLine)) return;

                var parts = requestLine.Split(' ');
                var method = parts[0];
                var path = parts[1];

                // Read headers
                int contentLength = -1;
                string? line;
                while (!string.IsNullOrEmpty(line = await reader.ReadLineAsync(ct)))
                {
                    var lower = line.ToLowerInvariant();
                    if (lower.StartsWith("content-length:"))
                        contentLength = int.Parse(line[(line.IndexOf(':') + 1)..].Trim());
                }

                // Read body
                string body;
                if (contentLength >= 0)
                {
                    var buf = new char[contentLength];
                    var totalRead = 0;
                    while (totalRead < contentLength)
                    {
                        var n = await reader.ReadAsync(buf, totalRead, contentLength - totalRead, ct);
                        if (n <= 0) break;
                        totalRead += n;
                    }
                    body = new string(buf, 0, totalRead);
                }
                else
                {
                    body = await reader.ReadToEndAsync(ct);
                }

                await HandleRequestAsync(stream, method, path, body, ct);
            }
            catch (Exception ex)
            {
                await RespondAsync(stream, 500, new Dictionary<string, object?> { ["error"] = ex.Message });
            }
        }
    }

    private async Task HandleRequestAsync(
        NetworkStream stream, string method, string path, string body, CancellationToken ct)
    {
        if (method == "GET" && path == "/health")
        {
            await RespondAsync(stream, 200, new Dictionary<string, object?>
            {
                ["status"] = "healthy",
                ["handlers"] = _handlers.Count
            });
            return;
        }

        if (method != "POST" || !path.StartsWith("/invoke/"))
        {
            await RespondAsync(stream, 404, new Dictionary<string, object?>
            {
                ["error"] = "expected POST /invoke/{module}/{entity}/{action}"
            });
            return;
        }

        // Parse /invoke/{module}/{entity}/{action}
        var segments = path.Split('/', 5, StringSplitOptions.RemoveEmptyEntries);
        if (segments.Length < 4)
        {
            await RespondAsync(stream, 400, new Dictionary<string, object?>
            {
                ["error"] = $"invalid invoke path: {path}"
            });
            return;
        }

        var module = segments[1];
        var entity = segments[2];
        var action = segments[3];
        var actionKey = $"{module}.{entity}.{action}";

        if (!_handlers.TryGetValue(actionKey, out var handler))
        {
            await RespondAsync(stream, 404, new Dictionary<string, object?>
            {
                ["error"] = $"no handler for {actionKey}"
            });
            return;
        }

        var bodyObj = string.IsNullOrWhiteSpace(body)
            ? new Dictionary<string, object?>()
            : JsonCodec.Decode(body) ?? new();

        var inv = Invocation.FromRequest(module, entity, action, bodyObj);
        var result = await handler(inv, _ctx);

        Dictionary<string, object?> wireResult;
        if (result is ActionResult ar)
        {
            wireResult = ar.ToWire();
        }
        else
        {
            wireResult = new() { ["data"] = result };
        }

        await RespondAsync(stream, 200, wireResult);
    }

    private static async Task RespondAsync(
        NetworkStream stream, int status, Dictionary<string, object?> body)
    {
        var json = JsonCodec.Encode(body);
        var bytes = Encoding.UTF8.GetBytes(json);

        var header = $"HTTP/1.1 {status} {(status == 200 ? "OK" : "Error")}\r\n" +
                     "Content-Type: application/json\r\n" +
                     $"Content-Length: {bytes.Length}\r\n" +
                     "Connection: close\r\n\r\n";

        await stream.WriteAsync(Encoding.UTF8.GetBytes(header));
        await stream.WriteAsync(bytes);
        await stream.FlushAsync();
    }

    private static int DetectPort()
    {
        var env = Environment.GetEnvironmentVariable("FORMA_APP_SOCKET");
        if (env?.StartsWith("http://") == true)
        {
            try
            {
                var uri = new Uri(env);
                if (uri.Port > 0) return uri.Port;
            }
            catch { }
        }
        return 9802; // default
    }

    private static string DetectSidecarEndpoint()
    {
        return Environment.GetEnvironmentVariable("FORMA_SIDECAR_SOCKET")
            ?? "unix:///tmp/formspec/sidecar.sock";
    }
}
