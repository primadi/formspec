<?php

declare(strict_types=1);

namespace Forma;

/**
 * One ctx primitive handle (db/cache/lock/...), optionally bound to a named
 * datastore via named(). Operations map 1:1 to POST /ctx/{prim}/{op}.
 */
final class CtxPrimitive
{
    public function __construct(
        private readonly SidecarClient $client,
        private readonly string $type,
        private readonly string $named = '',
    ) {
    }

    /** Bind to a named datastore instead of the default one. */
    public function named(string $name): self
    {
        return new self($this->client, $this->type, $name);
    }

    /**
     * @param list<mixed> $args
     * @return list<array<string, mixed>> result rows
     */
    public function query(string $sql, array $args = []): array
    {
        $body = ['sql' => $sql];
        if ($args !== []) {
            $body['args'] = $args;
        }

        return (array) ($this->call('query', $body)['data'] ?? []);
    }

    public function get(string $key): mixed
    {
        return $this->call('get', ['key' => $key])['data'] ?? null;
    }

    public function set(string $key, mixed $value, int $ttlSeconds = 0): void
    {
        $body = ['key' => $key, 'value' => $value];
        if ($ttlSeconds > 0) {
            $body['ttl_seconds'] = $ttlSeconds;
        }
        $this->call('set', $body);
    }

    public function delete(string $key): void
    {
        $this->call('delete', ['key' => $key]);
    }

    // ---- entity atomic operations ----

    /**
     * Atomically merge fields into an entity record (entity/update).
     * Uses jsonb_merge / json_patch — single SQL statement, no race condition.
     */
    public function update(string $id, array $fields): void
    {
        $body = ['key' => $id, 'fields' => $fields];
        $this->call('update', $body);
    }

    /**
     * Atomically increment a numeric field on an entity record.
     * Single SQL statement — no read-modify-write race condition.
     */
    public function increment(string $id, string $field, float $amount): void
    {
        $body = ['key' => $id, 'field' => $field, 'amount' => $amount];
        $this->call('increment', $body);
    }

    /**
     * Atomically decrement a numeric field on an entity record.
     * Includes a guard against negative values. Returns the new field value.
     */
    public function decrement(string $id, string $field, float $amount): mixed
    {
        $body = ['key' => $id, 'field' => $field, 'amount' => $amount];
        return $this->call('decrement', $body)['data'] ?? null;
    }

    /** @return bool true if the lock was acquired */
    public function acquire(string $key, int $ttlSeconds = 30): bool
    {
        return (bool) ($this->call('acquire', ['key' => $key, 'ttl_seconds' => $ttlSeconds])['ok'] ?? false);
    }

    public function release(string $key): void
    {
        $this->call('release', ['key' => $key]);
    }

    /**
     * @param array<string, mixed> $body
     * @return array<string, mixed>
     */
    private function call(string $op, array $body): array
    {
        if ($this->named !== '') {
            $body['named'] = $this->named;
        }

        return $this->client->post("/ctx/{$this->type}/{$op}", $body);
    }
}
