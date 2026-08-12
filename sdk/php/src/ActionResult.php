<?php

declare(strict_types=1);

namespace FormSpec;

/**
 * Structured handler result — the wire form of the engine's ExecuteResult.
 * Handlers may also return a plain array, which becomes `data`.
 */
final class ActionResult
{
    /** @var list<array{name: string, durable?: bool, payload?: array<string, mixed>}> */
    private array $events = [];

    public function __construct(
        public readonly mixed $data = null,
        public readonly ?string $newState = null,
    ) {
    }

    /** @param array<string, mixed> $payload */
    public function withEvent(string $name, array $payload = [], bool $durable = false): self
    {
        $event = ['name' => $name];
        if ($payload !== []) {
            $event['payload'] = $payload;
        }
        if ($durable) {
            $event['durable'] = true;
        }
        $this->events[] = $event;

        return $this;
    }

    /** @return array<string, mixed> */
    public function toWire(): array
    {
        $wire = ['data' => $this->data];
        if ($this->newState !== null) {
            $wire['new_state'] = $this->newState;
        }
        if ($this->events !== []) {
            $wire['events'] = $this->events;
        }

        return $wire;
    }
}
