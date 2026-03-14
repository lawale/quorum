import type { Request, Policy, AuditLog, PaginatedResponse, ListResponse } from './types';

export interface ClientConfig {
  apiUrl: string;
  token?: string;
  authHeaders?: Record<string, string>;
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
}

export function createClient(config: ClientConfig): QuorumClient {
  const { apiUrl, token, authHeaders } = config;
  const base = apiUrl.replace(/\/+$/, '') + '/api/v1';

  function headers(): Record<string, string> {
    const h: Record<string, string> = { 'Content-Type': 'application/json' };
    if (authHeaders) {
      Object.assign(h, authHeaders);
    } else if (token) {
      h['Authorization'] = `Bearer ${token}`;
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
