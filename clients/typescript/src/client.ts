import createClient, { type Client as FetchClient } from "openapi-fetch";
import type { paths } from "./generated/schema.js";
import { QuorumError } from "./errors.js";
import {
  credentialHeaders,
  DEFAULT_HEADER_NAMES,
  mergeHeaderNames,
  type Credentials,
  type HeaderNames,
} from "./identity.js";
import { isTerminalEvent, readEvents, type StreamEvent } from "./sse.js";
import type {
  AuditLog,
  CreateRequestInput,
  Health,
  JSONObject,
  Page,
  Policy,
  PolicyInput,
  PolicyUpdate,
  QuorumRequest,
  RequestStatus,
  Webhook,
  WebhookInput,
} from "./types.js";
import { isTerminal } from "./types.js";
import { VERSION } from "./version.js";

export interface ClientOptions {
  /** Tenant sent with trusted identities that do not name one themselves. */
  tenant?: string;
  /** Identity used until `as()` is called. */
  credentials?: Credentials;
  /** Header names a `trust` mode server reads identity from; unset fields keep the defaults. */
  trustHeaders?: Partial<HeaderNames>;
  /** Replaces the global fetch, e.g. for a proxy or instrumentation. */
  fetch?: typeof globalThis.fetch;
  /** Per-call timeout in milliseconds. Default 30 000. Event streams are not bounded. */
  timeoutMs?: number;
  /** Replaces the default `quorum-ts-client/<version>`. Browsers ignore it. */
  userAgent?: string;
}

/** Options every call accepts. */
export interface CallOptions {
  /** Aborts the call. */
  signal?: AbortSignal;
}

export interface CreateRequest {
  /** Selects the policy. */
  type: string;
  /** The change being proposed. Must be a non-empty object. */
  payload: JSONObject;
  /** Per-request allowlist of checker IDs; overrides role and permission rules. */
  eligibleReviewers?: string[];
  /** Free-form data for the consumer; `display` is rendered by the console and widgets. */
  metadata?: JSONObject;
  /** Retrying a submission with the same key returns the original request instead of opening another. */
  idempotencyKey?: string;
}

export interface ListRequestsFilter extends Paging {
  status?: RequestStatus;
  type?: string;
  makerId?: string;
  search?: string;
}

export interface Paging {
  page?: number;
  /** Capped at 100 by the server. */
  perPage?: number;
}

export interface WaitOptions extends CallOptions {
  /** How often to re-fetch the request when the event stream is unavailable. Default 5000. */
  pollIntervalMs?: number;
}

/**
 * Calls one Quorum server. Create one with `new QuorumClient(url, options)`
 * and derive per-identity clients with `as()`; a single instance can serve
 * every maker and checker in a process.
 */
export class QuorumClient {
  readonly baseUrl: string;
  private readonly tenant: string | undefined;
  private readonly credentials: Credentials | undefined;
  private readonly headerNames: HeaderNames;
  private readonly fetchImpl: typeof globalThis.fetch;
  private readonly timeoutMs: number;
  private readonly userAgent: string;
  private readonly api: FetchClient<paths>;

