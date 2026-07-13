<?php

declare(strict_types=1);

namespace Forma;

/**
 * The ctx.* surface handed to handlers. Every method call is an HTTP call
 * back to forma-sidecar (§4.3) — the same primitive contract Starlark
 * scripts use:
 *
 *     $ctx->db()->query('SELECT ...');
 *     $ctx->db()->named('analytics-db')->query('SELECT ...');
 *     $ctx->lock()->acquire('workspace:X', 30);
 *     $ctx->cache()->get('key');
 */
final class Ctx
{
    public function __construct(private readonly SidecarClient $client)
    {
    }

    public function db(): CtxPrimitive
    {
        return new CtxPrimitive($this->client, 'db');
    }

    public function cache(): CtxPrimitive
    {
        return new CtxPrimitive($this->client, 'cache');
    }

    public function lock(): CtxPrimitive
    {
        return new CtxPrimitive($this->client, 'lock');
    }

    public function queue(): CtxPrimitive
    {
        return new CtxPrimitive($this->client, 'queue');
    }

    public function pubsub(): CtxPrimitive
    {
        return new CtxPrimitive($this->client, 'pubsub');
    }

    public function storage(): CtxPrimitive
    {
        return new CtxPrimitive($this->client, 'storage');
    }

    public function kvstore(): CtxPrimitive
    {
        return new CtxPrimitive($this->client, 'kvstore');
    }

    /** Entity primitive — access entity records via named('module/entity'). */
    public function entity(): CtxPrimitive
    {
        return new CtxPrimitive($this->client, 'entity');
    }
}
