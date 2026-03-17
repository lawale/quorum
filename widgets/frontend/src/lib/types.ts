export interface Request {
  id: string;
  tenant_id: string;
  type: string;
  payload: Record<string, unknown>;
  status: RequestStatus;
  maker_id: string;
  idempotency_key?: string;
  fingerprint?: string;
  eligible_reviewers?: string[];
  metadata?: Record<string, unknown>;
  current_stage: number;
  expires_at?: string;
  created_at: string;
  updated_at: string;
  approvals?: Approval[];
}

export type RequestStatus = 'pending' | 'approved' | 'rejected' | 'cancelled' | 'expired';

export interface Approval {
  id: string;
  request_id: string;
  checker_id: string;
  decision: 'approved' | 'rejected';
  comment?: string;
  stage_index: number;
  created_at: string;
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
  created_at: string;
  updated_at: string;
}

export interface ApprovalStage {
  index: number;
  name?: string;
  required_approvals: number;
  allowed_checker_roles?: string[];
  rejection_policy: 'any' | 'threshold';
  max_checkers?: number;
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

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

export interface ListResponse<T> {
  data: T[];
}

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
