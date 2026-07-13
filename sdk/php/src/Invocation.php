<?php

declare(strict_types=1);

namespace Forma;

/**
 * One action invocation from the sidecar — the wire form of the engine's
 * ExecuteParams (docs/runtimes/04-forma-sidecar.md §4.2).
 */
final class Invocation
{
    public function __construct(
        public readonly string $module,
        public readonly string $entity,
        public readonly string $action,
        public readonly string $resourceId,
        /** @var array<string, mixed> current entity record */
        public readonly array $resource,
        /** @var array<string, mixed> action parameters from the request body */
        public readonly array $params,
        public readonly string $userId,
    ) {
    }

    /** @param array<string, mixed> $body decoded /invoke request body */
    public static function fromRequest(string $module, string $entity, string $action, array $body): self
    {
        return new self(
            module: $module,
            entity: $entity,
            action: $action,
            resourceId: (string) ($body['resource_id'] ?? ''),
            resource: (array) ($body['resource'] ?? []),
            params: (array) ($body['params'] ?? []),
            userId: (string) ($body['user_id'] ?? ''),
        );
    }
}
