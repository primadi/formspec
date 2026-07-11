import { FormaApiError } from "./error.js";
import type { ErrorResponse, ListOptions, ListResponse, ListResult, SingleResponse } from "./types.js";

export interface FormaClientOptions {
  /** Server origin, e.g. "https://api.example.com" (no trailing slash needed). */
  baseUrl: string;
  /** Workspace slug or UUID — every route is prefixed with it (§16). */
  workspace: string;
  /**
   * Returns the current bearer token, or undefined for an anonymous
   * request. Called before every request, so a token refreshed elsewhere
   * (e.g. by an auth library) is always picked up.
   */
  getToken?: () => string | undefined | Promise<string | undefined>;
  /** Override the fetch implementation (tests, non-browser runtimes). */
  fetch?: typeof fetch;
}

/**
 * Generic, untyped runtime for the Forma REST API
 * (docs/spec/02-core-basic.md §16). `forma generate` emits a typed layer on
 * top of this — one interface and one set of typed methods per exposed
 * entity — so application code almost never calls this class directly.
 *
 * This class is also the manual escape hatch: any entity/action the
 * generator hasn't run for yet (or a hand-built page, per
 * docs guides on building pages manually) can call these methods directly.
 */
export class FormaClient {
  constructor(private readonly options: FormaClientOptions) {}

  async list<T = Record<string, unknown>>(
    module: string,
    plural: string,
    query: ListOptions = {},
  ): Promise<ListResult<T>> {
    const params = new URLSearchParams();
    if (query.page !== undefined) params.set("page", String(query.page));
    if (query.perPage !== undefined) params.set("per_page", String(query.perPage));
    if (query.search !== undefined) params.set("search", query.search);
    const qs = params.toString();

    const resp = await this.request<ListResponse<T>>(
      "GET",
      `/${module}/${plural}${qs ? `?${qs}` : ""}`,
    );
    return {
      data: resp.data,
      page: resp.meta.page,
      perPage: resp.meta.per_page,
      total: resp.meta.total,
      totalPages: resp.meta.total_pages,
    };
  }

  async find<T = Record<string, unknown>>(module: string, plural: string, id: string): Promise<T> {
    const resp = await this.request<SingleResponse<T>>("GET", `/${module}/${plural}/${encodeURIComponent(id)}`);
    return resp.data;
  }

  // input/patch/params are typed `object`, not `Record<string, unknown>`:
  // a generated interface without an index signature (the common case for
  // codegen'd Create/Update/Params types) is not assignable to
  // Record<string, unknown> under TS's structural rules, but `object`
  // accepts any non-primitive shape while still rejecting stray primitives.
  async create<T = Record<string, unknown>>(
    module: string,
    plural: string,
    input: object,
  ): Promise<T> {
    const resp = await this.request<SingleResponse<T>>("POST", `/${module}/${plural}`, input);
    return resp.data;
  }

  /** Sends only the changed fields — the server reads current version internally (no CAS token to send). */
  async update<T = Record<string, unknown>>(
    module: string,
    plural: string,
    id: string,
    patch: object,
  ): Promise<T> {
    const resp = await this.request<SingleResponse<T>>(
      "PATCH",
      `/${module}/${plural}/${encodeURIComponent(id)}`,
      patch,
    );
    return resp.data;
  }

  async delete(module: string, plural: string, id: string): Promise<void> {
    await this.request<void>("DELETE", `/${module}/${plural}/${encodeURIComponent(id)}`);
  }

  /** Invokes a custom (non-CRUD) action: POST .../{id}/{action}. */
  async action<T = unknown>(
    module: string,
    plural: string,
    id: string,
    actionName: string,
    params: object = {},
  ): Promise<T> {
    const resp = await this.request<SingleResponse<T>>(
      "POST",
      `/${module}/${plural}/${encodeURIComponent(id)}/${actionName}`,
      params,
    );
    return resp.data;
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const doFetch = this.options.fetch ?? fetch;
    const token = await this.options.getToken?.();

    const headers: Record<string, string> = {};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (token) headers.Authorization = `Bearer ${token}`;

    const url = `${this.options.baseUrl.replace(/\/$/, "")}/${this.options.workspace}/api/v1${path}`;
    const resp = await doFetch(url, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });

    if (resp.status === 204) return undefined as T;

    const raw = await resp.text();
    const parsed = raw ? (JSON.parse(raw) as unknown) : undefined;

    if (!resp.ok) {
      throw new FormaApiError(resp.status, parsed as ErrorResponse | undefined, resp.statusText);
    }
    return parsed as T;
  }
}
