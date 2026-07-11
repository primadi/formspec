<?php

declare(strict_types=1);

namespace Forma;

/**
 * HTTP client to the forma-sidecar local listener — unix domain socket
 * (default) or localhost TCP.
 */
final class SidecarClient
{
    private readonly string $baseUrl;
    private readonly ?string $socketPath;

    /**
     * @param string $endpoint "unix:///var/run/forma/sidecar.sock" or "http://localhost:PORT"
     */
    public function __construct(string $endpoint, private readonly int $timeoutSeconds = 30)
    {
        if (str_starts_with($endpoint, 'unix://')) {
            $this->socketPath = substr($endpoint, strlen('unix://'));
            $this->baseUrl = 'http://forma-sidecar'; // host ignored; curl dials the socket
        } elseif (str_starts_with($endpoint, 'http://')) {
            $this->socketPath = null;
            $this->baseUrl = rtrim($endpoint, '/');
        } else {
            throw new FormaException("sidecar endpoint {$endpoint}: unsupported scheme (want unix:// or http://)");
        }
    }

    /**
     * @param array<string, mixed> $body
     * @return array<string, mixed> decoded ctx response
     */
    public function post(string $path, array $body): array
    {
        $ch = curl_init($this->baseUrl . $path);
        if ($ch === false) {
            throw new FormaException('curl_init failed');
        }
        if ($this->socketPath !== null) {
            curl_setopt($ch, CURLOPT_UNIX_SOCKET_PATH, $this->socketPath);
        }
        curl_setopt_array($ch, [
            CURLOPT_POST => true,
            CURLOPT_POSTFIELDS => json_encode($body, JSON_THROW_ON_ERROR),
            CURLOPT_HTTPHEADER => ['Content-Type: application/json'],
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_TIMEOUT => $this->timeoutSeconds,
        ]);

        $raw = curl_exec($ch);
        if ($raw === false) {
            $err = curl_error($ch);
            curl_close($ch);
            throw new FormaException("sidecar call {$path}: {$err}");
        }
        $status = (int) curl_getinfo($ch, CURLINFO_RESPONSE_CODE);
        curl_close($ch);

        $decoded = json_decode((string) $raw, true);
        if (!is_array($decoded)) {
            $decoded = [];
        }
        if ($status !== 200) {
            $msg = (string) ($decoded['error'] ?? "HTTP {$status}");
            throw new FormaException("sidecar call {$path}: {$msg}");
        }

        return $decoded;
    }
}
