import type { Request, Policy, AuditLog, PaginatedResponse, ListResponse } from './types';

export interface ClientConfig {
  apiUrl: string;
  token?: string;
  tenantId?: string;
  authHeaders?: Record<string, string>;
}

export interface SSEConnection {
  close(): void;
}

export interface QuorumClient {
  getRequest(id: string): Promise<Request>;
  listRequests(params?: { status?: string; type?: string; page?: number; per_page?: number }): Promise<PaginatedResponse<Request>>;
  getPolicy(id: string): Promise<Policy>;
  getPolicyByType(requestType: string): Promise<Policy | null>;
  listPolicies(): Promise<Policy[]>;
  approve(requestId: string, comment?: string): Promise<Request>;
  reject(requestId: string, comment?: string): Promise<Request>;
  getAudit(requestId: string): Promise<AuditLog[]>;
  connectSSE(requestId: string, onEvent: () => void, onDisconnect: () => void): SSEConnection;
}

export function createClient(config: ClientConfig): QuorumClient {
  const { apiUrl, token, tenantId, authHeaders } = config;
  const base = apiUrl.replace(/\/+$/, '') + '/api/v1';

  function headers(): Record<string, string> {
    const h: Record<string, string> = { 'Content-Type': 'application/json' };
    if (authHeaders) {
      Object.assign(h, authHeaders);
    } else if (token) {
      h['Authorization'] = `Bearer ${token}`;
    }
    if (tenantId) {
      h['X-Tenant-ID'] = tenantId;
    }
    return h;
  }

  async function request<T>(path: string, options?: RequestInit): Promise<T> {
    const res = await fetch(`${base}${path}`, {
      ...options,
      headers: { ...headers(), ...options?.headers },
    });

    if (!res.ok) {
      const body = await res.text();
      let message: string;
      try {
        message = JSON.parse(body).error || body;
      } catch {
        message = body;
      }
      throw new ApiError(message, res.status);
    }

    if (res.status === 204) return undefined as T;
    return res.json();
  }

  return {
    async getRequest(id) {
      return request<Request>(`/requests/${id}`);
    },

    async listRequests(params) {
      const qs = new URLSearchParams();
      if (params?.status) qs.set('status', params.status);
      if (params?.type) qs.set('type', params.type);
      if (params?.page) qs.set('page', String(params.page));
      if (params?.per_page) qs.set('per_page', String(params.per_page));
      const query = qs.toString();
      return request<PaginatedResponse<Request>>(`/requests${query ? '?' + query : ''}`);
    },

    async getPolicy(id) {
      return request<Policy>(`/policies/${id}`);
    },

    async getPolicyByType(requestType) {
      const res = await request<ListResponse<Policy>>('/policies');
      return res.data.find((p) => p.request_type === requestType) ?? null;
    },

    async listPolicies() {
      const res = await request<ListResponse<Policy>>('/policies');
      return res.data;
    },

    async approve(requestId, comment) {
      return request<Request>(`/requests/${requestId}/approve`, {
        method: 'POST',
        body: JSON.stringify(comment ? { comment } : {}),
      });
    },

    async reject(requestId, comment) {
      return request<Request>(`/requests/${requestId}/reject`, {
        method: 'POST',
        body: JSON.stringify(comment ? { comment } : {}),
      });
    },

    async getAudit(requestId) {
      const res = await request<ListResponse<AuditLog>>(`/requests/${requestId}/audit`);
      return res.data;
    },

    connectSSE(requestId, onEvent, onDisconnect) {
      const controller = new AbortController();
      const url = `${base}/requests/${requestId}/events`;

      // Use fetch (not EventSource) because EventSource cannot set custom headers.
      (async () => {
        try {
          const h = headers();
          // SSE doesn't need Content-Type (it's a GET with no body)
          delete h['Content-Type'];

          const res = await fetch(url, {
            headers: h,
            signal: controller.signal,
          });

          if (!res.ok || !res.body) {
            onDisconnect();
            return;
          }

          const reader = res.body.getReader();
          const decoder = new TextDecoder();
          let buffer = '';

          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });

            // Parse SSE frames (delimited by double newline)
            const parts = buffer.split('\n\n');
            buffer = parts.pop() ?? '';

            for (const part of parts) {
              // Skip comments (keepalives start with ':')
              const trimmed = part.trim();
              if (!trimmed || trimmed.startsWith(':')) continue;
              if (trimmed.includes('event:')) {
                onEvent();
              }
            }
          }

          // Stream ended (server closed connection)
          onDisconnect();
        } catch (e) {
          if (e instanceof DOMException && e.name === 'AbortError') return;
          onDisconnect();
        }
      })();

      return { close: () => controller.abort() };
    },
  };
}

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}
