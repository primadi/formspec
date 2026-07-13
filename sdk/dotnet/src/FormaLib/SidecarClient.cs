using System.Net.Sockets;
using System.Text;

namespace Forma;

/// <summary>
/// HTTP client to the forma-sidecar local listener — unix domain socket
/// (default) or localhost TCP.
/// </summary>
internal sealed class SidecarClient
{
    private readonly string? _socketPath;
    private readonly string? _tcpHost;
    private readonly int _tcpPort;
    private readonly int _timeoutMillis;

    /// <param name="endpoint">"unix:///tmp/forma/sidecar.sock" or "http://localhost:PORT"</param>
    public SidecarClient(string endpoint, int timeoutMillis = 30_000)
    {
        _timeoutMillis = timeoutMillis;
        if (endpoint.StartsWith("unix://"))
        {
            _socketPath = endpoint["unix://".Length..];
        }
        else if (endpoint.StartsWith("http://"))
        {
            var stripped = endpoint["http://".Length..].TrimEnd('/');
            var colon = stripped.LastIndexOf(':');
            if (colon < 0)
            {
                _tcpHost = stripped;
                _tcpPort = 80;
            }
            else
            {
                _tcpHost = stripped[..colon];
                _tcpPort = int.Parse(stripped[(colon + 1)..]);
            }
        }
        else
        {
            throw new FormaException(
                $"sidecar endpoint {endpoint}: unsupported scheme (want unix:// or http://)");
        }
    }

    public Dictionary<string, object?> Post(string path, Dictionary<string, object?> body)
    {
        var jsonBody = JsonCodec.Encode(body);
        var request = BuildHttpRequest("POST", path, jsonBody);

        using var socket = CreateSocket();
        socket.Connect(GetEndPoint());
        socket.Send(Encoding.UTF8.GetBytes(request));
        return ParseHttpResponse(socket);
    }

    private Socket CreateSocket()
    {
        if (_socketPath is not null)
        {
            return new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified);
        }
        return new Socket(AddressFamily.InterNetwork, SocketType.Stream, ProtocolType.Tcp)
        {
            ReceiveTimeout = _timeoutMillis,
            SendTimeout = _timeoutMillis
        };
    }

    private System.Net.EndPoint GetEndPoint()
    {
        if (_socketPath is not null)
        {
            return new UnixDomainSocketEndPoint(_socketPath);
        }
        var addr = System.Net.Dns.GetHostAddresses(_tcpHost!)[0];
        return new System.Net.IPEndPoint(addr, _tcpPort);
    }

    private static string BuildHttpRequest(string method, string path, string body)
    {
        var sb = new StringBuilder();
        sb.Append(method).Append(' ').Append(path).Append(" HTTP/1.1\r\n");
        sb.Append("Host: localhost\r\n");
        sb.Append("Content-Type: application/json\r\n");
        sb.Append("Content-Length: ").Append(Encoding.UTF8.GetByteCount(body)).Append("\r\n");
        sb.Append("Connection: close\r\n");
        sb.Append("\r\n");
        sb.Append(body);
        return sb.ToString();
    }

    private static Dictionary<string, object?> ParseHttpResponse(Socket socket)
    {
        using var ns = new NetworkStream(socket, ownsSocket: true);
        using var reader = new StreamReader(ns, Encoding.UTF8);

        // Parse status line
        var statusLine = reader.ReadLine();
        if (string.IsNullOrEmpty(statusLine))
            throw new FormaException("empty response from sidecar");

        var parts = statusLine.Split(' ', 3);
        var statusCode = int.Parse(parts[1]);

        // Parse headers
        int contentLength = -1;
        string? line;
        while (!string.IsNullOrEmpty(line = reader.ReadLine()))
        {
            var lower = line.ToLowerInvariant();
            if (lower.StartsWith("content-length:"))
            {
                contentLength = int.Parse(line[(line.IndexOf(':') + 1)..].Trim());
            }
        }

        // Parse body
        string body;
        if (contentLength >= 0)
        {
            var buf = new char[contentLength];
            var totalRead = 0;
            while (totalRead < contentLength)
            {
                var n = reader.Read(buf, totalRead, contentLength - totalRead);
                if (n <= 0) break;
                totalRead += n;
            }
            body = new string(buf, 0, totalRead);
        }
        else
        {
            body = reader.ReadToEnd();
        }

        var decoded = JsonCodec.Decode(body);
        if (statusCode != 200)
        {
            var errMsg = decoded?.GetValueOrDefault("error") is string err
                ? err
                : $"HTTP {statusCode}";
            throw new FormaException($"sidecar call {path}: {errMsg}");
        }

        return decoded ?? new();
    }
}
