/**
 * App -> Sidecar direction: the ctx.* primitive client.
 *
 * Every method is an HTTP call to forma-sidecar's /ctx/{primitive}/{operation}
 * endpoint — the same primitive contract Starlark scripts use. Node stdlib only.
 */

import * as http from "node:http";

/** Transport failure or sidecar-reported error. */
export class FormaError extends Error {}

interface CtxResponse {
  data?: unknown;
  ok?: boolean;
  error?: string;
}

/** HTTP client to the sidecar listener (unix:// socket or http:// TCP). */
export class SidecarClient {
  private readonly socketPath?: string;
  private readonly host?: string;
  private readonly port?: number;

  constructor(
    endpoint?: string,
    private readonly timeoutMs: number = 30_000,
  ) {
    endpoint ??= `unix://${process.env.FORMA_SIDECAR_SOCKET ?? "/tmp/forma/sidecar.sock"}`;
    if (endpoint.startsWith("unix://")) {
      this.socketPath = endpoint.slice("unix://".length);
    } else if (endpoint.startsWith("http://")) {
      const url = new URL(endpoint);
      this.host = url.hostname;
      this.port = url.port ? Number(url.port) : 80;
    } else {
      throw new FormaError(
        `sidecar endpoint ${endpoint}: unsupported scheme (want unix:// or http://)`,
      );
    }
  }

  post(path: string, body: Record<string, unknown>): Promise<CtxResponse> {
    const payload = JSON.stringify(body);
    const options: http.RequestOptions = {
      method: "POST",
      path,
      headers: {
        "Content-Type": "application/json",
        "Content-Length": Buffer.byteLength(payload),
      },
      timeout: this.timeoutMs,
      ...(this.socketPath
        ? { socketPath: this.socketPath }
        : { host: this.host, port: this.port }),
    };

    return new Promise((resolve, reject) => {
      const req = http.request(options, (res) => {
        const chunks: Buffer[] = [];
        res.on("data", (c: Buffer) => chunks.push(c));
        res.on("end", () => {
          let decoded: CtxResponse = {};
          try {
            decoded = JSON.parse(Buffer.concat(chunks).toString() || "{}");
          } catch {
            /* non-JSON error body; fall through to the status check */
          }
          if (res.statusCode !== 200) {
            reject(
              new FormaError(
                `sidecar call ${path}: ${decoded.error ?? `HTTP ${res.statusCode}`}`,
              ),
            );
            return;
          }
          resolve(decoded);
        });
      });
      req.on("timeout", () => req.destroy(new Error("timeout")));
      req.on("error", (err) =>
        reject(new FormaError(`sidecar call ${path}: ${err.message}`)),
      );
      req.end(payload);
    });
  }
}

/** One primitive handle (db/cache/lock/...); .named() binds a datastore. */
export class CtxPrimitive {
  constructor(
    private readonly client: SidecarClient,
    private readonly type: string,
    private readonly boundName: string = "",
  ) {}

  /** Bind to a named datastore instead of the default one. */
  named(name: string): CtxPrimitive {
    return new CtxPrimitive(this.client, this.type, name);
  }

  async query(sql: string, args?: unknown[]): Promise<Record<string, unknown>[]> {
    const body: Record<string, unknown> = { sql };
    if (args?.length) body.args = args;
    const resp = await this.call("query", body);
    return (resp.data as Record<string, unknown>[]) ?? [];
  }

  async get(key: string): Promise<unknown> {
    return (await this.call("get", { key })).data ?? null;
  }

  async set(key: string, value: unknown, ttlSeconds = 0): Promise<void> {
    const body: Record<string, unknown> = { key, value };
    if (ttlSeconds > 0) body.ttl_seconds = ttlSeconds;
    await this.call("set", body);
  }

  async delete(key: string): Promise<void> {
    await this.call("delete", { key });
  }

  /** Returns true if the lock was acquired. */
  async acquire(key: string, ttlSeconds = 30): Promise<boolean> {
    return Boolean((await this.call("acquire", { key, ttl_seconds: ttlSeconds })).ok);
  }

  async release(key: string): Promise<void> {
    await this.call("release", { key });
  }

  // ── Entity atomic operations ──

  /** Atomically merge fields into an entity record (entity/update).
   *  Uses jsonb_merge / json_patch — single SQL statement, no race condition.
   *  Workspace isolation is enforced by the sidecar — not a parameter. */
  async update(id: string, fields: Record<string, unknown>): Promise<void> {
    const body: Record<string, unknown> = { key: id, fields };
    await this.call("update", body);
  }

  /** Atomically increment a numeric field on an entity record.
   *  Single SQL statement — no read-modify-write race condition.
   *  Workspace isolation is enforced by the sidecar — not a parameter. */
  async increment(id: string, field: string, amount: number): Promise<void> {
    const body: Record<string, unknown> = { key: id, field, amount };
    await this.call("increment", body);
  }

  /** Atomically decrement a numeric field on an entity record.
   *  Includes a guard against negative values.
   *  Returns the new field value after decrement.
   *  Workspace isolation is enforced by the sidecar — not a parameter. */
  async decrement(id: string, field: string, amount: number): Promise<number> {
    const body: Record<string, unknown> = { key: id, field, amount };
    const resp = await this.call("decrement", body);
    return (resp.data as number) ?? 0;
  }

  private call(op: string, body: Record<string, unknown>): Promise<CtxResponse> {
    if (this.boundName) body.named = this.boundName;
    return this.client.post(`/ctx/${this.type}/${op}`, body);
  }
}

/**
 * The ctx.* surface handed to handlers.
 *
 *     const rows = await ctx.db().query("SELECT ...");
 *     await ctx.db().named("analytics-db").query("SELECT ...");
 *     await ctx.lock().acquire("workspace:X", 30);
 */
export class Ctx {
  constructor(private readonly client: SidecarClient) {}

  db(): CtxPrimitive {
    return new CtxPrimitive(this.client, "db");
  }
  cache(): CtxPrimitive {
    return new CtxPrimitive(this.client, "cache");
  }
  lock(): CtxPrimitive {
    return new CtxPrimitive(this.client, "lock");
  }
  queue(): CtxPrimitive {
    return new CtxPrimitive(this.client, "queue");
  }
  pubsub(): CtxPrimitive {
    return new CtxPrimitive(this.client, "pubsub");
  }
  storage(): CtxPrimitive {
    return new CtxPrimitive(this.client, "storage");
  }
  kvstore(): CtxPrimitive {
    return new CtxPrimitive(this.client, "kvstore");
  }

  /** Entity primitive — access entity records via named("module/entity").
   *
   * Usage:
   *   const med = await ctx.entity().named("pharmacy/medicine").get(id);
   *   await ctx.entity().named("pharmacy/medicine").set(id, newData);
   *   await ctx.entity().named("pharmacy/medicine").update(id, { stock: 100 });
   *   await ctx.entity().named("pharmacy/medicine").decrement(id, "stock", 4);
   */
  entity(): CtxPrimitive {
    return new CtxPrimitive(this.client, "entity");
  }
}
