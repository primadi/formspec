/**
 * Sidecar -> App direction: the /invoke listener.
 *
 * Receives POST /invoke/{module}/{entity}/{action} from forma-sidecar and
 * dispatches to registered handler functions. Also answers GET /health for
 * the sidecar's app monitor. Node stdlib only.
 */

import * as fs from "node:fs";
import * as http from "node:http";
import * as path from "node:path";

import { Ctx, FormaError, SidecarClient } from "./ctx";

/** One action invocation — the wire form of the engine's ExecuteParams. */
export interface Invocation {
  module: string;
  entity: string;
  action: string;
  resourceId: string;
  resource: Record<string, unknown>;
  params: Record<string, unknown>;
  tenantId: string;
  userId: string;
}

export interface EventEmission {
  name: string;
  durable?: boolean;
  payload?: Record<string, unknown>;
}

/**
 * Structured handler result — the wire form of ExecuteResult. Handlers may
 * also return plain data, which becomes `data`.
 */
export class ActionResult {
  private events: EventEmission[] = [];

  constructor(
    public readonly data: unknown = null,
    public readonly newState?: string,
  ) {}

  withEvent(name: string, payload?: Record<string, unknown>, durable = false): this {
    const event: EventEmission = { name };
    if (payload && Object.keys(payload).length > 0) event.payload = payload;
    if (durable) event.durable = true;
    this.events.push(event);
    return this;
  }

  toWire(): Record<string, unknown> {
    const wire: Record<string, unknown> = { data: this.data };
    if (this.newState !== undefined) wire.new_state = this.newState;
    if (this.events.length > 0) wire.events = this.events;
    return wire;
  }
}

export type Handler = (
  invocation: Invocation,
  ctx: Ctx,
) => Promise<ActionResult | unknown> | ActionResult | unknown;

export interface AppOptions {
  /** "unix:///path.sock"; default from FORMA_APP_SOCKET. */
  listen?: string;
  /** "unix:///path.sock" or "http://localhost:PORT"; default from FORMA_SIDECAR_SOCKET. */
  sidecarEndpoint?: string;
}

/**
 * The lib-forma (TypeScript) listener.
 *
 *     const app = new App();
 *     app.handle("billing.invoice.approve", async (inv, ctx) => {
 *       await ctx.lock().acquire(`invoice:${inv.resourceId}`);
 *       ...
 *       return new ActionResult({ ok: true }, "approved");
 *     });
 *     await app.run();
 */
export class App {
  private readonly handlers = new Map<string, Handler>();
  private readonly listen: string;
  readonly ctx: Ctx;

  constructor(options: AppOptions = {}) {
    this.listen =
      options.listen ??
      `unix://${process.env.FORMA_APP_SOCKET ?? "/var/run/forma/app.sock"}`;
    this.ctx = new Ctx(new SidecarClient(options.sidecarEndpoint));
  }

  /** Register a handler for "module.entity.action". */
  handle(action: string, handler: Handler): void {
    if (this.handlers.has(action)) {
      throw new FormaError(`handler for ${action} already registered`);
    }
    this.handlers.set(action, handler);
  }

  /** Starts the listener; resolves once it is accepting connections. */
  run(): Promise<http.Server> {
    if (!this.listen.startsWith("unix://")) {
      throw new FormaError(
        `listen ${this.listen}: only unix:// is supported by lib-forma`,
      );
    }
    const socketPath = this.listen.slice("unix://".length);

    fs.mkdirSync(path.dirname(socketPath), { recursive: true });
    if (fs.existsSync(socketPath)) {
      fs.unlinkSync(socketPath); // stale socket from a previous run
    }

    const server = http.createServer((req, res) => this.serve(req, res));

    return new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(socketPath, () => {
        fs.chmodSync(socketPath, 0o666); // sidecar runs as a different user
        process.stderr.write(`[lib-forma] listening on ${socketPath}\n`);
        resolve(server);
      });
    });
  }

  private serve(req: http.IncomingMessage, res: http.ServerResponse): void {
    const url = new URL(req.url ?? "/", "http://forma-app");

    if (req.method === "GET" && url.pathname === "/health") {
      respond(res, 200, { status: "healthy", handlers: this.handlers.size });
      return;
    }

    const parts = url.pathname
      .replace(/^\/+|\/+$/g, "")
      .split("/")
      .map(decodeURIComponent);
    if (req.method !== "POST" || parts.length !== 4 || parts[0] !== "invoke") {
      respond(res, 404, { error: "expected POST /invoke/{module}/{entity}/{action}" });
      return;
    }
    const [, module, entity, action] = parts;
    const key = `${module}.${entity}.${action}`;

    const handler = this.handlers.get(key);
    if (!handler) {
      respond(res, 500, { error: `no handler registered for ${key}` });
      return;
    }

    const chunks: Buffer[] = [];
    req.on("data", (c: Buffer) => chunks.push(c));
    req.on("end", async () => {
      let body: Record<string, unknown> = {};
      const raw = Buffer.concat(chunks).toString();
      if (raw) {
        try {
          body = JSON.parse(raw);
        } catch (err) {
          respond(res, 400, { error: `invalid JSON body: ${(err as Error).message}` });
          return;
        }
      }

      const invocation: Invocation = {
        module,
        entity,
        action,
        resourceId: String(body.resource_id ?? ""),
        resource: (body.resource as Record<string, unknown>) ?? {},
        params: (body.params as Record<string, unknown>) ?? {},
        tenantId: String(body.tenant_id ?? ""),
        userId: String(body.user_id ?? ""),
      };

      try {
        const result = await handler(invocation, this.ctx);
        if (result instanceof ActionResult) {
          respond(res, 200, result.toWire());
        } else {
          respond(res, 200, { data: result ?? null });
        }
      } catch (err) {
        respond(res, 500, { error: (err as Error).message ?? String(err) });
      }
    });
  }
}

function respond(
  res: http.ServerResponse,
  status: number,
  payload: Record<string, unknown>,
): void {
  const raw = JSON.stringify(payload);
  res.writeHead(status, {
    "Content-Type": "application/json",
    "Content-Length": Buffer.byteLength(raw),
  });
  res.end(raw);
}