  constructor(baseUrl: string, options: ClientOptions = {}) {
    let url: URL;
    try {
      url = new URL(baseUrl);
    } catch {
      throw new Error(`quorum: invalid base URL: ${baseUrl}`);
    }
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      throw new Error(`quorum: base URL must be http(s), got ${baseUrl}`);
    }
    this.baseUrl = baseUrl.replace(/\/+$/, "");
    this.tenant = options.tenant;
    this.credentials = options.credentials;
    this.headerNames = mergeHeaderNames({ ...DEFAULT_HEADER_NAMES }, options.trustHeaders);
    this.fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis);
    this.timeoutMs = options.timeoutMs ?? 30_000;
    this.userAgent = options.userAgent ?? `quorum-ts-client/${VERSION}`;
    this.api = createClient<paths>({
      baseUrl: this.baseUrl,
      headers: this.headers(),
      fetch: (req) => this.fetchImpl(req),
    });
  }

  /** Returns a client that acts as `credentials`. The receiver is unchanged. */
  as(credentials: Credentials): QuorumClient {
    return new QuorumClient(this.baseUrl, {
      tenant: this.tenant,
      credentials,
      trustHeaders: this.headerNames,
      fetch: this.fetchImpl,
      timeoutMs: this.timeoutMs,
      userAgent: this.userAgent,
    } as ClientOptions);
  }

  private headers(): Record<string, string> {
    return {
      "User-Agent": this.userAgent,
      ...credentialHeaders(this.credentials, this.headerNames, this.tenant),
    };
  }

  private signal(opts: CallOptions | undefined, bounded = true): AbortSignal | null {
    const signals: AbortSignal[] = [];
    if (opts?.signal) signals.push(opts.signal);
    if (bounded && this.timeoutMs > 0) signals.push(AbortSignal.timeout(this.timeoutMs));
    if (signals.length === 0) return null;
    return signals.length === 1 ? signals[0]! : AbortSignal.any(signals);
  }

  // --- system ---------------------------------------------------------------

  /** Reports the server's health and version. Needs no identity. */
  async health(opts?: CallOptions): Promise<Health> {
    const { data, error, response } = await this.api.GET("/health", { signal: this.signal(opts) });
    if (data) return data;
    if (response.status === 503 && error) return error as Health;
    throw this.error(response, error, "GET", "/health");
  }

  // --- requests -------------------------------------------------------------

  /** Submits a request on behalf of the client's identity, who becomes its maker. */
  async createRequest(input: CreateRequest, opts?: CallOptions): Promise<QuorumRequest> {
    const body: CreateRequestInput = { type: input.type, payload: input.payload };
    if (input.eligibleReviewers) body.eligible_reviewers = input.eligibleReviewers;
    if (input.metadata) body.metadata = input.metadata;
    const { data, error, response } = await this.api.POST("/api/v1/requests", {
      body,
      ...(input.idempotencyKey ? { params: { header: { "Idempotency-Key": input.idempotencyKey } } } : {}),
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "POST", "/api/v1/requests");
  }

  async getRequest(id: string, opts?: CallOptions): Promise<QuorumRequest> {
    const { data, error, response } = await this.api.GET("/api/v1/requests/{id}", {
      params: { path: { id } },
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "GET", `/api/v1/requests/${id}`);
  }

  async listRequests(filter: ListRequestsFilter = {}, opts?: CallOptions): Promise<Page<QuorumRequest>> {
    const query: NonNullable<paths["/api/v1/requests"]["get"]["parameters"]["query"]> = {};
    if (filter.status) query.status = filter.status;
    if (filter.type) query.type = filter.type;
    if (filter.makerId) query.maker_id = filter.makerId;
    if (filter.search) query.search = filter.search;
    if (filter.page) query.page = filter.page;
    if (filter.perPage) query.per_page = filter.perPage;
    const { data, error, response } = await this.api.GET("/api/v1/requests", {
      params: { query },
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "GET", "/api/v1/requests");
  }

  /** Records an approval by the client's identity at the request's current stage. */
  approve(id: string, comment?: string, opts?: CallOptions): Promise<QuorumRequest> {
    return this.decide("/api/v1/requests/{id}/approve", id, comment, opts);
  }

  /** Records a rejection by the client's identity; the request becomes terminal. */
  reject(id: string, comment?: string, opts?: CallOptions): Promise<QuorumRequest> {
    return this.decide("/api/v1/requests/{id}/reject", id, comment, opts);
  }

  private async decide(
    path: "/api/v1/requests/{id}/approve" | "/api/v1/requests/{id}/reject",
    id: string,
    comment: string | undefined,
    opts: CallOptions | undefined,
  ): Promise<QuorumRequest> {
    const { data, error, response } = await this.api.POST(path, {
      params: { path: { id } },
      ...(comment ? { body: { comment } } : {}),
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "POST", path.replace("{id}", id));
  }

  /** Withdraws a pending request. Only its maker may do so. */
  async cancel(id: string, opts?: CallOptions): Promise<QuorumRequest> {
    const { data, error, response } = await this.api.POST("/api/v1/requests/{id}/cancel", {
      params: { path: { id } },
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "POST", `/api/v1/requests/${id}/cancel`);
  }

  /** The request's audit trail, oldest first. */
  async audit(id: string, opts?: CallOptions): Promise<AuditLog[]> {
    const { data, error, response } = await this.api.GET("/api/v1/requests/{id}/audit", {
      params: { path: { id } },
      signal: this.signal(opts),
    });
    if (data) return data.data;
    throw this.error(response, error, "GET", `/api/v1/requests/${id}/audit`);
  }

  /**
   * Subscribes to a request's server-sent events. The iterator ends when the
   * server closes the stream (the request was already terminal), when the
   * signal aborts, or on a connection error, which is thrown. A server with
   * streaming disabled throws a 404 `QuorumError` immediately.
   */
  async *events(id: string, opts?: CallOptions): AsyncGenerator<StreamEvent> {
    const path = `/api/v1/requests/${id}/events`;
    const init: RequestInit = {
      method: "GET",
      headers: { ...this.headers(), Accept: "text/event-stream" },
    };
    init.signal = this.signal(opts, false);
    const response = await this.fetchImpl(this.baseUrl + path, init);
    if (response.status !== 200 || !response.body) {
      let detail: unknown;
      try {
        detail = await response.json();
      } catch {
        detail = undefined;
      }
      throw this.error(response, detail, "GET", path);
    }
    try {
      yield* readEvents(response.body);
    } finally {
      await response.body.cancel().catch(() => {});
    }
  }

  /**
   * Resolves once the request reaches a terminal status. Listens on the event
   * stream and falls back to polling when streaming is unavailable or drops.
   * Bound the wait with `signal`, for example `AbortSignal.timeout(ms)`.
   */
  async waitForDecision(id: string, opts: WaitOptions = {}): Promise<QuorumRequest> {
    const pollEvery = opts.pollIntervalMs && opts.pollIntervalMs > 0 ? opts.pollIntervalMs : 5_000;
    const call: CallOptions = opts.signal ? { signal: opts.signal } : {};
    for (;;) {
      // A fresh read first: the request may already be decided, and this
      // catches anything that happened before the stream opened.
      const current = await this.getRequest(id, call);
      if (isTerminal(current.status)) return current;

      let streamFailed = false;
      try {
        await this.followStream(id, call);
      } catch (err) {
        if (opts.signal?.aborted) throw err;
        streamFailed = true;
      }
      if (!streamFailed) continue; // the stream closed after an update; re-read

      return this.poll(id, pollEvery, opts.signal);
    }
  }

  /** Consumes events until the request looks decided or the stream ends. */
  private async followStream(id: string, call: CallOptions): Promise<void> {
    for await (const ev of this.events(id, call)) {
      if (isTerminalEvent(ev)) return;
      const current = await this.getRequest(id, call);
      if (isTerminal(current.status)) return;
    }
  }

  private async poll(id: string, everyMs: number, signal: AbortSignal | undefined): Promise<QuorumRequest> {
    for (;;) {
      await sleep(everyMs, signal);
      try {
        const current = await this.getRequest(id, signal ? { signal } : {});
        if (isTerminal(current.status)) return current;
      } catch (err) {
        if (signal?.aborted) throw err;
        // transient; keep polling
      }
    }
  }

  // --- policies -------------------------------------------------------------

  async createPolicy(input: PolicyInput, opts?: CallOptions): Promise<Policy> {
    const { data, error, response } = await this.api.POST("/api/v1/policies", {
      body: input,
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "POST", "/api/v1/policies");
  }

  async getPolicy(id: string, opts?: CallOptions): Promise<Policy> {
    const { data, error, response } = await this.api.GET("/api/v1/policies/{id}", {
      params: { path: { id } },
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "GET", `/api/v1/policies/${id}`);
  }

  async listPolicies(paging: Paging = {}, opts?: CallOptions): Promise<Page<Policy>> {
    const { data, error, response } = await this.api.GET("/api/v1/policies", {
      params: { query: pagingQuery(paging) },
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "GET", "/api/v1/policies");
  }

  /** Replaces the fields given; an empty array clears a list, an absent field keeps it. */
  async updatePolicy(id: string, update: PolicyUpdate, opts?: CallOptions): Promise<Policy> {
    const { data, error, response } = await this.api.PUT("/api/v1/policies/{id}", {
      params: { path: { id } },
      body: update,
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "PUT", `/api/v1/policies/${id}`);
  }

  async deletePolicy(id: string, opts?: CallOptions): Promise<void> {
    const { error, response } = await this.api.DELETE("/api/v1/policies/{id}", {
      params: { path: { id } },
      signal: this.signal(opts),
    });
    if (!response.ok) throw this.error(response, error, "DELETE", `/api/v1/policies/${id}`);
  }

  // --- webhooks -------------------------------------------------------------

  async createWebhook(input: WebhookInput, opts?: CallOptions): Promise<Webhook> {
    const { data, error, response } = await this.api.POST("/api/v1/webhooks", {
      body: input,
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "POST", "/api/v1/webhooks");
  }

  async listWebhooks(paging: Paging = {}, opts?: CallOptions): Promise<Page<Webhook>> {
    const { data, error, response } = await this.api.GET("/api/v1/webhooks", {
      params: { query: pagingQuery(paging) },
      signal: this.signal(opts),
    });
    if (data) return data;
    throw this.error(response, error, "GET", "/api/v1/webhooks");
  }

  async deleteWebhook(id: string, opts?: CallOptions): Promise<void> {
    const { error, response } = await this.api.DELETE("/api/v1/webhooks/{id}", {
      params: { path: { id } },
      signal: this.signal(opts),
    });
    if (!response.ok) throw this.error(response, error, "DELETE", `/api/v1/webhooks/${id}`);
  }

  // --- internals ------------------------------------------------------------

  private error(response: Response, body: unknown, method: string, path: string): QuorumError {
    const message =
      body && typeof body === "object" && typeof (body as { error?: unknown }).error === "string"
        ? (body as { error: string }).error
        : response.statusText || "request failed";
    return new QuorumError(response.status, message, { method, path });
  }
}

function pagingQuery(p: Paging): { page?: number; per_page?: number } {
  const q: { page?: number; per_page?: number } = {};
  if (p.page) q.page = p.page;
  if (p.perPage) q.per_page = p.perPage;
  return q;
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(signal.reason instanceof Error ? signal.reason : new Error("aborted"));
      return;
    }
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    const onAbort = () => {
      clearTimeout(timer);
      reject(signal!.reason instanceof Error ? signal!.reason : new Error("aborted"));
    };
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}
