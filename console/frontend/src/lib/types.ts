// --- Models ---

export interface Tenant {
  id: string;
  slug: string;
  name: string;
  created_at: string;
  updated_at: string;
}

export interface Operator {
  id: string;
  username: string;
  display_name: string;
  must_change_password: boolean;
  created_at: string;
  updated_at: string;
}

export interface ApprovalStage {
  index: number;
  name?: string;
  required_approvals: number;
  allowed_checker_roles?: string[];
  rejection_policy: string;
  max_checkers?: number;
}

export interface Policy {
  id: string;
  tenant_id: string;
  name: string;
  request_type: string;
  stages: ApprovalStage[];
  identity_fields?: string[];
  permission_check_url?: string;
  auto_expire_duration?: string;
  display_template?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface Webhook {
  id: string;
  tenant_id: string;
  url: string;
  events: string[];
  secret: string;
  request_type?: string;
  active: boolean;
  created_at: string;
}

export interface Approval {
  id: string;
  request_id: string;
  checker_id: string;
  decision: 'approved' | 'rejected';
  comment?: string;
  stage_index: number;
  created_at: string;
}

export interface Request {
  id: string;
  tenant_id: string;
  type: string;
  payload: Record<string, unknown>;
  status: 'pending' | 'approved' | 'rejected' | 'cancelled' | 'expired';
  maker_id: string;
  idempotency_key?: string;
  fingerprint?: string;
  eligible_reviewers?: string[];
  metadata?: Record<string, unknown>;
  current_stage: number;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}

export interface AuditLog {
  id: string;
  tenant_id: string;
  request_id: string;
  action: string;
  actor_id: string;
  details?: Record<string, unknown>;
  created_at: string;
}

export interface OutboxEntry {
  id: string;
  request_id: string;
  webhook_url: string;
  status: 'pending' | 'processing' | 'delivered' | 'failed';
  attempts: number;
  max_retries: number;
  last_error?: string;
  next_retry_at: string;
  created_at: string;
  delivered_at?: string;
}

export interface DeliveryStats {
  pending: number;
  processing: number;
  delivered: number;
  failed: number;
}

// --- API Responses ---

export interface AuthResponse {
  operator: Operator;
  token: string;
}

export interface ListResponse<T> {
  data: T[];
}

export interface PaginatedListResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

export interface SetupStatusResponse {
  needs_setup: boolean;
}

export interface ErrorResponse {
  error: string;
}

// --- Display ---

export interface ResolvedField {
  label: string;
  value: string;
}

export interface ResolvedItem {
  title: string;
  fields: ResolvedField[];
}

export interface ResolvedDisplay {
  title?: string;
  fields: ResolvedField[];
  items?: ResolvedItem[];
}

export function getDisplay(metadata?: Record<string, unknown>): ResolvedDisplay | null {
  if (!metadata || !metadata.display) return null;
  const display = metadata.display as Record<string, unknown>;
  if (!display.fields || !Array.isArray(display.fields)) return null;
  return display as unknown as ResolvedDisplay;
}
