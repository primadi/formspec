<?php

declare(strict_types=1);

namespace Forma;

/**
 * The lib-forma-php listener: accepts POST /invoke/{module}/{entity}/{action}
 * from forma-sidecar and dispatches to registered handlers. Also answers
 * GET /health for the sidecar's app monitor.
 *
 *     $app = new Forma\App();
 *     $app->handle('billing.invoice.approve', function (Invocation $inv, Ctx $ctx) {
 *         $ctx->lock()->acquire('invoice:' . $inv->resourceId);
 *         // ... business logic ...
 *         return (new ActionResult(['approved_at' => date('c')], newState: 'approved'))
 *             ->withEvent('invoice.approved', ['id' => $inv->resourceId]);
 *     });
 *     $app->run();
 *
 * The server is intentionally single-threaded and sequential: the sidecar
 * serializes handler invocations per action, and each request is served to
 * completion before the next accept.
 */
final class App
{
    /** @var array<string, callable(Invocation, Ctx): (ActionResult|array<string, mixed>|null)> */
    private array $handlers = [];

    private readonly string $listen;
    private readonly Ctx $ctx;

    public function __construct(?string $listen = null, ?string $sidecarEndpoint = null)
    {
        $this->listen = $listen
            ?? 'unix://' . ((getenv('FORMA_APP_SOCKET') ?: '/var/run/forma/app.sock'));
        $sidecarEndpoint ??= 'unix://' . ((getenv('FORMA_SIDECAR_SOCKET') ?: '/var/run/forma/sidecar.sock'));
        $this->ctx = new Ctx(new SidecarClient($sidecarEndpoint));
    }

    /**
     * Register a handler for "module.entity.action".
     *
     * @param callable(Invocation, Ctx): (ActionResult|array<string, mixed>|null) $handler
     */
    public function handle(string $action, callable $handler): void
    {
        if (isset($this->handlers[$action])) {
            throw new FormaException("handler for {$action} already registered");
        }
        $this->handlers[$action] = $handler;
    }

    /** Blocks serving requests until the process is terminated. */
    public function run(): void
    {
        if (!str_starts_with($this->listen, 'unix://')) {
            throw new FormaException("listen {$this->listen}: only unix:// is supported by lib-forma-php");
        }
        $socketPath = substr($this->listen, strlen('unix://'));

        if (file_exists($socketPath)) {
            @unlink($socketPath); // stale socket from a previous run
        }
        @mkdir(dirname($socketPath), 0755, true);

        $server = @stream_socket_server("unix://{$socketPath}", $errno, $errstr);
        if ($server === false) {
            throw new FormaException("listen on {$socketPath}: {$errstr}");
        }
        chmod($socketPath, 0666); // sidecar runs as a different user

        fwrite(STDERR, "[lib-forma-php] listening on {$socketPath}\n");

        while (($conn = @stream_socket_accept($server, -1)) !== false) {
            try {
                $this->serveOne($conn);
            } finally {
                fclose($conn);
            }
        }
    }

    /** @param resource $conn */
    private function serveOne($conn): void
    {
        $request = $this->readRequest($conn);
        if ($request === null) {
            return;
        }
        [$method, $path, $body] = $request;

        if ($method === 'GET' && $path === '/health') {
            $this->respond($conn, 200, ['status' => 'healthy', 'handlers' => count($this->handlers)]);
            return;
        }

        if ($method !== 'POST' || !str_starts_with($path, '/invoke/')) {
            $this->respond($conn, 404, ['error' => 'expected POST /invoke/{module}/{entity}/{action}']);
            return;
        }

        $parts = explode('/', trim(substr($path, strlen('/invoke/')), '/'));
        if (count($parts) !== 3) {
            $this->respond($conn, 404, ['error' => 'expected POST /invoke/{module}/{entity}/{action}']);
            return;
        }
        [$module, $entity, $action] = array_map('rawurldecode', $parts);
        $key = "{$module}.{$entity}.{$action}";

        $handler = $this->handlers[$key] ?? null;
        if ($handler === null) {
            $this->respond($conn, 500, ['error' => "no handler registered for {$key}"]);
            return;
        }

        $decoded = json_decode($body, true);
        $invocation = Invocation::fromRequest($module, $entity, $action, is_array($decoded) ? $decoded : []);

        try {
            $result = $handler($invocation, $this->ctx);
        } catch (\Throwable $e) {
            $this->respond($conn, 500, ['error' => $e->getMessage()]);
            return;
        }

        if ($result instanceof ActionResult) {
            $this->respond($conn, 200, $result->toWire());
        } else {
            $this->respond($conn, 200, ['data' => $result]);
        }
    }

    /**
     * Minimal HTTP/1.1 request parser: request line + headers + body.
     *
     * @param resource $conn
     * @return array{string, string, string}|null [method, path, body]
     */
    private function readRequest($conn): ?array
    {
        $requestLine = fgets($conn);
        if ($requestLine === false) {
            return null;
        }
        $lineParts = explode(' ', trim($requestLine));
        if (count($lineParts) < 2) {
            return null;
        }
        [$method, $target] = $lineParts;

        $contentLength = 0;
        while (($line = fgets($conn)) !== false) {
            $line = trim($line);
            if ($line === '') {
                break; // end of headers
            }
            if (stripos($line, 'content-length:') === 0) {
                $contentLength = (int) trim(substr($line, strlen('content-length:')));
            }
        }

        $body = '';
        while (strlen($body) < $contentLength && !feof($conn)) {
            $chunk = fread($conn, $contentLength - strlen($body));
            if ($chunk === false) {
                break;
            }
            $body .= $chunk;
        }

        $path = parse_url($target, PHP_URL_PATH) ?: $target;

        return [$method, $path, $body];
    }

    /**
     * @param resource $conn
     * @param array<string, mixed> $payload
     */
    private function respond($conn, int $status, array $payload): void
    {
        $reasons = [200 => 'OK', 404 => 'Not Found', 500 => 'Internal Server Error'];
        $reason = $reasons[$status] ?? 'Error';
        $body = json_encode($payload, JSON_THROW_ON_ERROR);

        fwrite(
            $conn,
            "HTTP/1.1 {$status} {$reason}\r\n"
            . "Content-Type: application/json\r\n"
            . 'Content-Length: ' . strlen($body) . "\r\n"
            . "Connection: close\r\n\r\n"
            . $body
        );
    }
}
