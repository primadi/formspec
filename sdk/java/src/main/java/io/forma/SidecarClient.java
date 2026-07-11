package io.forma;

import java.io.*;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.net.UnixDomainSocketAddress;
import java.nio.channels.SocketChannel;
import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * HTTP client to the forma-sidecar local listener — unix domain socket
 * (default) or localhost TCP.
 *
 * Uses raw sockets (like Python's _UnixHTTPConnection) instead of
 * java.net.http.HttpClient, because HttpClient does not natively support
 * Unix domain sockets without custom SelectorProvider plumbing.
 */
public final class SidecarClient {
    private final String socketPath;
    private final String tcpHost;
    private final int tcpPort;
    private final int timeoutMillis;

    /**
     * @param endpoint "unix:///var/run/forma/sidecar.sock" or "http://localhost:PORT"
     */
    public SidecarClient(String endpoint) {
        this(endpoint, 30_000);
    }

    /**
     * @param endpoint      "unix:///var/run/forma/sidecar.sock" or "http://localhost:PORT"
     * @param timeoutMillis request timeout in milliseconds
     */
    public SidecarClient(String endpoint, int timeoutMillis) {
        this.timeoutMillis = timeoutMillis;
        if (endpoint.startsWith("unix://")) {
            this.socketPath = endpoint.substring("unix://".length());
            this.tcpHost = null;
            this.tcpPort = -1;
        } else if (endpoint.startsWith("http://")) {
            this.socketPath = null;
            var stripped = endpoint.replaceAll("^http://", "").replaceAll("/$", "");
            var colon = stripped.lastIndexOf(':');
            if (colon < 0) {
                this.tcpHost = stripped;
                this.tcpPort = 80;
            } else {
                this.tcpHost = stripped.substring(0, colon);
                this.tcpPort = Integer.parseInt(stripped.substring(colon + 1));
            }
        } else {
            throw new FormaException(
                    "sidecar endpoint " + endpoint + ": unsupported scheme (want unix:// or http://)");
        }
    }

    /**
     * POST to a sidecar endpoint.
     *
     * @param path the request path (e.g., "/ctx/db/query")
     * @param body the JSON-serializable request body
     * @return decoded JSON response as a Map
     */
    public Map<String, Object> post(String path, Map<String, Object> body) {
        var jsonBody = JsonCodec.encode(body);
        var request = buildHttpRequest("POST", path, jsonBody);

        try (var sock = createSocket()) {
            var os = new BufferedOutputStream(sock.getOutputStream());
            var is = new BufferedInputStream(sock.getInputStream());

            os.write(request.getBytes("UTF-8"));
            os.flush();

            return parseHttpResponse(is);
        } catch (FormaException e) {
            throw e;
        } catch (Exception e) {
            throw new FormaException("sidecar call " + path + ": " + e.getMessage(), e);
        }
    }

    private Socket createSocket() throws IOException {
        if (socketPath != null) {
            // Java 16+ Unix domain socket
            var addr = UnixDomainSocketAddress.of(Path.of(socketPath));
            var ch = SocketChannel.open(addr);
            var sock = ch.socket();
            sock.setSoTimeout(timeoutMillis);
            return sock;
        } else {
            var sock = new Socket();
            sock.connect(new InetSocketAddress(tcpHost, tcpPort), timeoutMillis);
            sock.setSoTimeout(timeoutMillis);
            return sock;
        }
    }

    private String buildHttpRequest(String method, String path, String body) {
        var sb = new StringBuilder();
        sb.append(method).append(" ").append(path).append(" HTTP/1.1\r\n");
        sb.append("Host: localhost\r\n");
        sb.append("Content-Type: application/json\r\n");
        sb.append("Content-Length: ").append(body.getBytes().length).append("\r\n");
        sb.append("Connection: close\r\n");
        sb.append("\r\n");
        sb.append(body);
        return sb.toString();
    }

    @SuppressWarnings("unchecked")
    private Map<String, Object> parseHttpResponse(InputStream is) throws IOException {
        // Parse status line
        var statusLine = readLine(is);
        if (statusLine == null || statusLine.isEmpty()) {
            throw new FormaException("empty response from sidecar");
        }
        var parts = statusLine.split(" ", 3);
        int statusCode = Integer.parseInt(parts[1]);

        // Parse headers (skip them)
        int contentLength = -1;
        String line;
        while ((line = readLine(is)) != null && !line.isEmpty()) {
            var lower = line.toLowerCase();
            if (lower.startsWith("content-length:")) {
                contentLength = Integer.parseInt(line.substring(line.indexOf(':') + 1).trim());
            }
        }

        // Parse body
        String body;
        if (contentLength >= 0) {
            var buf = new byte[contentLength];
            int off = 0;
            while (off < contentLength) {
                int n = is.read(buf, off, contentLength - off);
                if (n < 0) break;
                off += n;
            }
            body = new String(buf, 0, off, "UTF-8");
        } else {
            // Chunked or no length — read until connection close
            var baos = new ByteArrayOutputStream();
            byte[] buf = new byte[4096];
            int n;
            while ((n = is.read(buf)) >= 0) {
                baos.write(buf, 0, n);
            }
            body = baos.toString("UTF-8");
        }

        var decoded = JsonCodec.decode(body);
        if (statusCode != 200) {
            var errMsg = decoded != null
                    ? (String) decoded.getOrDefault("error", "HTTP " + statusCode)
                    : "HTTP " + statusCode;
            throw new FormaException("sidecar call returned " + statusCode + ": " + errMsg);
        }
        return decoded != null ? decoded : new LinkedHashMap<>();
    }

    private String readLine(InputStream is) throws IOException {
        var baos = new ByteArrayOutputStream();
        int b;
        while ((b = is.read()) >= 0) {
            if (b == '\r') {
                int next = is.read();
                if (next != '\n' && next >= 0) {
                    baos.write(next);
                }
                break;
            }
            if (b == '\n') break;
            baos.write(b);
        }
        return baos.size() > 0 ? baos.toString("UTF-8") : null;
    }
}
