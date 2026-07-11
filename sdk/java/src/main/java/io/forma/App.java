package io.forma;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

import java.io.*;
import java.net.InetSocketAddress;
import java.net.URI;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.BiFunction;

/**
 * The lib-forma-java listener: accepts {@code POST /invoke/{module}/{entity}/{action}}
 * from forma-sidecar and dispatches to registered handlers. Also answers
 * {@code GET /health} for the sidecar's app monitor.
 *
 * <p>The listener binds to <strong>localhost TCP</strong> (not a unix socket)
 * because Java's com.sun.net.httpserver does not natively support Unix domain
 * sockets. The sidecar communicates via TCP when --app-endpoint points to
 * localhost.
 *
 * <pre>{@code
 * var app = new App();
 * app.handle("billing.invoice.approve", (inv, ctx) -> {
 *     ctx.lock().acquire("invoice:" + inv.resourceId(), 30);
 *     // ... business logic ...
 *     return new ActionResult(Map.of("approved_at", Instant.now().toString()), "approved")
 *             .withEvent("invoice.approved", Map.of("id", inv.resourceId()), true);
 * });
 * app.run();
 * }</pre>
 *
 * The server is intentionally single-threaded and sequential: the sidecar
 * serializes handler invocations per action, and each request is served to
 * completion before the next accept.
 */
public final class App implements AutoCloseable {
    private final Map<String, BiFunction<Invocation, Ctx, Object>> handlers = new ConcurrentHashMap<>();
    private final int port;
    private final Ctx ctx;
    private HttpServer server;

    /**
     * Creates an App that listens on localhost.
     *
     * <p>Socket paths are read from environment variables:
     * {@code FORMA_APP_SOCKET} (default {@code /var/run/forma/app.sock}) and
     * {@code FORMA_SIDECAR_SOCKET} (default {@code /var/run/forma/sidecar.sock}).
     * For the TCP listener, only the port portion is extracted from
     * {@code FORMA_APP_SOCKET} if it uses TCP; otherwise the default
     * {@code localhost:9801} is used.
     */
    public App() {
        this(detectPort(), detectSidecarEndpoint());
    }

    /**
     * @param listenPort       localhost TCP port to listen on
     * @param sidecarEndpoint  "unix://..." or "http://..." for the sidecar client
     */
    public App(int listenPort, String sidecarEndpoint) {
        this.port = listenPort;
        this.ctx = new Ctx(new SidecarClient(sidecarEndpoint));
    }

    /** Register a handler for "module.entity.action". */
    public void handle(String action, BiFunction<Invocation, Ctx, Object> handler) {
        if (handlers.putIfAbsent(action, handler) != null) {
            throw new FormaException("handler for " + action + " already registered");
        }
    }

    /** Starts the listener; blocks until the server is stopped. */
    public void run() {
        try {
            server = HttpServer.create(new InetSocketAddress(port), 0);
            server.createContext("/", this::serve);
            server.setExecutor(null); // use the default (single-threaded) executor
            server.start();
            System.err.println("[lib-forma-java] listening on http://localhost:" + port + "/");
        } catch (IOException e) {
            throw new FormaException("failed to start server on port " + port, e);
        }
    }

    /** Stop the server. */
    @Override
    public void close() {
        if (server != null) {
            server.stop(1);
        }
    }

    // ---- internal ----

    private void serve(HttpExchange exchange) {
        try {
            var method = exchange.getRequestMethod();
            var path = URI.create(exchange.getRequestURI().toString()).getPath();

            if ("GET".equals(method) && "/health".equals(path)) {
                respond(exchange, 200, Map.of("status", "healthy", "handlers", handlers.size()));
                return;
            }

            if (!"POST".equals(method) || !path.startsWith("/invoke/")) {
                respond(exchange, 404, Map.of("error",
                        "expected POST /invoke/{module}/{entity}/{action}"));
                return;
            }

            // Parse /invoke/{module}/{entity}/{action}
            var segments = path.split("/", 5);
            if (segments.length < 5) {
                respond(exchange, 400, Map.of("error", "invalid invoke path: " + path));
                return;
            }
            var module = segments[2];
            var entity = segments[3];
            var action = segments[4];
            var actionKey = module + "." + entity + "." + action;

            var handler = handlers.get(actionKey);
            if (handler == null) {
                respond(exchange, 404, Map.of("error", "no handler for " + actionKey));
                return;
            }

            // Read request body
            var bodyBytes = exchange.getRequestBody().readAllBytes();
            var bodyStr = new String(bodyBytes, "UTF-8");
            @SuppressWarnings("unchecked")
            Map<String, Object> body = bodyStr.isBlank()
                    ? new LinkedHashMap<>()
                    : (Map<String, Object>) JsonCodec.decode(bodyStr);

            var inv = Invocation.fromRequest(module, entity, action, body);
            var result = handler.apply(inv, ctx);

            // Serialize response
            Map<String, Object> wireResult;
            if (result instanceof ActionResult ar) {
                wireResult = ar.toWire();
            } else {
                wireResult = new LinkedHashMap<>();
                wireResult.put("data", result);
            }

            respond(exchange, 200, wireResult);

        } catch (Exception e) {
            try {
                respond(exchange, 500, Map.of("error", e.getMessage() != null ? e.getMessage() : "internal error"));
            } catch (Exception ignored) {
                // response already committed
            }
        }
    }

    private void respond(HttpExchange exchange, int status, Object body) throws IOException {
        var json = JsonCodec.encode(body);
        var bytes = json.getBytes("UTF-8");
        exchange.getResponseHeaders().set("Content-Type", "application/json");
        exchange.sendResponseHeaders(status, bytes.length);
        try (var os = exchange.getResponseBody()) {
            os.write(bytes);
        }
    }

    private static int detectPort() {
        var socketEnv = System.getenv("FORMA_APP_SOCKET");
        if (socketEnv != null && socketEnv.startsWith("http://")) {
            try {
                var uri = URI.create(socketEnv);
                if (uri.getPort() > 0) return uri.getPort();
            } catch (Exception ignored) {
            }
        }
        return 9801; // default
    }

    private static String detectSidecarEndpoint() {
        var env = System.getenv("FORMA_SIDECAR_SOCKET");
        return env != null ? env : "unix:///var/run/forma/sidecar.sock";
    }
}
