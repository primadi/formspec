package io.forma;

import java.util.Collections;
import java.util.Map;

/**
 * One action invocation from the sidecar — the wire form of the engine's
 * ExecuteParams (docs/runtimes/04-forma-sidecar.md §4.2).
 *
 * @param module     owning module name
 * @param entity     entity name
 * @param action     action name
 * @param resourceId identifier of the resource being acted upon
 * @param resource   current entity record
 * @param params     action parameters from the request body
 * @param tenantId   tenant context
 * @param userId     user context
 */
public record Invocation(
        String module,
        String entity,
        String action,
        String resourceId,
        Map<String, Object> resource,
        Map<String, Object> params,
        String tenantId,
        String userId) {

    @SuppressWarnings("unchecked")
    static Invocation fromRequest(String module, String entity, String action, Map<String, Object> body) {
        return new Invocation(
                module,
                entity,
                action,
                (String) body.getOrDefault("resource_id", ""),
                (Map<String, Object>) body.getOrDefault("resource", Collections.emptyMap()),
                (Map<String, Object>) body.getOrDefault("params", Collections.emptyMap()),
                (String) body.getOrDefault("tenant_id", ""),
                (String) body.getOrDefault("user_id", ""));
    }
}
