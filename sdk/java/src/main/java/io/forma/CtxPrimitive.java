package io.forma;

import java.util.*;

/**
 * One ctx primitive handle (db/cache/lock/…), optionally bound to a named
 * datastore via {@link #named(String)}. Operations map 1:1 to
 * {@code POST /ctx/{prim}/{op}}.
 */
public final class CtxPrimitive {
    private final SidecarClient client;
    private final String type;
    private final String boundName;

    CtxPrimitive(SidecarClient client, String type) {
        this(client, type, "");
    }

    CtxPrimitive(SidecarClient client, String type, String boundName) {
        this.client = client;
        this.type = type;
        this.boundName = boundName;
    }

    /** Bind to a named datastore instead of the default one. */
    public CtxPrimitive named(String name) {
        return new CtxPrimitive(client, type, name);
    }

    // ---- db ----

    /**
     * Execute a query on the datastore.
     *
     * @param sql  the SQL query string
     * @param args positional arguments (optional)
     * @return list of result rows as maps
     */
    @SuppressWarnings("unchecked")
    public List<Map<String, Object>> query(String sql, Object... args) {
        var body = new LinkedHashMap<String, Object>();
        body.put("sql", sql);
        if (args.length > 0) {
            body.put("args", List.of(args));
        }
        var resp = call("query", body);
        var data = resp.get("data");
        return data instanceof List ? (List<Map<String, Object>>) data : List.of();
    }

    // ---- cache / kvstore ----

    /**
     * Get a value by key.
     *
     * @return the value, or {@code null} if not found
     */
    public Object get(String key) {
        return call("get", Map.of("key", key)).get("data");
    }

    /**
     * Set a value by key.
     *
     * @param ttlSeconds time-to-live in seconds (0 = no expiry)
     */
    public void set(String key, Object value, int ttlSeconds) {
        var body = new LinkedHashMap<String, Object>();
        body.put("key", key);
        body.put("value", value);
        if (ttlSeconds > 0) {
            body.put("ttl_seconds", ttlSeconds);
        }
        call("set", body);
    }

    /** Delete a key. */
    public void delete(String key) {
        call("delete", Map.of("key", key));
    }

    // ---- lock ----

    /**
     * Acquire a distributed lock.
     *
     * @param key        the lock key
     * @param ttlSeconds lock TTL in seconds (default 30)
     * @return {@code true} if the lock was acquired
     */
    public boolean acquire(String key, int ttlSeconds) {
        var resp = call("acquire", Map.of("key", key, "ttl_seconds", ttlSeconds));
        return Boolean.TRUE.equals(resp.get("ok"));
    }

    /** Release a distributed lock. */
    public void release(String key) {
        call("release", Map.of("key", key));
    }

    // ---- internal ----

    private Map<String, Object> call(String op, Map<String, Object> body) {
        if (!boundName.isEmpty()) {
            body = new LinkedHashMap<>(body);
            body.put("named", boundName);
        }
        return client.post("/ctx/" + type + "/" + op, body);
    }
}
