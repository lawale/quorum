import { clearSession } from './auth';
import { get } from 'svelte/store';
import { selectedTenant } from './stores';
import type {
  AuthResponse,
  Operator,
  Tenant,
  Policy,
  Webhook,
  Request,
  AuditLog,
  OutboxEntry,
  DeliveryStats,
  ListResponse,
  PaginatedListResponse,
  SetupStatusResponse,
} from './types';

const BASE = '/api/v1/console';

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) ?? {}),
  };

  const res = await fetch(`${BASE}${path}`, { ...options, headers, credentials: 'include' });

  if (res.status === 401) {
    clearSession();
    window.location.hash = '#/login';
    throw new ApiError(401, 'Unauthorized');
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const body = await res.json();

  if (!res.ok) {
    throw new ApiError(res.status, body.error || 'Unknown error');
  }

  return body as T;
}

// Helper: append tenant_id query param if a tenant is selected
function withTenant(path: string): string {
  const tenant = get(selectedTenant);
  if (!tenant) return path;
  const separator = path.includes('?') ? '&' : '?';
  return `${path}${separator}tenant_id=${encodeURIComponent(tenant)}`;
}

// --- Tenants ---

export const tenants = {
  list: (params?: { page?: number; per_page?: number }) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.per_page) query.set('per_page', String(params.per_page));
    const qs = query.toString();
    return request<PaginatedListResponse<Tenant>>(qs ? `/tenants?${qs}` : '/tenants');
  },
  create: (slug: string, name: string) =>
    request<Tenant>('/tenants', {
      method: 'POST',
      body: JSON.stringify({ slug, name }),
    }),
  delete: (id: string) => request<void>(`/tenants/${id}`, { method: 'DELETE' }),
};

// --- Auth ---

export const auth = {
  status: () => request<SetupStatusResponse>('/auth/status'),
  setup: (username: string, password: string, displayName: string) =>
    request<AuthResponse>('/auth/setup', {
      method: 'POST',
      body: JSON.stringify({ username, password, display_name: displayName }),
    }),
  login: (username: string, password: string) =>
    request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
};

// --- Operator ---

export const operators = {
  me: () => request<Operator>('/me'),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<{ message: string }>('/me/password', {
      method: 'PUT',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    }),
  list: (params?: { page?: number; per_page?: number }) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.per_page) query.set('per_page', String(params.per_page));
    const qs = query.toString();
    return request<PaginatedListResponse<Operator>>(qs ? `/operators?${qs}` : '/operators');
  },
  create: (username: string, password: string, displayName: string) =>
    request<Operator>('/operators', {
      method: 'POST',
      body: JSON.stringify({ username, password, display_name: displayName }),
    }),
  delete: (id: string) => request<void>(`/operators/${id}`, { method: 'DELETE' }),
};

// --- Policies ---

export const policies = {
  list: (params?: { page?: number; per_page?: number }) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.per_page) query.set('per_page', String(params.per_page));
    const qs = query.toString();
    const base = withTenant('/policies');
    return request<PaginatedListResponse<Policy>>(qs ? `${base}${base.includes('?') ? '&' : '?'}${qs}` : base);
  },
  get: (id: string) => request<Policy>(`/policies/${id}`),
  create: (policy: Partial<Policy>) =>
    request<Policy>(withTenant('/policies'), {
      method: 'POST',
      body: JSON.stringify(policy),
    }),
  update: (id: string, policy: Partial<Policy>) =>
    request<Policy>(`/policies/${id}`, {
      method: 'PUT',
      body: JSON.stringify(policy),
    }),
  delete: (id: string) => request<void>(`/policies/${id}`, { method: 'DELETE' }),
};

// --- Webhooks ---

export const webhooks = {
  list: (params?: { page?: number; per_page?: number }) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.per_page) query.set('per_page', String(params.per_page));
    const qs = query.toString();
    const base = withTenant('/webhooks');
    return request<PaginatedListResponse<Webhook>>(qs ? `${base}${base.includes('?') ? '&' : '?'}${qs}` : base);
  },
  create: (webhook: Partial<Webhook>) =>
    request<Webhook>(withTenant('/webhooks'), {
      method: 'POST',
      body: JSON.stringify(webhook),
    }),
  delete: (id: string) => request<void>(`/webhooks/${id}`, { method: 'DELETE' }),
};

// --- Requests ---

export const requests = {
  list: (params?: { page?: number; per_page?: number; status?: string; type?: string }) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.per_page) query.set('per_page', String(params.per_page));
    if (params?.status) query.set('status', params.status);
    if (params?.type) query.set('type', params.type);
    const qs = query.toString();
    return request<PaginatedListResponse<Request>>(withTenant(`/requests${qs ? '?' + qs : ''}`));
  },
  get: (id: string) => request<Request>(`/requests/${id}`),
  audit: (id: string) => request<ListResponse<AuditLog>>(`/requests/${id}/audit`),
};

// --- Deliveries ---

export const deliveries = {
  list: (params?: { page?: number; per_page?: number; status?: string; request_id?: string }) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.per_page) query.set('per_page', String(params.per_page));
    if (params?.status) query.set('status', params.status);
    if (params?.request_id) query.set('request_id', params.request_id);
    const qs = query.toString();
    return request<PaginatedListResponse<OutboxEntry>>(withTenant(`/deliveries${qs ? '?' + qs : ''}`));
  },
  stats: () => request<DeliveryStats>(withTenant('/deliveries/stats')),
  retry: (id: string) =>
    request<{ status: string }>(`/deliveries/${id}/retry`, { method: 'POST' }),
  retryAllForRequest: (requestId: string) =>
    request<{ reset: number }>(`/requests/${requestId}/retry-deliveries`, { method: 'POST' }),
};

export { ApiError };
