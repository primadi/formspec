package io.forma;

/**
 * The ctx.* surface handed to handlers. Every method call is an HTTP call
 * back to forma-sidecar (§4.3) — the same primitive contract Starlark
 * scripts use:
 *
 * <pre>{@code
 * ctx.db().query("SELECT ...");
 * ctx.db().named("analytics-db").query("SELECT ...");
 * ctx.lock().acquire("workspace:X", 30);
 * ctx.cache().get("key");
 * }</pre>
 */
public final class Ctx {
    private final SidecarClient client;

    Ctx(SidecarClient client) {
        this.client = client;
    }

    public CtxPrimitive db() {
        return new CtxPrimitive(client, "db");
    }

    public CtxPrimitive cache() {
        return new CtxPrimitive(client, "cache");
    }

    public CtxPrimitive lock() {
        return new CtxPrimitive(client, "lock");
    }

    public CtxPrimitive queue() {
        return new CtxPrimitive(client, "queue");
    }

    public CtxPrimitive pubsub() {
        return new CtxPrimitive(client, "pubsub");
    }

    public CtxPrimitive storage() {
        return new CtxPrimitive(client, "storage");
    }

    public CtxPrimitive kvstore() {
        return new CtxPrimitive(client, "kvstore");
    }

    /** Entity primitive — access entity records via named("module/entity"). */
    public CtxPrimitive entity() {
        return new CtxPrimitive(client, "entity");
    }
}
