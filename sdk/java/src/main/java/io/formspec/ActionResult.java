package io.formspec;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Structured handler result — the wire form of the engine's ExecuteResult.
 * Handlers may also return a plain Map/List, which becomes {@code data}.
 */
public final class ActionResult {
    private final Object data;
    private final String newState;
    private final List<Map<String, Object>> events;

    public ActionResult(Object data) {
        this(data, null);
    }

    public ActionResult(Object data, String newState) {
        this.data = data;
        this.newState = newState;
        this.events = new ArrayList<>();
    }

    /** Set the new state after this action. */
    public ActionResult withNewState(String newState) {
        return new ActionResult(this.data, newState);
    }

    /** Add an event emission to the result. */
    public ActionResult withEvent(String name) {
        return withEvent(name, Map.of(), false);
    }

    /** Add an event emission with a payload. */
    public ActionResult withEvent(String name, Map<String, Object> payload) {
        return withEvent(name, payload, false);
    }

    /** Add an event emission with a payload and durability flag. */
    public ActionResult withEvent(String name, Map<String, Object> payload, boolean durable) {
        var event = new LinkedHashMap<String, Object>();
        event.put("name", name);
        if (payload != null && !payload.isEmpty()) {
            event.put("payload", payload);
        }
        if (durable) {
            event.put("durable", true);
        }
        var result = new ActionResult(this.data, this.newState);
        result.events.addAll(this.events);
        result.events.add(event);
        return result;
    }

    /** Serialize to the wire format expected by formspec-sidecar. */
    public Map<String, Object> toWire() {
        var wire = new LinkedHashMap<String, Object>();
        wire.put("data", data);
        if (newState != null) {
            wire.put("new_state", newState);
        }
        if (!events.isEmpty()) {
            wire.put("events", events);
        }
        return wire;
    }
}
